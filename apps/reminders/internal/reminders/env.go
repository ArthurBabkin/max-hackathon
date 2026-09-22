package reminders

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ArthurBabkin/max-hackathon/packages/db"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// FromEnv собирает воркер из окружения: DATABASE_URL, MAX_BOT_TOKEN,
// MAX_BOT_NAME, MAX_BOT_ID, REMINDER_HOUR, MAX_API_BASE. Без токена воркер
// ведёт план напоминаний, но ничего не отправляет.
func FromEnv(ctx context.Context) (*Worker, error) {
	bot, err := config.BotIdentity()
	if err != nil {
		return nil, err
	}
	hour, err := config.ReminderHour()
	if err != nil {
		return nil, err
	}
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, fmt.Errorf("база: %w", err)
	}
	var sender maxapi.Sender
	if token := os.Getenv("MAX_BOT_TOKEN"); token != "" {
		sender = maxapi.New(config.Get("MAX_API_BASE", maxapi.DefaultBaseURL), token)
	} else {
		slog.Warn("MAX_BOT_TOKEN пуст — напоминания планируются, но не отправляются")
	}
	return New(store.New(pool), sender, bot, hour), nil
}
