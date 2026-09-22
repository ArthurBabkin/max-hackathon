// Package auth — вход в мини-приложение: проверка строки запуска MAX
// (initData), дев-обход для браузера и JWT сессии.
//
// Алгоритм подписи сверен с https://dev.max.ru/docs/webapps/validation и с
// функцией ValidateInitData официального Go SDK
// (github.com/max-messenger/max-bot-api-client-go, Apache 2.0). Сам SDK
// подключить нельзя: он требует go 1.24, а рантайм Cloud Functions — golang123.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InitDataMaxAge — сколько живёт строка запуска. SDK MAX свежесть не
// проверяет; без этого перехваченная строка работала бы бессрочно.
const InitDataMaxAge = time.Hour

// futureSkew — допустимое расхождение часов клиента и сервера.
const futureSkew = time.Minute

var (
	// ErrMalformed — строка не разбирается или в ней нет обязательных полей.
	ErrMalformed = errors.New("auth: initData не разбирается")
	// ErrBadSignature — подпись не сходится: строку подделали или подписал другой бот.
	ErrBadSignature = errors.New("auth: подпись initData не сходится")
	// ErrExpired — подпись верна, но строка устарела или пришла «из будущего».
	ErrExpired = errors.New("auth: initData устарела")
	// ErrNoBotToken — токен бота не настроен, проверять подпись нечем.
	ErrNoBotToken = errors.New("auth: токен бота не задан")
)

// User — поле user строки запуска.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	PhotoURL     string `json:"photo_url"`
}

// Chat — поле chat: откуда открыто мини-приложение. Есть не всегда.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// InitData — разобранная строка запуска.
type InitData struct {
	AuthDate   time.Time
	QueryID    string
	StartParam string
	IP         string
	User       User
	Chat       *Chat
	// Unsigned — строка принята дев-обходом без подписи. По нему хендлер
	// пишет предупреждение в лог на каждый такой вход.
	Unsigned bool
}

// VerifyInitData проверяет подпись и свежесть строки запуска и разбирает её.
//
// Строка проверки — пары key=value без hash, значения URL-декодированы,
// отсортированы по ключу и склеены через \n;
// secret = HMAC_SHA256(key="WebAppData", msg=botToken);
// подпись = hex(HMAC_SHA256(key=secret, msg=строка)).
func VerifyInitData(raw, botToken string, now time.Time, maxAge time.Duration) (InitData, error) {
	if botToken == "" {
		return InitData{}, ErrNoBotToken
	}
	pairs, err := verifiedPairs(raw, botToken)
	if err != nil {
		return InitData{}, err
	}
	data, err := parseFields(pairs, true)
	if err != nil {
		return InitData{}, err
	}
	if age := now.Sub(data.AuthDate); age > maxAge || age < -futureSkew {
		return InitData{}, fmt.Errorf("%w: auth_date %s, сейчас %s", ErrExpired,
			data.AuthDate.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	return data, nil
}

// verifiedPairs находит разбор строки, подпись которого сходится.
//
// Документация велит декодировать каждое значение отдельно. SDK вместо этого
// сначала декодирует всю строку целиком — так он понимает initData,
// закодированную ещё раз (например, взятую из фрагмента URL), но ломается на
// «&» внутри имени. Пробуем оба варианта и оба способа раскодировать «+»:
// подделкой это не является, потому что принимается только тот разбор, для
// которого подпись сошлась, и поля берутся именно из него.
func verifiedPairs(raw, botToken string) (map[string]string, error) {
	sources := []string{raw}
	if whole, err := url.QueryUnescape(raw); err == nil && whole != raw {
		sources = append(sources, whole)
	}
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))

	parsed := false
	for _, src := range sources {
		for _, unescape := range []func(string) (string, error){url.QueryUnescape, url.PathUnescape} {
			pairs, err := splitPairs(src, unescape)
			if err != nil {
				continue
			}
			hash, ok := pairs["hash"]
			if !ok {
				continue
			}
			parsed = true
			want := hex.EncodeToString(hmacSHA256(secret, []byte(checkString(pairs))))
			if hmac.Equal([]byte(want), []byte(hash)) {
				return pairs, nil
			}
		}
	}
	if !parsed {
		return nil, ErrMalformed
	}
	return nil, ErrBadSignature
}

// splitPairs разбирает key=value&… и отказывает на повторяющихся ключах:
// двусмысленная строка (какой из двух user настоящий?) не должна проходить.
func splitPairs(s string, unescape func(string) (string, error)) (map[string]string, error) {
	if s == "" {
		return nil, ErrMalformed
	}
	out := map[string]string{}
	for _, part := range strings.Split(s, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		key, err := unescape(k)
		if err != nil {
			return nil, ErrMalformed
		}
		val, err := unescape(v)
		if err != nil {
			return nil, ErrMalformed
		}
		if _, dup := out[key]; dup {
			return nil, ErrMalformed
		}
		out[key] = val
	}
	return out, nil
}

func checkString(pairs map[string]string) string {
	lines := make([]string, 0, len(pairs))
	for k, v := range pairs {
		if k != "hash" {
			lines = append(lines, k+"="+v)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

// parseFields раскладывает пары по полям. auth_date — Unix-время в секундах.
func parseFields(pairs map[string]string, requireAuthDate bool) (InitData, error) {
	var d InitData
	rawUser, ok := pairs["user"]
	if !ok {
		return d, fmt.Errorf("%w: нет user", ErrMalformed)
	}
	if err := json.Unmarshal([]byte(rawUser), &d.User); err != nil {
		return d, fmt.Errorf("%w: user: %v", ErrMalformed, err)
	}
	if d.User.ID <= 0 {
		return d, fmt.Errorf("%w: нет user.id", ErrMalformed)
	}
	if rawChat, ok := pairs["chat"]; ok && rawChat != "" {
		d.Chat = &Chat{}
		if err := json.Unmarshal([]byte(rawChat), d.Chat); err != nil {
			return d, fmt.Errorf("%w: chat: %v", ErrMalformed, err)
		}
	}
	if rawDate, ok := pairs["auth_date"]; ok {
		sec, err := strconv.ParseInt(rawDate, 10, 64)
		if err != nil || sec <= 0 {
			return d, fmt.Errorf("%w: auth_date %q", ErrMalformed, rawDate)
		}
		d.AuthDate = time.Unix(sec, 0).UTC()
	} else if requireAuthDate {
		return d, fmt.Errorf("%w: нет auth_date", ErrMalformed)
	}
	d.QueryID = pairs["query_id"]
	d.StartParam = pairs["start_param"]
	d.IP = pairs["ip"]
	return d, nil
}
