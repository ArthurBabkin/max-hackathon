// Package bot — сценарии чата MAX. Логика живёт здесь, а не в cmd, чтобы
// одна и та же реализация работала и в Cloud Functions, и в обычном
// http.Server из docker-compose, и в long polling для разработки.
package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// Handler — вебхук MAX.
type Handler struct {
	secret string
	bot    *Bot
}

func NewHandler(secret string, b *Bot) *Handler {
	return &Handler{secret: secret, bot: b}
}

// ServeHTTP отвечает MAX всегда 200, кроме неверного секрета: любой другой
// код — ошибка доставки, повторы через минуты, а через 8 часов неудач MAX
// отписывает бота. Наши ошибки повтором не лечатся — их видно в логах.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Секрет проверяем, если он задан; сравнение постоянного времени.
	if h.secret != "" {
		got := req.Header.Get("X-Max-Bot-Api-Secret")
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.secret)) != 1 {
			slog.Warn("вебхук: секрет не совпал")
			rw.WriteHeader(http.StatusForbidden)
			return
		}
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		slog.Warn("вебхук: чтение тела", "err", err)
		rw.WriteHeader(http.StatusOK)
		return
	}
	var u maxapi.Update
	if err := json.Unmarshal(body, &u); err != nil {
		slog.Warn("вебхук: разбор обновления", "err", err)
		rw.WriteHeader(http.StatusOK)
		return
	}
	// MAX ждёт ответа не дольше 30 секунд.
	ctx, cancel := context.WithTimeout(req.Context(), 25*time.Second)
	defer cancel()
	h.bot.handleLogged(ctx, u)
	rw.WriteHeader(http.StatusOK)
}

// handleLogged — обработка без паники наружу: сбой в одном сценарии не
// должен ронять функцию и терять следующие обновления.
func (b *Bot) handleLogged(ctx context.Context, u maxapi.Update) {
	defer func() {
		if p := recover(); p != nil {
			slog.Error("паника в обработке обновления", "update_type", u.UpdateType, "panic", p)
			b.apologize(ctx, u)
		}
	}()
	start := time.Now()
	if err := b.Handle(ctx, u); err != nil {
		slog.Error("обработка обновления", "update_type", u.UpdateType, "err", err)
		b.apologize(ctx, u)
		return
	}
	slog.Info("обновление", "update_type", u.UpdateType, "ms", time.Since(start).Milliseconds())
}

// apologize — пользователь не должен остаться без ответа, если что-то
// сломалось на нашей стороне.
func (b *Bot) apologize(ctx context.Context, u maxapi.Update) {
	from, ok := u.From()
	if !ok {
		return
	}
	text := kidVoice(&turn{user: from}).T("bot.error", nil)
	if u.Callback != nil {
		_ = b.max.Answer(ctx, u.Callback.CallbackID, maxapi.CallbackAnswer{Notification: text})
		return
	}
	_, _ = b.max.Send(ctx, from.UserID, maxapi.Text(text))
}

// Poller — источник обновлений long polling. Реализует *maxapi.Client.
type Poller interface {
	Updates(ctx context.Context, marker *int64, timeout time.Duration, types []string) (maxapi.UpdateList, error)
}

// Poll — long polling для разработки (BOT_MODE=poll): бот отлаживается в
// настоящем MAX без HTTPS-адреса. Пока есть подписка на вебхук, MAX сюда
// ничего не отдаёт.
func (b *Bot) Poll(ctx context.Context, p Poller) error {
	var marker *int64
	for {
		list, err := p.Updates(ctx, marker, 30*time.Second, maxapi.SubscribedTypes)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			slog.Warn("long polling", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range list.Updates {
			b.handleLogged(ctx, u)
		}
		if list.Marker != nil {
			marker = list.Marker
		}
	}
}
