package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("test-jwt-secret-0123456789abcdef-long-enough")

func testClaims() Claims {
	return Claims{
		UserID:       "6f1c2a44-0000-4000-8000-000000000001",
		MaxUserID:    900000001,
		MemberID:     "6f1c2a44-0000-4000-8000-000000000002",
		TrajectoryID: "6f1c2a44-0000-4000-8000-000000000003",
		Role:         "kid",
		IsCreator:    true,
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	token, exp, err := Issue(testClaims(), testSecret, time.Hour, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(testNow.Add(time.Hour)) {
		t.Errorf("expires_at = %v, want %v", exp, testNow.Add(time.Hour))
	}
	got, err := Parse(token, testSecret, testNow.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("свежий токен отклонён: %v", err)
	}
	want := testClaims()
	want.IssuedAt, want.ExpiresAt = testNow, exp
	if got != want {
		t.Errorf("claims = %+v\nwant   %+v", got, want)
	}
}

func TestJWT_HeaderPinsHS256(t *testing.T) {
	token, _, err := Issue(testClaims(), testSecret, time.Hour, testNow)
	if err != nil {
		t.Fatal(err)
	}
	header, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	if !strings.Contains(string(header), `"alg":"HS256"`) {
		t.Errorf("заголовок = %s", header)
	}
}

func TestJWT_RejectsExpired(t *testing.T) {
	token, _, _ := Issue(testClaims(), testSecret, time.Hour, testNow)
	if _, err := Parse(token, testSecret, testNow.Add(time.Hour+2*time.Minute)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("просроченный токен должен отклоняться, получили %v", err)
	}
}

func TestJWT_RejectsForeignSecret(t *testing.T) {
	token, _, _ := Issue(testClaims(), []byte("another-secret-0123456789abcdef-xxxxxx"), time.Hour, testNow)
	if _, err := Parse(token, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("токен с чужим секретом должен отклоняться, получили %v", err)
	}
}

func TestJWT_RejectsFlippedSignature(t *testing.T) {
	token, _, _ := Issue(testClaims(), testSecret, time.Hour, testNow)
	parts := strings.Split(token, ".")
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	if _, err := Parse(parts[0]+"."+parts[1]+"."+string(sig), testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("испорченная подпись должна отклоняться, получили %v", err)
	}
}

func payloadFor(t *testing.T, now time.Time) string {
	t.Helper()
	token, _, err := Issue(testClaims(), testSecret, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(token, ".")[1]
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// Классическая атака: alg=none и пустая подпись. Без WithValidMethods
// некоторые библиотеки такой токен принимают.
func TestJWT_RejectsAlgNone(t *testing.T) {
	token := b64(`{"alg":"none","typ":"JWT"}`) + "." + payloadFor(t, testNow) + "."
	if _, err := Parse(token, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("alg=none должен отклоняться, получили %v", err)
	}
}

// Токен, честно подписанный тем же секретом, но другим HMAC-алгоритмом, тоже
// отклоняется: алгоритм закреплён, а не берётся из заголовка.
func TestJWT_RejectsOtherHMACAlgorithm(t *testing.T) {
	signingInput := b64(`{"alg":"HS512","typ":"JWT"}`) + "." + payloadFor(t, testNow)
	mac := hmac.New(sha512.New, testSecret)
	mac.Write([]byte(signingInput))
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := Parse(token, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS512 должен отклоняться, получили %v", err)
	}
}

// Подписываем произвольный payload нашим секретом: так проверяются
// iss/aud/обязательные поля отдельно от подписи.
func signRaw(payload string) string {
	signingInput := b64(`{"alg":"HS256","typ":"JWT"}`) + "." + b64(payload)
	mac := hmac.New(sha256.New, testSecret)
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestJWT_ValidatesIssuerAudienceAndRequiredClaims(t *testing.T) {
	iat, exp := testNow.Unix(), testNow.Add(time.Hour).Unix()
	base := func(extra string) string {
		return `{"sub":"u","mxi":1,"mid":"m","tid":"t","rol":"kid","cre":false,` + extra +
			`"iat":` + itoa(iat) + `,"exp":` + itoa(exp) + `}`
	}
	cases := map[string]string{
		"чужой iss":     base(`"iss":"evil","aud":["traektoria-miniapp"],`),
		"чужой aud":     base(`"iss":"traektoria","aud":["other"],`),
		"нет exp":       `{"sub":"u","mxi":1,"mid":"m","tid":"t","rol":"kid","iss":"traektoria","aud":["traektoria-miniapp"],"iat":` + itoa(iat) + `}`,
		"нет sub":       `{"mxi":1,"mid":"m","tid":"t","rol":"kid","iss":"traektoria","aud":["traektoria-miniapp"],"iat":` + itoa(iat) + `,"exp":` + itoa(exp) + `}`,
		"нет mid":       `{"sub":"u","mxi":1,"tid":"t","rol":"kid","iss":"traektoria","aud":["traektoria-miniapp"],"iat":` + itoa(iat) + `,"exp":` + itoa(exp) + `}`,
		"нет tid":       `{"sub":"u","mxi":1,"mid":"m","rol":"kid","iss":"traektoria","aud":["traektoria-miniapp"],"iat":` + itoa(iat) + `,"exp":` + itoa(exp) + `}`,
		"чужая роль":    base(`"iss":"traektoria","aud":["traektoria-miniapp"],"rol":"admin",`),
		"iat в будущем": `{"sub":"u","mxi":1,"mid":"m","tid":"t","rol":"kid","iss":"traektoria","aud":["traektoria-miniapp"],"iat":` + itoa(testNow.Add(time.Hour).Unix()) + `,"exp":` + itoa(testNow.Add(2*time.Hour).Unix()) + `}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(signRaw(payload), testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("ожидали ErrInvalidToken, получили %v", err)
			}
		})
	}
	ok := base(`"iss":"traektoria","aud":["traektoria-miniapp"],`)
	if _, err := Parse(signRaw(ok), testSecret, testNow); err != nil {
		t.Fatalf("контрольный токен должен проходить: %v", err)
	}
}

func TestJWT_IssueRefusesWeakInput(t *testing.T) {
	if _, _, err := Issue(testClaims(), nil, time.Hour, testNow); err == nil {
		t.Error("подписывать пустым секретом нельзя")
	}
	if _, _, err := Issue(testClaims(), testSecret, 0, testNow); err == nil {
		t.Error("нулевой срок жизни — ошибка конфигурации")
	}
	bad := testClaims()
	bad.Role = "admin"
	if _, _, err := Issue(bad, testSecret, time.Hour, testNow); err == nil {
		t.Error("роль вне kid|parent выпускать нельзя")
	}
	bad = testClaims()
	bad.MemberID = ""
	if _, _, err := Issue(bad, testSecret, time.Hour, testNow); err == nil {
		t.Error("токен без участника выпускать нельзя")
	}
}

func TestJWT_ParseRefusesEmptySecret(t *testing.T) {
	token, _, _ := Issue(testClaims(), testSecret, time.Hour, testNow)
	if _, err := Parse(token, nil, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("пустой секрет при разборе — отказ, получили %v", err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
