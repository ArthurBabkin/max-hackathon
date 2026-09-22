package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtIssuer   = "traektoria"
	jwtAudience = "traektoria-miniapp"
	// jwtLeeway — допуск на расхождение часов между инстансами функций.
	jwtLeeway = 30 * time.Second
)

// ErrInvalidToken — JWT не прошёл проверку. Причина в тексте ошибки, клиенту
// она не показывается: ответ всегда 401.
var ErrInvalidToken = errors.New("auth: недействительный токен")

// Claims — содержимое JWT сессии мини-приложения.
//
// Права (Permissions контракта) в токен не кладутся: они зависят от того,
// есть ли в траектории ученик, а это меняется, пока токен жив. Их считают на
// каждый запрос.
type Claims struct {
	UserID       string // sub — users.id
	MaxUserID    int64  // mxi — id пользователя в MAX
	MemberID     string // mid — members.id
	TrajectoryID string // tid — trajectories.id
	Role         string // rol — kid | parent
	IsCreator    bool   // cre
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

type jwtClaims struct {
	MaxUserID    int64  `json:"mxi"`
	MemberID     string `json:"mid"`
	TrajectoryID string `json:"tid"`
	Role         string `json:"rol"`
	IsCreator    bool   `json:"cre"`
	jwt.RegisteredClaims
}

func (c Claims) validate() error {
	switch {
	case c.UserID == "":
		return errors.New("нет sub")
	case c.MemberID == "":
		return errors.New("нет mid")
	case c.TrajectoryID == "":
		return errors.New("нет tid")
	case c.Role != "kid" && c.Role != "parent":
		return fmt.Errorf("роль %q", c.Role)
	}
	return nil
}

// Issue подписывает токен HS256 и возвращает его вместе с моментом истечения
// (он же expires_at в ответе POST /session).
func Issue(c Claims, secret []byte, ttl time.Duration, now time.Time) (string, time.Time, error) {
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("auth: пустой секрет JWT")
	}
	if ttl <= 0 {
		return "", time.Time{}, fmt.Errorf("auth: срок жизни JWT %v", ttl)
	}
	if err := c.validate(); err != nil {
		return "", time.Time{}, fmt.Errorf("auth: %w", err)
	}
	// Секунды: в JWT время целое, и иначе round-trip расходился бы с ответом.
	iat := now.UTC().Truncate(time.Second)
	exp := iat.Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		MaxUserID:    c.MaxUserID,
		MemberID:     c.MemberID,
		TrajectoryID: c.TrajectoryID,
		Role:         c.Role,
		IsCreator:    c.IsCreator,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   c.UserID,
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(iat),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: подпись JWT: %w", err)
	}
	return signed, exp, nil
}

// Parse проверяет подпись, алгоритм, издателя, аудиторию и сроки токена.
// Алгоритм закреплён: заголовок токена не выбирает, чем его проверять,
// поэтому alg=none и чужие алгоритмы отбрасываются.
func Parse(token string, secret []byte, now time.Time) (Claims, error) {
	if len(secret) == 0 {
		return Claims{}, fmt.Errorf("%w: пустой секрет", ErrInvalidToken)
	}
	var jc jwtClaims
	_, err := jwt.ParseWithClaims(token, &jc,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(jwtLeeway),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	c := Claims{
		UserID:       jc.Subject,
		MaxUserID:    jc.MaxUserID,
		MemberID:     jc.MemberID,
		TrajectoryID: jc.TrajectoryID,
		Role:         jc.Role,
		IsCreator:    jc.IsCreator,
	}
	if jc.IssuedAt != nil {
		c.IssuedAt = jc.IssuedAt.Time.UTC()
	}
	c.ExpiresAt = jc.ExpiresAt.Time.UTC()
	if err := c.validate(); err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return c, nil
}
