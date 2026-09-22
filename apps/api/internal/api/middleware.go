package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// Порядок: снять префикс → CORS → request id → recover → лог → роутер.
// Авторизация — внутри роутера, на уровне отдельных ручек.

// stripPrefix убирает базовый путь контракта. Локально nginx передаёт путь
// целиком (/api/v1/home), а в облаке функция может получить его без префикса.
func stripPrefix(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := strings.TrimPrefix(r.URL.Path, prefix); p != r.URL.Path {
			if p == "" {
				p = "/"
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = p
			r2.URL.RawPath = ""
			r = r2
		}
		next.ServeHTTP(w, r)
	})
}

// cors нужен в проде: мини-приложение лежит в Object Storage, а API — на
// домене функций, и запрос с Authorization идёт через preflight. Разрешены
// только перечисленные источники; локально запросы same-origin через прокси.
func cors(allowed []string, next http.Handler) http.Handler {
	set := map[string]bool{}
	for _, o := range allowed {
		if o = strings.TrimSpace(o); o != "" {
			set[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && set[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type ctxKey int

const requestIDKey ctxKey = iota

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func reqID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// logRequests пишет одну строку на запрос: метод, шаблон маршрута, статус,
// время. Ни токенов, ни тел, ни query — там могут быть персональные данные.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		defer func() {
			if p := recover(); p != nil {
				slog.Error("паника в обработчике", "request_id", reqID(r.Context()),
					"panic", p, "stack", string(debug.Stack()))
				if rec.status == 0 {
					writeError(rec, errInternal)
				}
			}
			route := r.Pattern
			if route == "" {
				route = r.Method + " (нет маршрута)"
			}
			slog.Info("запрос", "request_id", reqID(r.Context()), "route", route,
				"status", rec.status, "ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rec, r)
	})
}
