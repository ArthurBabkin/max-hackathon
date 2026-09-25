package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// Тела запросов мини-приложения маленькие; лимит защищает функцию от
// случайного или намеренного мегабайтного тела.
const maxBody = 64 << 10

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("запись ответа", "err", err)
	}
}

// orEmpty — пустой срез вместо nil: в JSON массив, а не null.
func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return badRequest("Пустое тело запроса.")
		}
		return badRequest("Некорректный JSON.")
	}
	return nil
}
