// Заглушка вебхука MAX: принимает любое обновление и отвечает 200 с пустым телом.
// Нужна, чтобы зарегистрировать подписку (POST /subscriptions) до того, как появится логика:
// URL функции присваивается при создании и больше не меняется.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

type update struct {
	UpdateType string `json:"update_type"`
}

// Handler — точка входа Cloud Functions.
func Handler(rw http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("read body: %v", err)
		rw.WriteHeader(http.StatusOK) // MAX не должен ретраить из-за нашей ошибки чтения
		return
	}

	// Секрет проверяем, только если он задан в окружении: на этапе регистрации
	// подписки его ещё может не быть, и отбивка 401 помешала бы MAX принять вебхук.
	if want := os.Getenv("WEBHOOK_SECRET"); want != "" {
		got := req.Header.Get("X-Max-Bot-Api-Secret")
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			log.Printf("secret mismatch")
			rw.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var u update
	if err := json.Unmarshal(body, &u); err != nil {
		log.Printf("unmarshal: %v, raw=%s", err, truncate(body))
	} else {
		log.Printf("update_type=%s raw=%s", u.UpdateType, truncate(body))
	}

	rw.WriteHeader(http.StatusOK)
}

func truncate(b []byte) string {
	const max = 1000
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
