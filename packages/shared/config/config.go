// Package config читает настройки из окружения. В Cloud Functions переменные
// приходят из версии функции и из Lockbox, локально — из .env через compose.
package config

import (
	"log"
	"os"
)

// Get возвращает значение переменной или запасной вариант.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// MustGet завершает процесс, если переменная не задана.
// Для функций это лучше, чем работать с пустым токеном и молча отдавать ошибки.
func MustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("переменная окружения %s не задана", key)
	}
	return v
}
