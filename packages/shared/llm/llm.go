// Package llm — клиент OpenAI-совместимого chat/completions. Им ходим в
// GigaChat через шлюз polza.ai (LLM_BASE_URL, LLM_MODEL, POLZA_AI_API_KEY).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message — реплика диалога: system, user или assistant.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Completer — то, что нужно помощнику; в тестах подменяется.
type Completer interface {
	// JSON просит у модели ответ одним JSON-объектом и возвращает его текст.
	JSON(ctx context.Context, messages []Message) (string, error)
}

type Client struct {
	baseURL, apiKey, model string
	http                   *http.Client
}

// New — пустой apiKey означает «помощник без модели»: вызывающий обязан
// проверить Enabled и ответить шаблоном.
func New(baseURL, apiKey, model string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model,
		http: &http.Client{Timeout: 25 * time.Second}}
}

func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// ErrEmpty — модель ответила без текста.
var ErrEmpty = errors.New("llm: пустой ответ модели")

func (c *Client) JSON(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":           c.model,
		"messages":        messages,
		"temperature":     0.1,
		"max_tokens":      700,
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("llm: чтение ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Тело ошибки шлюза не логируем целиком: там может быть эхо запроса.
		return "", fmt.Errorf("llm: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("llm: разбор ответа: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", ErrEmpty
	}
	return out.Choices[0].Message.Content, nil
}
