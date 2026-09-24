// Package api — REST для мини-приложения. Одна функция на все ручки:
// внутренний роутер вместо функции на эндпоинт, чтобы не множить холодные старты.
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/assistant"
	"github.com/ArthurBabkin/max-hackathon/packages/core/auth"
	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/llm"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/version"
)

// BasePath — базовый путь контракта (servers[0].url в openapi.yaml).
const BasePath = "/api/v1"

type Deps struct {
	Store  *store.Store
	Config config.Config
	// CORSOrigins — источники, которым разрешён кросс-доменный доступ
	// (адрес мини-приложения в Object Storage). Локально пусто: прокси.
	CORSOrigins []string
	// Max — отправка уведомлений семье в чат бота (F41, F45, F46, F48).
	// nil — уведомления выключены (локально без токена).
	Max maxapi.Sender
	// LLM — модель помощника. nil — помощник отвечает только шаблоном
	// «данных нет» (нет POLZA_AI_API_KEY).
	LLM llm.Completer
	// Now подменяется в тестах; по умолчанию time.Now.
	Now func() time.Time
}

type Server struct {
	store     *store.Store
	cfg       config.Config
	policy    auth.Policy
	now       func() time.Time
	mux       *http.ServeMux
	notify    *notify.Notifier
	assistant *assistant.Assistant
	aiLimit   *limiter
}

// Sender — клиент MAX для уведомлений семье; без токена бота — nil,
// и уведомления молча выключены.
func Sender(cfg config.Config) maxapi.Sender {
	if cfg.MaxBotToken == "" {
		return nil
	}
	return maxapi.New(config.Get("MAX_API_BASE", maxapi.DefaultBaseURL), cfg.MaxBotToken)
}

// Model — модель помощника из окружения (POLZA_AI_API_KEY, LLM_BASE_URL,
// LLM_MODEL); без ключа — nil.
func Model() llm.Completer {
	key := config.Get("POLZA_AI_API_KEY", "")
	if key == "" {
		return nil
	}
	return llm.New(config.Get("LLM_BASE_URL", "https://polza.ai/api/v1"), key,
		config.Get("LLM_MODEL", "GigaChat/GigaChat-3-Pro"))
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
		notify: &notify.Notifier{Store: d.Store, Max: d.Max, BotName: d.Config.MaxBotName,
			BotID: d.Config.MaxBotID, ReminderHour: d.Config.ReminderHour},
		assistant: &assistant.Assistant{Store: d.Store, LLM: d.LLM},
		// Пять вопросов подряд, дальше один в 12 секунд — не больше пяти в минуту.
		aiLimit: newLimiter(5, 12*time.Second),
	}
	if s.now == nil {
		s.now = time.Now
	}
	s.routes()
	return s
}

// CheckMux проверяет, что ServeMux понимает шаблоны с методом («GET /health»).
// Рантайм Cloud Functions собирает функцию плагином к своему main-модулю, и
// дефолт GODEBUG берётся оттуда, а не из нашего go.mod: там включён старый
// роутер (httpmuxgo121=1). Он считает «GET /health» буквальным путём, и все
// запросы молча уходят в catch-all «/» с ответом 404. Лучше не стартовать
// вовсе — тогда проба выкладки увидит 502 и откатит версию.
func CheckMux() error {
	mux := http.NewServeMux()
	matched := false
	mux.HandleFunc("GET /probe", func(http.ResponseWriter, *http.Request) { matched = true })
	mux.ServeHTTP(discard{}, &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/probe"}})
	if !matched {
		return errors.New("ServeMux в режиме Go 1.21 не понимает шаблоны маршрутов: " +
			"задайте функции GODEBUG=httpmuxgo121=0")
	}
	return nil
}

type discard struct{}

func (discard) Header() http.Header         { return http.Header{} }
func (discard) Write(b []byte) (int, error) { return len(b), nil }
func (discard) WriteHeader(int)             {}

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
		// version — по нему видно, какой коммит сейчас в проде.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version.Commit()})
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
	s.mux.HandleFunc("GET /directions", s.authed(s.directions))
	s.mux.HandleFunc("GET /tracker", s.authed(s.getTracker))
	s.mux.HandleFunc("POST /tracker", s.authed(s.addToTracker))
	s.mux.HandleFunc("DELETE /tracker/{id}", s.authed(s.removeFromTracker))
	s.mux.HandleFunc("PUT /tracker/{id}/registered", s.authed(s.setRegistered(true)))
	s.mux.HandleFunc("DELETE /tracker/{id}/registered", s.authed(s.setRegistered(false)))
	s.mux.HandleFunc("GET /calendar", s.authed(s.calendar))
	s.mux.HandleFunc("GET /calendar/link", s.authed(s.calendarLink))
	// Без сессии: файл забирает браузер телефона по ссылке из /calendar/link.
	s.mux.HandleFunc("GET /calendar.ics", s.handle(s.calendarFile))
	s.mux.HandleFunc("POST /proposals", s.authed(s.propose))
	s.mux.HandleFunc("POST /proposals/{id}/accept", s.authed(s.resolveProposal(true)))
	s.mux.HandleFunc("POST /proposals/{id}/decline", s.authed(s.resolveProposal(false)))
	s.mux.HandleFunc("GET /family", s.authed(s.family))
	s.mux.HandleFunc("POST /family/invites", s.authed(s.createInvite))
	s.mux.HandleFunc("DELETE /family/members/{id}", s.authed(s.removeMember))
	s.mux.HandleFunc("POST /family/leave", s.authed(s.leave))
	s.mux.HandleFunc("GET /ai/chats", s.authed(s.aiChats))
	s.mux.HandleFunc("POST /ai/chats", s.authed(s.startAiChat))
	s.mux.HandleFunc("PATCH /ai/chats/{id}", s.authed(s.renameAiChat))
	s.mux.HandleFunc("GET /ai/chats/{id}/messages", s.authed(s.aiChatHistory))
	s.mux.HandleFunc("POST /ai/chats/{id}/messages", s.authed(s.askInAiChat))

	// Ручки контракта, которые ещё не написаны, честно отвечают 501 в
	// формате ошибки — фронт отличает «не готово» от «сломалось».
	s.mux.HandleFunc("/", s.handle(func(w http.ResponseWriter, r *http.Request) error {
		return notFound("Нет такого адреса.")
	}))
}
