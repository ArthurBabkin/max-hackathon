package api

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/assistant"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// historyLimit — сколько последних реплик чата показывает экран помощника.
const historyLimit = 50

type aiMessageDTO struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"`
	Text      string            `json:"text"`
	CardRefs  []store.AiCardRef `json:"card_refs"`
	Sources   []sourceDTO       `json:"sources"`
	Refused   bool              `json:"refused"`
	CreatedAt time.Time         `json:"created_at"`
}

func aiMessageOf(m store.AiMessage) aiMessageDTO {
	d := aiMessageDTO{ID: m.ID, Role: m.Role, Text: m.Text, CardRefs: m.CardRefs, Sources: []sourceDTO{},
		Refused: m.Refused, CreatedAt: m.CreatedAt}
	if d.CardRefs == nil {
		d.CardRefs = []store.AiCardRef{}
	}
	for _, s := range m.Sources {
		d.Sources = append(d.Sources, *sourceOf(&s))
	}
	return d
}

// maxChatTitleRunes — F59: своё название чата от 1 до 60 символов.
const maxChatTitleRunes = 60

var errChatNotFound = notFound("Чат не найден.")

// aiChatDTO — AiChat контракта.
type aiChatDTO struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
	LastMessageAt time.Time `json:"last_message_at"`
}

func aiChatOf(c store.AiChat) aiChatDTO {
	return aiChatDTO{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, LastMessageAt: c.LastMessageAt}
}

func aiExchangeOf(ex store.AiExchange) map[string]any {
	return map[string]any{"chat": aiChatOf(ex.Chat), "question": aiMessageOf(ex.Question), "answer": aiMessageOf(ex.Answer)}
}

// aiChats — GET /ai/chats (F58): свои чаты, свежие сверху.
func (s *Server) aiChats(w http.ResponseWriter, r *http.Request) error {
	chats, err := s.store.AiChats(r.Context(), me(r).MemberID)
	if err != nil {
		return err
	}
	items := make([]aiChatDTO, len(chats))
	for i, c := range chats {
		items[i] = aiChatOf(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

// aiChatOfPath — свой чат из пути. Чужой, несуществующий или не uuid — 404:
// о чужих чатах участник не узнаёт даже их существования (F37).
func (s *Server) aiChatOfPath(r *http.Request) (store.AiChat, error) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		return store.AiChat{}, errChatNotFound
	}
	c, err := s.store.AiChat(r.Context(), me(r).MemberID, id)
	if errors.Is(err, store.ErrNotFound) {
		return c, errChatNotFound
	}
	return c, err
}

// aiChatHistory — GET /ai/chats/{id}/messages (F37).
func (s *Server) aiChatHistory(w http.ResponseWriter, r *http.Request) error {
	c, err := s.aiChatOfPath(r)
	if err != nil {
		return err
	}
	msgs, err := s.store.AiChatMessages(r.Context(), me(r).MemberID, c.ID, historyLimit)
	if err != nil {
		return err
	}
	items := make([]aiMessageDTO, len(msgs))
	for i, m := range msgs {
		items[i] = aiMessageOf(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

var errRateLimited = &Error{http.StatusTooManyRequests, "RATE_LIMITED",
	"Слишком много вопросов подряд. Подождите минуту и спросите снова."}

// readQuestion — текст вопроса из тела: без пробелов по краям, не пустой и
// не длиннее 500 символов (ТЗ §6.4).
func readQuestion(r *http.Request) (string, error) {
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return "", err
	}
	question := strings.TrimSpace(body.Text)
	if question == "" {
		return "", badRequest("Напишите вопрос.")
	}
	if utf8.RuneCountInString(question) > assistant.MaxQuestionRunes {
		return "", badRequest("Вопрос длиннее 500 символов — сократите его.")
	}
	return question, nil
}

// answer — ответ помощника на вопрос в чате с историей history (F35, F36, F60).
func (s *Server) answer(r *http.Request, history []store.AiMessage, question string) (voice.Voice, store.Trajectory, store.AiMessage, error) {
	ctx, m := r.Context(), me(r)
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return voice.Voice{}, t, store.AiMessage{}, err
	}
	v := voice.New(voice.Role(m.Role), t.StudentName, m.FirstName)
	ans, err := s.assistant.Ask(ctx, v, t, history, question)
	return v, t, store.AiMessage{Text: ans.Text, CardRefs: ans.CardRefs, Sources: ans.Sources, Refused: ans.Refused}, err
}

// startAiChat — POST /ai/chats (F58, F59): новый чат начинается с вопроса и
// называется по дню этого вопроса в часовом поясе траектории.
func (s *Server) startAiChat(w http.ResponseWriter, r *http.Request) error {
	question, err := readQuestion(r)
	if err != nil {
		return err
	}
	m := me(r)
	if !s.aiLimit.allow(m.MemberID, s.now()) {
		return errRateLimited
	}
	v, t, ans, err := s.answer(r, nil, question)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(t.TZ)
	if err != nil {
		loc = moscow
	}
	title := v.T("ai.chatTitle", voice.Vars{"date": stages.Day(s.now().In(loc))})
	ex, err := s.store.StartAiChat(r.Context(), m.MemberID, title, question, ans)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, aiExchangeOf(ex))
	return nil
}

// askInAiChat — POST /ai/chats/{id}/messages: модель видит последние
// реплики этого чата (F60).
func (s *Server) askInAiChat(w http.ResponseWriter, r *http.Request) error {
	question, err := readQuestion(r)
	if err != nil {
		return err
	}
	c, err := s.aiChatOfPath(r)
	if err != nil {
		return err
	}
	m := me(r)
	if !s.aiLimit.allow(m.MemberID, s.now()) {
		return errRateLimited
	}
	history, err := s.store.AiChatMessages(r.Context(), m.MemberID, c.ID, assistant.HistoryMessages)
	if err != nil {
		return err
	}
	_, _, ans, err := s.answer(r, history, question)
	if err != nil {
		return err
	}
	ex, err := s.store.SaveAiExchange(r.Context(), m.MemberID, c.ID, question, ans)
	if errors.Is(err, store.ErrNotFound) {
		return errChatNotFound
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, aiExchangeOf(ex))
	return nil
}

// renameAiChat — PATCH /ai/chats/{id} (F59).
func (s *Server) renameAiChat(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		return badRequest("Напишите название чата.")
	}
	if utf8.RuneCountInString(title) > maxChatTitleRunes {
		return badRequest("Название длиннее 60 символов — сократите его.")
	}
	c, err := s.aiChatOfPath(r)
	if err != nil {
		return err
	}
	c, err = s.store.RenameAiChat(r.Context(), me(r).MemberID, c.ID, title)
	if errors.Is(err, store.ErrNotFound) {
		return errChatNotFound
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, aiChatOf(c))
	return nil
}

// limiter — корзина токенов на участника: burst вопросов подряд, дальше
// один в every. Живёт в памяти инстанса функции: в serverless каждый
// тёплый инстанс считает сам, это защита от случайного залпа и перерасхода
// модели, а не строгая квота.
type limiter struct {
	mu      sync.Mutex
	burst   float64
	every   time.Duration
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	at     time.Time
}

func newLimiter(burst int, every time.Duration) *limiter {
	return &limiter{burst: float64(burst), every: every, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) > 10000 {
			l.buckets = map[string]*bucket{}
		}
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+float64(now.Sub(b.at))/float64(l.every))
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
