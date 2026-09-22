package notifier

import (
	"context"
	"fmt"
	"os"

	"github.com/ArthurBabkin/max-hackathon/packages/db"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// FromEnv собирает воркер из окружения: DATABASE_URL, MAX_BOT_TOKEN,
// MAX_BOT_NAME, MAX_BOT_ID, MAX_API_BASE. Без токена события копятся.
func FromEnv(ctx context.Context) (*Worker, error) {
	bot, err := config.BotIdentity()
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
	}
	return New(store.New(pool), sender, bot), nil
}
