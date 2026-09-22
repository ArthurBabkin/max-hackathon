package api

import (
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/assistant"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// historyLimit — сколько последних реплик показывает чат помощника.
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

// aiHistory — GET /ai/messages (F37): только свой чат.
func (s *Server) aiHistory(w http.ResponseWriter, r *http.Request) error {
	msgs, err := s.store.AiMessages(r.Context(), me(r).MemberID, historyLimit)
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

// askAI — POST /ai/messages (F35, F36).
func (s *Server) askAI(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	question := strings.TrimSpace(body.Text)
	if question == "" {
		return badRequest("Напишите вопрос.")
	}
	if utf8.RuneCountInString(question) > assistant.MaxQuestionRunes {
		return badRequest("Вопрос длиннее 500 символов — сократите его.")
	}
	if !s.aiLimit.allow(m.MemberID, s.now()) {
		return errRateLimited
	}
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	v := voice.New(voice.Role(m.Role), t.StudentName, m.FirstName)
	ans, err := s.assistant.Ask(ctx, v, t, question)
	if err != nil {
		return err
	}
	q, a, err := s.store.SaveAiExchange(ctx, m.MemberID, question, store.AiMessage{
		Text: ans.Text, CardRefs: ans.CardRefs, Sources: ans.Sources, Refused: ans.Refused})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"question": aiMessageOf(q), "answer": aiMessageOf(a)})
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
