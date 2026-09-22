// Точка входа Cloud Functions для REST мини-приложения.
// Entrypoint: apps/api/cmd/function/main.Handler
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин.
package main

import (
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/api/internal/api"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

var router = api.New()

// init проверяет конфиг при загрузке плагина: инстанс с опасной комбинацией
// (дев-обход подписи в production, слабый JWT_SECRET) не должен обслужить ни
// одного запроса.
func init() {
	if _, err := config.Load(); err != nil {
		log.Fatalf("конфиг: %v", err)
	}
}

func Handler(rw http.ResponseWriter, req *http.Request) {
	router.ServeHTTP(rw, req)
}

// main нужен только для локальной сборки, рантайм его не вызывает.
func main() {}
