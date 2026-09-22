package maxapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL — с 19.07.2026 единственный допустимый адрес Bot API.
const DefaultBaseURL = "https://platform-api2.max.ru"

// Сертификат platform-api2.max.ru выпущен «Russian Trusted Sub CA», а тот —
// корнем Минцифры, которого нет в стандартных образах Linux. Сервер отдаёт
// цепочку до Sub CA, поэтому доверять нужно только корню.
// SHA-256: D2:6D:2D:02:31:B7:C3:9F:92:CC:73:85:12:BA:54:10:35:19:E4:40:5D:68:B5:BD:70:3E:97:88:CA:8E:CF:31
//
//go:embed russian_trusted_root_ca.pem
var russianTrustedRootCA []byte

// rootPool — системные корни плюс корень Минцифры.
func rootPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(russianTrustedRootCA)
	return pool
}

// Sender — то, что бот, воркер и API делают с чатом. Интерфейс нужен
// тестам: fake из maxtest записывает сообщения вместо сети.
type Sender interface {
	Send(ctx context.Context, userID int64, m NewMessage) (mid string, err error)
	Edit(ctx context.Context, mid string, m NewMessage) error
	Answer(ctx context.Context, callbackID string, a CallbackAnswer) error
}

// Error — ответ MAX с кодом, отличным от 200, или success: false.
type Error struct {
	Status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("MAX %d %s: %s", e.Status, e.Code, e.Message)
}

// IsBlocked — пользователь остановил бота или удалил диалог: писать ему
// бессмысленно, повтор не поможет.
func IsBlocked(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.Status == http.StatusForbidden || e.Status == http.StatusNotFound)
}

// Client — клиент Bot API. Токен — в заголовке Authorization без префикса:
// передача в query больше не поддерживается.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	// Лимиты платформы: 30 запросов в секунду на бота и 2 в секунду на чат.
	// Интервал между запросами держит общий лимит с запасом; упёрлись в
	// лимит чата — 429 и повтор с паузой.
	limit   *interval
	retries int
	pause   func(attempt int) time.Duration
}

func New(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			// Таймаут короче таймаута функции: лучше отдать MAX 200 и
			// потерять одну отправку, чем упереться в лимит выполнения.
			// Long polling задаёт свой таймаут через контекст.
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSClientConfig:     &tls.Config{RootCAs: rootPool(), MinVersion: tls.VersionTLS12},
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		limit:   &interval{every: time.Second / 25},
		retries: 3,
		pause:   func(attempt int) time.Duration { return time.Duration(attempt) * time.Second },
	}
}

// Send — POST /messages?user_id=: сообщение в личный диалог с ботом.
func (c *Client) Send(ctx context.Context, userID int64, m NewMessage) (string, error) {
	var out struct {
		Message Message `json:"message"`
	}
	q := url.Values{"user_id": {strconv.FormatInt(userID, 10)}}
	if err := c.do(ctx, http.MethodPost, "/messages", q, m, &out); err != nil {
		return "", err
	}
	return out.Message.Body.MID, nil
}

// Edit — PUT /messages?message_id=. Сообщение с клавиатурой в диалоге
// редактируется без ограничения по давности.
func (c *Client) Edit(ctx context.Context, mid string, m NewMessage) error {
	return c.doSuccess(ctx, http.MethodPut, "/messages", url.Values{"message_id": {mid}}, m)
}

func (c *Client) Delete(ctx context.Context, mid string) error {
	return c.doSuccess(ctx, http.MethodDelete, "/messages", url.Values{"message_id": {mid}}, nil)
}

// Answer — POST /answers?callback_id=: подтверждает нажатие, заменяет
// сообщение с кнопкой и/или показывает всплывающее уведомление.
func (c *Client) Answer(ctx context.Context, callbackID string, a CallbackAnswer) error {
	return c.doSuccess(ctx, http.MethodPost, "/answers", url.Values{"callback_id": {callbackID}}, a)
}

func (c *Client) Me(ctx context.Context) (BotInfo, error) {
	var out BotInfo
	return out, c.do(ctx, http.MethodGet, "/me", nil, nil, &out)
}

// SetCommands — PATCH /me/commands: подсказки при вводе «/» (F51).
func (c *Client) SetCommands(ctx context.Context, cmds []Command) error {
	return c.do(ctx, http.MethodPatch, "/me/commands", nil, map[string]any{"commands": cmds}, nil)
}

// Subscribe — POST /subscriptions. Адрес только https на 443-м порту;
// секрет MAX присылает в X-Max-Bot-Api-Secret.
func (c *Client) Subscribe(ctx context.Context, webhookURL, secret string, types []string) error {
	body := map[string]any{"url": webhookURL, "update_types": types}
	if secret != "" {
		body["secret"] = secret
	}
	return c.doSuccess(ctx, http.MethodPost, "/subscriptions", nil, body)
}

func (c *Client) Subscriptions(ctx context.Context) ([]Subscription, error) {
	var out struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	return out.Subscriptions, c.do(ctx, http.MethodGet, "/subscriptions", nil, nil, &out)
}

func (c *Client) Unsubscribe(ctx context.Context, webhookURL string) error {
	return c.doSuccess(ctx, http.MethodDelete, "/subscriptions", url.Values{"url": {webhookURL}}, nil)
}

// Updates — GET /updates, long polling для локальной разработки. Пока есть
// подписка на вебхук, MAX обновлений сюда не отдаёт.
func (c *Client) Updates(ctx context.Context, marker *int64, timeout time.Duration, types []string) (UpdateList, error) {
	q := url.Values{"timeout": {strconv.Itoa(int(timeout.Seconds()))}, "limit": {"100"}}
	if marker != nil {
		q.Set("marker", strconv.FormatInt(*marker, 10))
	}
	if len(types) > 0 {
		q.Set("types", strings.Join(types, ","))
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancel()
	var out UpdateList
	return out, c.doWith(ctx, &http.Client{Transport: c.http.Transport}, http.MethodGet, "/updates", q, nil, &out)
}

// doSuccess — для методов, которые отвечают {success, message}: 200 с
// success: false — тоже ошибка.
func (c *Client) doSuccess(ctx context.Context, method, path string, q url.Values, body any) error {
	var out struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if err := c.do(ctx, method, path, q, body, &out); err != nil {
		return err
	}
	if out.Success != nil && !*out.Success {
		return &Error{Status: http.StatusOK, Code: "success.false", Message: out.Message}
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	return c.doWith(ctx, c.http, method, path, q, body, out)
}

func (c *Client) doWith(ctx context.Context, hc *http.Client, method, path string, q url.Values, body, out any) error {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return fmt.Errorf("маршалинг тела %s: %w", path, err)
		}
	}
	target := c.baseURL + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		if err := c.limit.wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("сборка запроса %s: %w", path, err)
		}
		req.Header.Set("Authorization", c.token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := hc.Do(req)
		if err != nil {
			// Текст ошибки net/http содержит адрес, но не заголовки — токен не утечёт.
			return fmt.Errorf("запрос к MAX %s: %w", path, err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("чтение ответа MAX %s: %w", path, err)
		}
		// 429 значит «запрос не принят»: повторить безопасно даже отправку.
		// 5xx не повторяем — сообщение могло уйти, и второе было бы дублем.
		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.retries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.pause(attempt + 1)):
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			e := &Error{Status: resp.StatusCode}
			_ = json.Unmarshal(data, e)
			return e
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("разбор ответа MAX %s: %w", path, err)
		}
		return nil
	}
}

// interval — минимальный промежуток между запросами одного процесса.
// В serverless лимит на инстанс, и это осознанно: инстансов у бота мало,
// а распределённый лимитер для MVP — переусложнение.
type interval struct {
	mu    sync.Mutex
	next  time.Time
	every time.Duration
}

func (l *interval) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.every)
	l.mu.Unlock()
	if d := time.Until(at); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	return nil
}
