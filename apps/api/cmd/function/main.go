// Точка входа Cloud Functions для REST мини-приложения.
// Entrypoint: apps/api/cmd/function/main.Handler
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин.
package main

import (
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/api/internal/api"
)

var router = api.New()

func Handler(rw http.ResponseWriter, req *http.Request) {
	router.ServeHTTP(rw, req)
}

// main нужен только для локальной сборки, рантайм его не вызывает.
func main() {}
