// Разовая настройка бота в MAX: подсказки команд (F51) и подписка на
// вебхук. Запуск: go run ./apps/bot/cmd/setup [-webhook https://…] [-unsubscribe https://…]
// Без флагов только ставит команды и печатает текущие подписки.
// Секрет вебхука берётся из WEBHOOK_SECRET, токен — из MAX_BOT_TOKEN.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

func main() {
	webhook := flag.String("webhook", "", "подписать бота на этот https-адрес")
	unsubscribe := flag.String("unsubscribe", "", "снять подписку с этого адреса")
	flag.Parse()

	token := os.Getenv("MAX_BOT_TOKEN")
	if token == "" {
		log.Fatal("MAX_BOT_TOKEN не задан")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := maxapi.New(config.Get("MAX_API_BASE", maxapi.DefaultBaseURL), token)

	me, err := c.Me(ctx)
	if err != nil {
		log.Fatalf("GET /me: %v", err)
	}
	fmt.Printf("бот: @%s, user_id %d\n", me.Username, me.UserID)
	if id, _ := config.BotIdentity(); id.ID != me.UserID || id.Name != me.Username {
		fmt.Printf("ВНИМАНИЕ: MAX_BOT_NAME/MAX_BOT_ID (%s/%d) не совпадают с ботом токена\n", id.Name, id.ID)
	}

	if err := c.SetCommands(ctx, bot.Commands()); err != nil {
		log.Fatalf("PATCH /me/commands: %v", err)
	}
	fmt.Println("команды: /menu /settings /help /delete")

	if *unsubscribe != "" {
		if err := c.Unsubscribe(ctx, *unsubscribe); err != nil {
			log.Fatalf("DELETE /subscriptions: %v", err)
		}
		fmt.Println("подписка снята:", *unsubscribe)
	}
	if *webhook != "" {
		if !strings.HasPrefix(*webhook, "https://") {
			log.Fatal("вебхук MAX принимает только https-адрес")
		}
		secret := os.Getenv("WEBHOOK_SECRET")
		if secret == "" {
			log.Fatal("WEBHOOK_SECRET не задан: без секрета вебхук примет чужие запросы")
		}
		if err := c.Subscribe(ctx, *webhook, secret, maxapi.SubscribedTypes); err != nil {
			log.Fatalf("POST /subscriptions: %v", err)
		}
		fmt.Println("подписан:", *webhook)
	}

	subs, err := c.Subscriptions(ctx)
	if err != nil {
		log.Fatalf("GET /subscriptions: %v", err)
	}
	if len(subs) == 0 {
		fmt.Println("подписок нет — обновления доступны через long polling (BOT_MODE=poll)")
	}
	for _, s := range subs {
		fmt.Printf("подписка: %s %v\n", s.URL, s.UpdateTypes)
	}
}
