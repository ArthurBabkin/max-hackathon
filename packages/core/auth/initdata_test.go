package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "test-bot-token"

var testNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// sign собирает initData так, как описано на dev.max.ru/docs/webapps/validation:
// строка проверки — отсортированные пары key=value (значения декодированы)
// через \n, secret = HMAC_SHA256(key="WebAppData", BOT_TOKEN),
// hash = hex(HMAC_SHA256(secret, строка)). Написано независимо от кода
// пакета, чтобы тест ловил ошибки в самом алгоритме.
func sign(token string, pairs map[string]string) string {
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+pairs[k])
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))

	v := url.Values{}
	for k, val := range pairs {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func validPairs(authDate time.Time) map[string]string {
	return map[string]string{
		"auth_date":   strconv.FormatInt(authDate.Unix(), 10),
		"query_id":    "4c0ab423-342b-4e45-aea4-2747dbc500cd",
		"ip":          "192.168.0.1",
		"start_param": "inv_abc",
		"chat":        `{"id":12345,"type":"DIALOG"}`,
		"user": `{"id":67890,"first_name":"Ольга","last_name":"Петрова","username":"olga",` +
			`"language_code":"ru","photo_url":"https://example.org/p.jpg"}`,
	}
}

func TestVerifyInitData_AcceptsValidSignatureAndParsesFields(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow.Add(-time.Minute)))

	got, err := VerifyInitData(raw, testBotToken, testNow, time.Hour)
	if err != nil {
		t.Fatalf("верная подпись отклонена: %v", err)
	}
	want := User{ID: 67890, FirstName: "Ольга", LastName: "Петрова", Username: "olga",
		LanguageCode: "ru", PhotoURL: "https://example.org/p.jpg"}
	if got.User != want {
		t.Errorf("user = %+v, want %+v", got.User, want)
	}
	if got.Chat == nil || got.Chat.ID != 12345 || got.Chat.Type != "DIALOG" {
		t.Errorf("chat = %+v", got.Chat)
	}
	if got.QueryID != "4c0ab423-342b-4e45-aea4-2747dbc500cd" || got.IP != "192.168.0.1" || got.StartParam != "inv_abc" {
		t.Errorf("поля запуска разобраны неверно: %+v", got)
	}
	if !got.AuthDate.Equal(testNow.Add(-time.Minute)) {
		t.Errorf("auth_date = %v", got.AuthDate)
	}
	if got.Unsigned {
		t.Error("подписанная строка не должна помечаться как неподписанная")
	}
}

func TestVerifyInitData_ChatIsOptional(t *testing.T) {
	p := validPairs(testNow)
	delete(p, "chat")
	got, err := VerifyInitData(sign(testBotToken, p), testBotToken, testNow, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Chat != nil {
		t.Errorf("chat без параметра должен быть nil, получили %+v", got.Chat)
	}
}

func TestVerifyInitData_RejectsFlippedSignatureByte(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow))
	v, _ := url.ParseQuery(raw)
	h := []byte(v.Get("hash"))
	if h[0] == 'a' {
		h[0] = 'b'
	} else {
		h[0] = 'a'
	}
	v.Set("hash", string(h))

	_, err := VerifyInitData(v.Encode(), testBotToken, testNow, time.Hour)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("ожидали ErrBadSignature, получили %v", err)
	}
}

func TestVerifyInitData_RejectsTamperedField(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow))
	v, _ := url.ParseQuery(raw)
	v.Set("user", `{"id":1,"first_name":"Взломщик"}`)

	if _, err := VerifyInitData(v.Encode(), testBotToken, testNow, time.Hour); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("подмена user должна ломать подпись, получили %v", err)
	}
}

func TestVerifyInitData_RejectsOtherBotToken(t *testing.T) {
	raw := sign("other-bot-token", validPairs(testNow))
	if _, err := VerifyInitData(raw, testBotToken, testNow, time.Hour); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("подпись чужим токеном должна отклоняться, получили %v", err)
	}
}

// SDK MAX свежесть auth_date не проверяет, а без неё перехваченная строка
// запуска работает вечно.
func TestVerifyInitData_RejectsStaleAuthDateEvenWithValidSignature(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow.Add(-2*time.Hour)))
	if _, err := VerifyInitData(raw, testBotToken, testNow, time.Hour); !errors.Is(err, ErrExpired) {
		t.Fatalf("двухчасовой auth_date с верной подписью должен давать ErrExpired, получили %v", err)
	}
}

func TestVerifyInitData_AcceptsAuthDateWithinMaxAge(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow.Add(-59*time.Minute)))
	if _, err := VerifyInitData(raw, testBotToken, testNow, time.Hour); err != nil {
		t.Fatalf("строка 59-минутной давности должна проходить: %v", err)
	}
}

func TestVerifyInitData_RejectsAuthDateFromFuture(t *testing.T) {
	skewed := sign(testBotToken, validPairs(testNow.Add(30*time.Second)))
	if _, err := VerifyInitData(skewed, testBotToken, testNow, time.Hour); err != nil {
		t.Fatalf("расхождение часов в 30 секунд допустимо: %v", err)
	}
	future := sign(testBotToken, validPairs(testNow.Add(5*time.Minute)))
	if _, err := VerifyInitData(future, testBotToken, testNow, time.Hour); !errors.Is(err, ErrExpired) {
		t.Fatalf("auth_date из будущего должен отклоняться, получили %v", err)
	}
}

func TestVerifyInitData_MalformedInput(t *testing.T) {
	noUser := validPairs(testNow)
	delete(noUser, "user")
	zeroID := validPairs(testNow)
	zeroID["user"] = `{"id":0,"first_name":"А"}`
	badJSON := validPairs(testNow)
	badJSON["user"] = `{"id":`
	noDate := validPairs(testNow)
	delete(noDate, "auth_date")
	badDate := validPairs(testNow)
	badDate["auth_date"] = "вчера"

	cases := map[string]string{
		"пустая строка":      "",
		"нет hash":           "auth_date=1&user=%7B%22id%22%3A1%7D",
		"нет user":           sign(testBotToken, noUser),
		"user.id = 0":        sign(testBotToken, zeroID),
		"битый JSON user":    sign(testBotToken, badJSON),
		"нет auth_date":      sign(testBotToken, noDate),
		"auth_date не число": sign(testBotToken, badDate),
		"повторяющийся ключ": sign(testBotToken, validPairs(testNow)) + "&user=%7B%22id%22%3A1%7D",
		"повторяющийся hash": sign(testBotToken, validPairs(testNow)) + "&hash=00",
		"битое кодирование":  "user=%ZZ&auth_date=1&hash=00",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyInitData(raw, testBotToken, testNow, time.Hour); !errors.Is(err, ErrMalformed) {
				t.Fatalf("ожидали ErrMalformed, получили %v", err)
			}
		})
	}
}

func TestVerifyInitData_RequiresBotToken(t *testing.T) {
	raw := sign(testBotToken, validPairs(testNow))
	if _, err := VerifyInitData(raw, "", testNow, time.Hour); !errors.Is(err, ErrNoBotToken) {
		t.Fatalf("без токена бота проверять нечем: ожидали ErrNoBotToken, получили %v", err)
	}
}

// Амперсанд, знак равенства, плюс и процент в имени не должны ломать разбор:
// значения декодируются по одному, как требует документация.
func TestVerifyInitData_SpecialCharactersInValues(t *testing.T) {
	p := validPairs(testNow)
	p["user"] = `{"id":67890,"first_name":"Анна & Co = 1 + 2 %"}`
	got, err := VerifyInitData(sign(testBotToken, p), testBotToken, testNow, time.Hour)
	if err != nil {
		t.Fatalf("спецсимволы в значениях сломали проверку: %v", err)
	}
	if got.User.FirstName != "Анна & Co = 1 + 2 %" {
		t.Errorf("first_name = %q", got.User.FirstName)
	}
}

// Официальный Go SDK сначала декодирует строку целиком и только потом
// разбирает её. Так он понимает initData, закодированную ещё раз (например,
// взятую из фрагмента URL). Такую строку тоже принимаем.
func TestVerifyInitData_AcceptsWholeStringEncodedOnceMore(t *testing.T) {
	raw := url.QueryEscape(sign(testBotToken, validPairs(testNow)))
	got, err := VerifyInitData(raw, testBotToken, testNow, time.Hour)
	if err != nil {
		t.Fatalf("дважды закодированная строка отклонена: %v", err)
	}
	if got.User.ID != 67890 {
		t.Errorf("user.id = %d", got.User.ID)
	}
}

// Параметры, которых нет в документации (web_app_platform и т. п.), входят в
// подпись как есть и не мешают разбору.
func TestVerifyInitData_UnknownKeysAreSignedButIgnored(t *testing.T) {
	p := validPairs(testNow)
	p["web_app_platform"] = "ios"
	if _, err := VerifyInitData(sign(testBotToken, p), testBotToken, testNow, time.Hour); err != nil {
		t.Fatalf("незнакомый параметр сломал проверку: %v", err)
	}
}
