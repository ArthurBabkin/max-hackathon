// Package api — REST для мини-приложения. Одна функция на все ручки:
// внутренний роутер вместо функции на эндпоинт, чтобы не множить холодные старты.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/auth"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

// BasePath — базовый путь контракта (servers[0].url в openapi.yaml).
const BasePath = "/api/v1"

type Deps struct {
	Store  *store.Store
	Config config.Config
	// CORSOrigins — источники, которым разрешён кросс-доменный доступ
	// (адрес мини-приложения в Object Storage). Локально пусто: прокси.
	CORSOrigins []string
	// Now подменяется в тестах; по умолчанию time.Now.
	Now func() time.Time
}

type Server struct {
	store  *store.Store
	cfg    config.Config
	policy auth.Policy
	now    func() time.Time
	mux    *http.ServeMux
}

// New собирает обработчик со всеми middleware.
func New(d Deps) http.Handler {
	s := newServer(d)
	return s.handler(d.CORSOrigins)
}

func newServer(d Deps) *Server {
	s := &Server{
		store: d.Store,
		cfg:   d.Config,
		policy: auth.Policy{
			BotToken:            d.Config.MaxBotToken,
			DevUnsignedInitData: d.Config.DevUnsignedInitData,
			AppEnv:              d.Config.AppEnv,
		},
		now: d.Now,
		mux: http.NewServeMux(),
	}
	if s.now == nil {
		s.now = time.Now
	}
	s.routes()
	return s
}

func (s *Server) handler(corsOrigins []string) http.Handler {
	return stripPrefix(BasePath, cors(corsOrigins, requestID(logRequests(s.mux))))
}

// handlerFunc возвращает ошибку вместо того, чтобы писать её сам: так каждая
// ручка отвечает об ошибке в одном формате контракта.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			e, known := asError(err)
			if !known {
				slog.Error("ошибка обработчика", "request_id", reqID(r.Context()),
					"route", r.Pattern, "err", err)
			}
			writeError(w, e)
		}
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /session", s.handle(s.createSession))
	s.mux.HandleFunc("GET /home", s.authed(s.home))
	s.mux.HandleFunc("GET /profile", s.authed(s.getProfile))
	s.mux.HandleFunc("PATCH /profile", s.authed(s.patchProfile))
	s.mux.HandleFunc("PUT /profile/universities", s.authed(s.putUniversities))
	s.mux.HandleFunc("GET /recommendations", s.authed(s.recommendations))
	s.mux.HandleFunc("GET /olympiads", s.authed(s.olympiads))
	s.mux.HandleFunc("GET /olympiads/{id}", s.authed(s.olympiad))
	s.mux.HandleFunc("GET /universities", s.authed(s.universities))
	s.mux.HandleFunc("GET /universities/{id}", s.authed(s.university))
	s.mux.HandleFunc("GET /tracker", s.authed(s.getTracker))
	s.mux.HandleFunc("POST /tracker", s.authed(s.addToTracker))
	s.mux.HandleFunc("DELETE /tracker/{id}", s.authed(s.removeFromTracker))
	s.mux.HandleFunc("PUT /tracker/{id}/registered", s.authed(s.setRegistered(true)))
	s.mux.HandleFunc("DELETE /tracker/{id}/registered", s.authed(s.setRegistered(false)))
	s.mux.HandleFunc("GET /calendar", s.authed(s.calendar))

	// Ручки контракта, которые ещё не написаны, честно отвечают 501 в
	// формате ошибки — фронт отличает «не готово» от «сломалось».
	s.mux.HandleFunc("/", s.handle(func(w http.ResponseWriter, r *http.Request) error {
		return notFound("Нет такого адреса.")
	}))
}
