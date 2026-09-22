// Package bot содержит обработку вебхука MAX. Логика живёт здесь, а не в cmd,
// чтобы одна и та же реализация работала и в Cloud Functions, и в обычном
// http.Server из docker-compose.
package bot

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

type Handler struct {
	secret string
	max    *maxapi.Client
}

func New(secret string, client *maxapi.Client) *Handler {
	return &Handler{secret: secret, max: client}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("чтение тела: %v", err)
		// Отдаём 200: ошибка наша, и повторы MAX её не исправят.
		rw.WriteHeader(http.StatusOK)
		return
	}

	// Секрет проверяем, только если он задан. На этапе регистрации подписки
	// его может не быть, и отбивка помешала бы MAX принять вебхук.
	if h.secret != "" {
		got := req.Header.Get("X-Max-Bot-Api-Secret")
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.secret)) != 1 {
			log.Printf("секрет не совпал")
			rw.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var u maxapi.Update
	if err := json.Unmarshal(body, &u); err != nil {
		log.Printf("разбор обновления: %v, raw=%s", err, truncate(body))
		rw.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("update_type=%s raw=%s", u.UpdateType, truncate(body))

	// Здесь появятся сценарии из раздела 6 ТЗ: онбординг, подбор, трекер.
	// Пока любой апдейт подтверждается без ответа пользователю.

	// MAX ждёт 200 в течение 30 секунд; любой другой код считается ошибкой
	// доставки, а через 8 часов неудач бот отписывается от вебхука.
	rw.WriteHeader(http.StatusOK)
}

func truncate(b []byte) string {
	const max = 1000
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
