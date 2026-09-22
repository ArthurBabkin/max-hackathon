package auth

import (
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// stubInitData — ровно то, что собирает заглушка MAX Bridge в браузере
// (apps/web/src/bridge/stub.ts: new URLSearchParams({user, auth_date, hash})),
// с подставленным id.
func stubInitData(id int64, authDate time.Time) string {
	return "user=" + url.QueryEscape(`{"id":`+strconv.FormatInt(id, 10)+`,"first_name":"Артём"}`) +
		"&auth_date=" + strconv.FormatInt(authDate.Unix(), 10) +
		"&hash=dev-stub-unsigned"
}

func devPolicy() Policy {
	return Policy{BotToken: testBotToken, MaxAge: time.Hour, DevUnsignedInitData: true, AppEnv: "development"}
}

func TestPolicy_AcceptsBrowserStubVerbatim(t *testing.T) {
	// Строка скопирована из вывода URLSearchParams в Node — байт в байт то,
	// что уходит из браузера.
	raw := "user=%7B%22id%22%3A900000001%2C%22first_name%22%3A%22%D0%90%D1%80%D1%82%D1%91%D0%BC%22%7D" +
		"&auth_date=1790000000&hash=dev-stub-unsigned"

	got, err := devPolicy().Authenticate(raw, testNow)
	if err != nil {
		t.Fatalf("строка заглушки должна проходить дев-обход: %v", err)
	}
	if got.User.ID != 900000001 || got.User.FirstName != "Артём" {
		t.Errorf("user = %+v", got.User)
	}
	if !got.Unsigned {
		t.Error("принятая через обход строка должна быть помечена Unsigned — по нему пишется WARN")
	}
}

// Заглушка ставит auth_date в момент загрузки страницы, а после часа работы
// мини-приложение переавторизуется той же строкой. Для дев-диапазона свежесть
// не проверяется: подделать там нечего.
func TestPolicy_DevBypassIgnoresFreshness(t *testing.T) {
	if _, err := devPolicy().Authenticate(stubInitData(900000001, testNow.Add(-3*time.Hour)), testNow); err != nil {
		t.Fatalf("старая строка заглушки должна проходить в dev: %v", err)
	}
}

func TestPolicy_DevBypassRange(t *testing.T) {
	cases := []struct {
		id int64
		ok bool
	}{
		{899999999, false},
		{900000000, true},
		{900000001, true},
		{900000999, true},
		{900001000, false},
		{12345, false},
	}
	for _, c := range cases {
		_, err := devPolicy().Authenticate(stubInitData(c.id, testNow), testNow)
		if c.ok && err != nil {
			t.Errorf("id %d из дев-диапазона отклонён: %v", c.id, err)
		}
		if !c.ok && err == nil {
			t.Errorf("id %d вне дев-диапазона принят без подписи", c.id)
		}
	}
}

func TestPolicy_DevBypassRefusedInProduction(t *testing.T) {
	p := devPolicy()
	p.AppEnv = "production"
	_, err := p.Authenticate(stubInitData(900000001, testNow), testNow)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("в production неподписанная строка должна давать ErrBadSignature, получили %v", err)
	}
}

// Пустой APP_ENV считается production: забытая переменная не должна
// открывать обход.
func TestPolicy_DevBypassRefusedWhenAppEnvEmpty(t *testing.T) {
	p := devPolicy()
	p.AppEnv = ""
	if _, err := p.Authenticate(stubInitData(900000001, testNow), testNow); err == nil {
		t.Fatal("при пустом APP_ENV обход должен быть выключен")
	}
}

func TestPolicy_DevBypassRefusedWhenFlagOff(t *testing.T) {
	p := devPolicy()
	p.DevUnsignedInitData = false
	if _, err := p.Authenticate(stubInitData(900000001, testNow), testNow); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("без DEV_UNSIGNED_INITDATA обход выключен, получили %v", err)
	}
}

// Настоящая подписанная строка в dev проходит обычной проверкой — и для
// реального id, и без пометки Unsigned.
func TestPolicy_SignedDataStillVerifiedInDev(t *testing.T) {
	got, err := devPolicy().Authenticate(sign(testBotToken, validPairs(testNow)), testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unsigned || got.User.ID != 67890 {
		t.Errorf("ожидали подписанного пользователя 67890, получили %+v", got)
	}
}

// Обход не отменяет проверку свежести у подписанных строк реальных id.
func TestPolicy_StaleSignedRealUserStillExpiredInDev(t *testing.T) {
	_, err := devPolicy().Authenticate(sign(testBotToken, validPairs(testNow.Add(-2*time.Hour))), testNow)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("ожидали ErrExpired, получили %v", err)
	}
}

// Локально токена бота может не быть вовсе — тогда работает только обход.
func TestPolicy_NoBotToken(t *testing.T) {
	p := devPolicy()
	p.BotToken = ""
	if _, err := p.Authenticate(stubInitData(900000001, testNow), testNow); err != nil {
		t.Fatalf("без токена бота дев-обход должен работать: %v", err)
	}
	p.DevUnsignedInitData = false
	if _, err := p.Authenticate(stubInitData(900000001, testNow), testNow); !errors.Is(err, ErrNoBotToken) {
		t.Fatalf("без токена и без обхода ожидали ErrNoBotToken, получили %v", err)
	}
}

func TestPolicy_DevBypassRejectsMalformed(t *testing.T) {
	if _, err := devPolicy().Authenticate("user=%7B&hash=x", testNow); err == nil {
		t.Fatal("битая строка не должна проходить и через обход")
	}
}

func TestPolicy_DevBypassEnabled(t *testing.T) {
	cases := []struct {
		flag bool
		env  string
		want bool
	}{
		{true, "development", true},
		{true, "test", true},
		{true, "production", false},
		{true, "", false},
		{false, "development", false},
	}
	for _, c := range cases {
		p := Policy{DevUnsignedInitData: c.flag, AppEnv: c.env}
		if got := p.DevBypassEnabled(); got != c.want {
			t.Errorf("DevUnsigned=%v AppEnv=%q: %v, want %v", c.flag, c.env, got, c.want)
		}
	}
}
