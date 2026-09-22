// Package api — REST для мини-приложения. Одна функция на все ручки:
// внутренний роутер вместо функции на эндпоинт, чтобы не множить холодные старты.
package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type Router struct {
	mux *http.ServeMux
}

func New() *Router {
	r := &Router{mux: http.NewServeMux()}
	r.routes()
	return r
}

func (r *Router) routes() {
	r.mux.HandleFunc("GET /health", func(rw http.ResponseWriter, _ *http.Request) {
		writeJSON(rw, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Ручки из раздела 10 ТЗ появятся здесь. Пока честно отвечаем 501,
	// чтобы фронт отличал «не реализовано» от «сломалось».
	r.mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		log.Printf("не реализовано: %s %s", req.Method, req.URL.Path)
		writeJSON(rw, http.StatusNotImplemented, map[string]string{
			"error": "not_implemented",
			"path":  req.URL.Path,
		})
	})
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(rw, req)
}

func writeJSON(rw http.ResponseWriter, code int, body any) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(code)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.Printf("запись ответа: %v", err)
	}
}
