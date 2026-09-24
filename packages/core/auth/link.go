package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Ссылка — короткий пропуск к одному ресурсу для тех, кто не пришлёт
// заголовок Authorization: файл календаря скачивает браузер телефона или
// сам MAX, токена сессии у них нет.
//
// Это тот же HS256-JWT, но с аудиторией «traektoria-link-<назначение>»: Parse
// сессии её не примет, а ParseLink не примет токен сессии. Поэтому адрес из
// истории браузера открывает только свой ресурс и только до истечения срока.

// IssueLink подписывает ссылку участника memberID для назначения purpose.
func IssueLink(purpose, memberID string, secret []byte, ttl time.Duration, now time.Time) (string, time.Time, error) {
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("auth: пустой секрет ссылки")
	}
	if purpose == "" || memberID == "" {
		return "", time.Time{}, errors.New("auth: у ссылки нет назначения или участника")
	}
	if ttl <= 0 {
		return "", time.Time{}, fmt.Errorf("auth: срок жизни ссылки %v", ttl)
	}
	iat := now.UTC().Truncate(time.Second)
	exp := iat.Add(ttl)
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    jwtIssuer,
		Subject:   memberID,
		Audience:  jwt.ClaimStrings{linkAudience(purpose)},
		IssuedAt:  jwt.NewNumericDate(iat),
		ExpiresAt: jwt.NewNumericDate(exp),
	}).SignedString(secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: подпись ссылки: %w", err)
	}
	return signed, exp, nil
}

// ParseLink проверяет ссылку и возвращает участника, которому она выдана.
func ParseLink(purpose, token string, secret []byte, now time.Time) (string, error) {
	if len(secret) == 0 {
		return "", fmt.Errorf("%w: пустой секрет", ErrInvalidToken)
	}
	var c jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(linkAudience(purpose)),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(jwtLeeway),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if c.Subject == "" {
		return "", fmt.Errorf("%w: нет sub", ErrInvalidToken)
	}
	return c.Subject, nil
}

// Своя приставка: назначение «miniapp» не должно совпасть с аудиторией сессии.
func linkAudience(purpose string) string { return "traektoria-link-" + purpose }
