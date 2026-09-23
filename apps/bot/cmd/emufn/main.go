// Точка входа Cloud Functions для эмулятора MAX (веб-чат с ботом).
// Entrypoint: apps/bot/cmd/emufn/main.Handler, вызывается через API Gateway:
// без шлюза функция не получает путь запроса.
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/emu"
	"github.com/ArthurBabkin/max-hackathon/packages/db"
)

var handler http.Handler

// init — один раз за холодный старт. Без ключа эмулятор не стартует: адрес
// шлюза публичный, а чат пишет в базу от имени любого тестового пользователя.
func init() {
	if os.Getenv("EMU_KEY") == "" {
		log.Fatal("эмулятор: EMU_KEY не задан")
	}
	pool, err := db.Pool(context.Background())
	if err != nil {
		log.Fatalf("эмулятор: %v", err)
	}
	e, err := emu.FromEnv(pool)
	if err != nil {
		log.Fatalf("эмулятор: %v", err)
	}
	handler = e
}

// Handler — обработчик, который вызывает рантайм.
func Handler(rw http.ResponseWriter, req *http.Request) {
	handler.ServeHTTP(rw, req)
}

// main существует только чтобы пакет собирался обычным go build.
func main() {}
