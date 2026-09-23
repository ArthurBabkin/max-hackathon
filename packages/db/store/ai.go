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

// AiChat — чат участника с помощником (F58).
type AiChat struct {
	ID            string
	Title         string
	CreatedAt     time.Time
	LastMessageAt time.Time
}

// AiExchange — вопрос и ответ, сохранённые в чат.
type AiExchange struct {
	Chat     AiChat
	Question AiMessage
	Answer   AiMessage
}

const aiChatCols = `id::text, title, created_at, last_message_at`

func scanAiChat(r rowScanner) (AiChat, error) {
	var c AiChat
	return c, r.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.LastMessageAt)
}

// AiChats — чаты участника, свежие сверху.
func (s *Store) AiChats(ctx context.Context, memberID string) ([]AiChat, error) {
	rows, err := s.db.Query(ctx, `SELECT `+aiChatCols+` FROM ai_chats WHERE member_id = $1
		ORDER BY last_message_at DESC, created_at DESC, id`, memberID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanAiChat)
}

// AiChat — чат участника. Чужой чат для него не существует (F37): ErrNotFound.
func (s *Store) AiChat(ctx context.Context, memberID, chatID string) (AiChat, error) {
	c, err := scanAiChat(s.db.QueryRow(ctx, `SELECT `+aiChatCols+` FROM ai_chats
		WHERE id = $1 AND member_id = $2`, chatID, memberID))
	return c, wrap(err)
}

// RenameAiChat меняет название своего чата. Место в списке не меняется:
// порядок задаёт последняя реплика, а не правка.
func (s *Store) RenameAiChat(ctx context.Context, memberID, chatID, title string) (AiChat, error) {
	c, err := scanAiChat(s.db.QueryRow(ctx, `UPDATE ai_chats SET title = $3
		WHERE id = $1 AND member_id = $2 RETURNING `+aiChatCols, chatID, memberID, title))
	return c, wrap(err)
}

// AiChatMessages — последние limit реплик своего чата, от старых к новым.
func (s *Store) AiChatMessages(ctx context.Context, memberID, chatID string, limit int) ([]AiMessage, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, role, text, card_refs, sources, refused, created_at FROM (
		  SELECT id::text, role, text, card_refs, sources, refused, created_at FROM ai_messages
		  WHERE chat_id = $1 AND member_id = $2 ORDER BY created_at DESC, role DESC LIMIT $3) m
		ORDER BY created_at, role DESC`, chatID, memberID, limit)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanAiMessage)
}

// StartAiChat создаёт чат вместе с первым вопросом и ответом: пустых чатов
// в базе не бывает.
func (s *Store) StartAiChat(ctx context.Context, memberID, title, question string, answer AiMessage) (AiExchange, error) {
	var ex AiExchange
	err := s.Tx(ctx, func(tx *Store) error {
		var err error
		ex.Chat, err = scanAiChat(tx.db.QueryRow(ctx, `INSERT INTO ai_chats (member_id, title)
			VALUES ($1, $2) RETURNING `+aiChatCols, memberID, title))
		if err != nil {
			return wrap(err)
		}
		ex.Question, ex.Answer, err = tx.insertExchange(ctx, memberID, ex.Chat.ID, question, answer)
		return err
	})
	return ex, err
}

// SaveAiExchange дописывает вопрос и ответ в свой чат и поднимает его в
// списке. Чужой чат — ErrNotFound, в него ничего не пишется.
func (s *Store) SaveAiExchange(ctx context.Context, memberID, chatID, question string, answer AiMessage) (AiExchange, error) {
	var ex AiExchange
	err := s.Tx(ctx, func(tx *Store) error {
		var err error
		ex.Chat, err = scanAiChat(tx.db.QueryRow(ctx, `UPDATE ai_chats SET last_message_at = now()
			WHERE id = $1 AND member_id = $2 RETURNING `+aiChatCols, chatID, memberID))
		if err != nil {
			return wrap(err)
		}
		ex.Question, ex.Answer, err = tx.insertExchange(ctx, memberID, chatID, question, answer)
		return err
	})
	return ex, err
}

// insertExchange пишет пару одной транзакцией: в истории не бывает вопроса
// без ответа. Время вопроса и ответа одно — порядок задаёт роль (user
// раньше assistant).
func (s *Store) insertExchange(ctx context.Context, memberID, chatID, question string, answer AiMessage) (q, a AiMessage, err error) {
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
	row := s.db.QueryRow(ctx, `
		INSERT INTO ai_messages (member_id, chat_id, role, text) VALUES ($1, $2, 'user', $3)
		RETURNING id::text, role, text, card_refs, sources, refused, created_at`, memberID, chatID, question)
	if q, err = scanAiMessage(row); err != nil {
		return q, a, wrap(err)
	}
	row = s.db.QueryRow(ctx, `
		INSERT INTO ai_messages (member_id, chat_id, role, text, card_refs, sources, refused)
		VALUES ($1, $2, 'assistant', $3, $4, $5, $6)
		RETURNING id::text, role, text, card_refs, sources, refused, created_at`,
		memberID, chatID, answer.Text, refs, sources, answer.Refused)
	if a, err = scanAiMessage(row); err != nil {
		return q, a, wrap(err)
	}
	return q, a, nil
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
