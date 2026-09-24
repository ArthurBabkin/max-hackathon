// Package jev — клиент классификатора TypeSafe Jev (typesafe/jev) через шлюз
// polza.ai: POST {LLM_BASE_URL}/systemone, тот же ключ POLZA_AI_API_KEY. Вопрос
// типа choice выбирает один класс из описанных, в ответе — вероятность
// каждого класса. Платятся только входные токены: описания классов.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Choice — вопрос «какой из классов»: класс → описание, по которому Jev
// узнаёт его в тексте.
type Choice struct {
	Instructions string
	Classes      map[string]string
}

// Classifier — то, что нужно помощнику; в тестах подменяется. Ответ —
// вероятности классов по каждому вопросу.
type Classifier interface {
	Classify(ctx context.Context, state map[string]string, questions map[string]Choice) (map[string]map[string]float64, error)
}

type Client struct {
	baseURL, apiKey string
	http            *http.Client
}

// New — пустой apiKey означает «без классификатора»: вызывающий проверяет
// Enabled.
func New(baseURL, apiKey string) *Client {
	// Jev отвечает за 1–3 секунды. Функция api живёт 30 секунд, из них до 25 —
	// ответ модели: дольше 4 секунд не ждём и отвечаем по поиску названий.
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: &http.Client{Timeout: 4 * time.Second}}
}

func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions,omitempty"`
	Criteria     map[string]string `json:"criteria"`
}

func (c *Client) Classify(ctx context.Context, state map[string]string, questions map[string]Choice) (map[string]map[string]float64, error) {
	qs := make(map[string]question, len(questions))
	for name, q := range questions {
		qs[name] = question{Type: "choice", Instructions: q.Instructions, Criteria: q.Classes}
	}
	body, err := json.Marshal(map[string]any{"model": "typesafe/jev", "state": state, "questions": qs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/systemone", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("jev: чтение ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Тело ошибки шлюза не логируем: там может быть эхо вопроса.
		return nil, fmt.Errorf("jev: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Answers map[string]struct {
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("jev: разбор ответа: %w", err)
	}
	probs := make(map[string]map[string]float64, len(questions))
	for name := range questions {
		a, ok := out.Answers[name]
		if !ok || len(a.Probabilities) == 0 {
			return nil, fmt.Errorf("jev: нет ответа на вопрос %q", name)
		}
		probs[name] = a.Probabilities
	}
	return probs, nil
}
