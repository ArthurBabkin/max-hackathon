package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client — минимальный клиент Bot API MAX.
// Токен передаётся заголовком Authorization: передача в query не поддерживается.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		// Таймаут короче, чем таймаут функции: лучше отдать 200 в MAX
		// и потерять одну отправку, чем упереться в лимит выполнения.
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

type sendMessageRequest struct {
	Text string `json:"text"`
}

// SendMessage отправляет текст в чат. Лимит платформы — 4000 символов,
// 30 запросов в секунду всего и 2 в секунду на один чат.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	q := url.Values{}
	q.Set("chat_id", fmt.Sprintf("%d", chatID))

	body, err := json.Marshal(sendMessageRequest{Text: text})
	if err != nil {
		return fmt.Errorf("маршалинг сообщения: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/messages?"+q.Encode(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("сборка запроса: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("запрос к MAX: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MAX ответил %d", resp.StatusCode)
	}
	return nil
}
