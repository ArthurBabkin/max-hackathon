package store

import (
	"context"
	"encoding/json"
	"time"
)

// AiCardRef — кнопка «Карточка «…»» под ответом помощника (F35).
type AiCardRef struct {
	Type  string `json:"type"` // olympiad | university
	ID    string `json:"id"`   // olympiad_profile_id или universities.id
	Title string `json:"title"`
}

// AiMessage — реплика личного чата участника с помощником (F37).
type AiMessage struct {
	ID        string
	Role      string // user | assistant
	Text      string
	CardRefs  []AiCardRef
	Sources   []Source
	Refused   bool
	CreatedAt time.Time
}

// AiMessages — последние limit реплик участника, от старых к новым.
func (s *Store) AiMessages(ctx context.Context, memberID string, limit int) ([]AiMessage, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, role, text, card_refs, sources, refused, created_at FROM (
		  SELECT id::text, role, text, card_refs, sources, refused, created_at FROM ai_messages
		  WHERE member_id = $1 ORDER BY created_at DESC, role DESC LIMIT $2) m
		ORDER BY created_at, role DESC`, memberID, limit)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanAiMessage)
}

func scanAiMessage(r rowScanner) (AiMessage, error) {
	var m AiMessage
	var refs, sources []byte
	if err := r.Scan(&m.ID, &m.Role, &m.Text, &refs, &sources, &m.Refused, &m.CreatedAt); err != nil {
		return m, err
	}
	if err := json.Unmarshal(refs, &m.CardRefs); err != nil {
		return m, err
	}
	return m, json.Unmarshal(sources, &m.Sources)
}

// SaveAiExchange сохраняет вопрос и ответ одной транзакцией: в истории не
// бывает вопроса без ответа. Время вопроса и ответа одно — порядок задаёт
// роль (user раньше assistant).
func (s *Store) SaveAiExchange(ctx context.Context, memberID, question string, answer AiMessage) (q, a AiMessage, err error) {
	if answer.CardRefs == nil {
		answer.CardRefs = []AiCardRef{}
	}
	if answer.Sources == nil {
		answer.Sources = []Source{}
	}
	refs, err := json.Marshal(answer.CardRefs)
	if err != nil {
		return q, a, err
	}
	sources, err := json.Marshal(answer.Sources)
	if err != nil {
		return q, a, err
	}
	err = s.Tx(ctx, func(tx *Store) error {
		row := tx.db.QueryRow(ctx, `
			INSERT INTO ai_messages (member_id, role, text) VALUES ($1, 'user', $2)
			RETURNING id::text, role, text, card_refs, sources, refused, created_at`, memberID, question)
		if q, err = scanAiMessage(row); err != nil {
			return wrap(err)
		}
		row = tx.db.QueryRow(ctx, `
			INSERT INTO ai_messages (member_id, role, text, card_refs, sources, refused)
			VALUES ($1, 'assistant', $2, $3, $4, $5)
			RETURNING id::text, role, text, card_refs, sources, refused, created_at`,
			memberID, answer.Text, refs, sources, answer.Refused)
		if a, err = scanAiMessage(row); err != nil {
			return wrap(err)
		}
		return nil
	})
	return q, a, err
}

// Named — id и название для поиска упоминаний в вопросе помощнику.
type Named struct {
	ID   string
	Name string
}

// OlympiadNames — все олимпиады базы: помощник ищет их в тексте вопроса.
func (s *Store) OlympiadNames(ctx context.Context) ([]Named, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name FROM olympiads ORDER BY id`)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Named, error) {
		var n Named
		return n, r.Scan(&n.ID, &n.Name)
	})
}

// UniversityRules — источник «правила приёма» вуза для отказа помощника.
func (s *Store) UniversityRules(ctx context.Context, universityID string) (*Source, error) {
	var short string
	var url *string
	var verified *time.Time
	err := s.db.QueryRow(ctx, `SELECT short_name, rules_url, rules_verified_at FROM universities WHERE id = $1`,
		universityID).Scan(&short, &url, &verified)
	if err != nil || url == nil || *url == "" {
		return nil, wrap(err)
	}
	return &Source{ID: "rules-" + universityID, Kind: "rules", Title: short + ": правила приёма", URL: *url,
		VerifiedAt: verified}, nil
}

// OrderSource — приказ о перечне олимпиад: общий первоисточник по льготам.
func (s *Store) OrderSource(ctx context.Context) (*Source, error) {
	var src Source
	err := s.db.QueryRow(ctx, `
		SELECT id, kind, title, url, verified_at FROM sources WHERE kind = 'order' ORDER BY id LIMIT 1`).
		Scan(&src.ID, &src.Kind, &src.Title, &src.URL, &src.VerifiedAt)
	if err != nil {
		return nil, wrap(err)
	}
	return &src, nil
}
