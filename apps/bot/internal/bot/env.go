package bot

import (
	"context"
	"fmt"
	"os"

	"github.com/ArthurBabkin/max-hackathon/packages/db"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// FromEnv собирает бота из окружения: DATABASE_URL, MAX_BOT_TOKEN,
// MAX_BOT_NAME, MAX_BOT_ID, REMINDER_HOUR, MAX_API_BASE. Пул ленивый —
// вебхук с чужим секретом до базы не доходит. Пустой токен не ошибка:
// локальный стенд поднимается без секретов, отправка просто не пройдёт.
func FromEnv(ctx context.Context) (*Bot, *maxapi.Client, error) {
	token := os.Getenv("MAX_BOT_TOKEN")
	id, err := config.BotIdentity()
	if err != nil {
		return nil, nil, err
	}
	hour, err := config.ReminderHour()
	if err != nil {
		return nil, nil, err
	}
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("база: %w", err)
	}
	client := maxapi.New(config.Get("MAX_API_BASE", maxapi.DefaultBaseURL), token)
	return New(store.New(pool), client, Config{BotName: id.Name, BotID: id.ID, ReminderHour: hour}), client, nil
}
