package auth

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

// Дев-диапазон id: заглушка MAX Bridge в браузере представляется
// пользователем из него. Настоящих аккаунтов MAX с такими id нет, поэтому
// обход подписи не позволяет войти под живым человеком.
const (
	DevUserIDMin int64 = 900_000_000
	DevUserIDMax int64 = 900_000_999
)

var (
	// ErrDevBypassDisabled — обход выключен флагом или окружением.
	ErrDevBypassDisabled = errors.New("auth: дев-обход подписи выключен")
	// ErrNotDevUser — id вне дев-диапазона: без подписи такого не пускаем.
	ErrNotDevUser = errors.New("auth: id вне дев-диапазона")
)

// Policy — правила входа для POST /session. Поля берутся из конфига.
type Policy struct {
	BotToken            string
	MaxAge              time.Duration // 0 — InitDataMaxAge
	DevUnsignedInitData bool          // DEV_UNSIGNED_INITDATA
	AppEnv              string        // APP_ENV
}

// DevBypassEnabled — обход разрешён флагом и окружение не production.
// Пустое окружение считается production: забытая переменная обход не открывает.
func (p Policy) DevBypassEnabled() bool {
	return p.DevUnsignedInitData && p.AppEnv != "" && p.AppEnv != "production"
}

// Authenticate проверяет строку запуска. Сначала всегда настоящая проверка
// подписи; дев-обход пробуется, только если подписи нет или она не сходится
// (строка заглушки), — но не для устаревшей строки с верной подписью и не
// для битой строки.
func (p Policy) Authenticate(raw string, now time.Time) (InitData, error) {
	maxAge := p.MaxAge
	if maxAge <= 0 {
		maxAge = InitDataMaxAge
	}
	data, err := VerifyInitData(raw, p.BotToken, now, maxAge)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, ErrBadSignature) && !errors.Is(err, ErrNoBotToken) {
		return InitData{}, err
	}
	dev, devErr := p.DevUnsigned(raw)
	if devErr != nil {
		return InitData{}, err
	}
	return dev, nil
}

// DevUnsigned принимает неподписанную строку заглушки
// (apps/web/src/bridge/stub.ts): user, auth_date и hash «dev-stub-unsigned».
// Условия — все три сразу: DEV_UNSIGNED_INITDATA=true, APP_ENV не production,
// id из дев-диапазона. Подпись и свежесть не проверяются: заглушка ставит
// auth_date при загрузке страницы, а через час мини-приложение
// переавторизуется той же строкой.
func (p Policy) DevUnsigned(raw string) (InitData, error) {
	if !p.DevBypassEnabled() {
		return InitData{}, ErrDevBypassDisabled
	}
	pairs, err := splitPairs(raw, url.QueryUnescape)
	if err != nil {
		return InitData{}, err
	}
	data, err := parseFields(pairs, false)
	if err != nil {
		return InitData{}, err
	}
	if data.User.ID < DevUserIDMin || data.User.ID > DevUserIDMax {
		return InitData{}, fmt.Errorf("%w: %d", ErrNotDevUser, data.User.ID)
	}
	data.Unsigned = true
	return data, nil
}
