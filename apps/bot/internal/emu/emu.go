// Package emu — эмулятор MAX для проверки сценариев бота без аккаунта в
// мессенджере: веб-чат, в котором нажимаются inline-кнопки, отправляются
// /start с параметром, текст и геопозиция.
//
// Бот работает в том же процессе: действие в чате превращается в
// maxapi.Update и сразу уходит в bot.Handle, а ответы бота через Sender
// ложатся в таблицы emu_* вместо Bot API. Состояние целиком в Postgres,
// поэтому эмулятор живёт и в Cloud Functions, где инстансов несколько и
// память между вызовами не гарантирована.
package emu

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

//go:embed index.html
var indexHTML []byte

// Персонажи демо-траектории (packages/db/migrations-demo) — есть в списке
// сразу. Новые пользователи получают id из того же дев-диапазона.
var defaultUsers = []User{{900000001, "Артём"}, {900000002, "Ольга"}}

const firstNewUser = 900000100

// schema — таблицы эмулятора рядом с таблицами приложения, в той же схеме
// (search_path из DATABASE_URL). Создаются при старте, не миграциями:
// в прод-схему эмулятор не попадает.
const schema = `
CREATE TABLE IF NOT EXISTS emu_users (
	user_id    bigint PRIMARY KEY,
	first_name text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS emu_messages (
	id         bigserial PRIMARY KEY,
	mid        text NOT NULL UNIQUE,
	user_id    bigint NOT NULL,
	from_bot   boolean NOT NULL,
	text       text NOT NULL DEFAULT '',
	keyboard   jsonb,
	location   jsonb,
	created_at timestamptz NOT NULL DEFAULT now(),
	edited     boolean NOT NULL DEFAULT false,
	deleted    boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS emu_messages_user ON emu_messages (user_id, id);
CREATE TABLE IF NOT EXISTS emu_toasts (
	id         bigserial PRIMARY KEY,
	user_id    bigint NOT NULL,
	kind       text NOT NULL,
	text       text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS emu_callbacks (
	callback_id text PRIMARY KEY,
	user_id     bigint NOT NULL,
	mid         text NOT NULL,
	created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS emu_stopped (user_id bigint PRIMARY KEY);
`

type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
}

type Message struct {
	MID      string          `json:"mid"`
	FromBot  bool            `json:"from_bot"`
	Text     string          `json:"text"`
	Keyboard maxapi.Keyboard `json:"keyboard,omitempty"`
	Location []float64       `json:"location,omitempty"`
	Time     int64           `json:"time"`
	Edited   bool            `json:"edited,omitempty"`
	Deleted  bool            `json:"deleted,omitempty"`
}

type Toast struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"` // notification | system | error
	Text string `json:"text"`
}

// Config — то, что эмулятор знает о боте и окружении.
type Config struct {
	Bot    bot.Config
	Key    string // EMU_KEY: без него чат открыт всем, у кого есть адрес
	WebApp string // адрес мини-приложения для кнопок open_app; пусто — не открывать
}

type Emu struct {
	pool *pgxpool.Pool
	bot  *bot.Bot
	cfg  Config

	initMu sync.Mutex
	ready  bool
}

// New — эмулятор поверх пула. Бот получает Sender эмулятора вместо клиента MAX.
func New(pool *pgxpool.Pool, cfg Config) *Emu {
	e := &Emu{pool: pool, cfg: cfg}
	e.bot = bot.New(store.New(pool), sender{e}, cfg.Bot)
	return e
}

// FromEnv: DATABASE_URL, MAX_BOT_NAME, MAX_BOT_ID, REMINDER_HOUR, EMU_KEY, EMU_WEB_APP.
func FromEnv(pool *pgxpool.Pool) (*Emu, error) {
	id, err := config.BotIdentity()
	if err != nil {
		return nil, err
	}
	hour, err := config.ReminderHour()
	if err != nil {
		return nil, err
	}
	return New(pool, Config{
		Bot:    bot.Config{BotName: id.Name, BotID: id.ID, ReminderHour: hour},
		Key:    config.Get("EMU_KEY", ""),
		WebApp: config.Get("EMU_WEB_APP", ""),
	}), nil
}

// init — таблицы эмулятора. Под advisory-блокировкой: параллельный
// CREATE TABLE IF NOT EXISTS с bigserial из двух инстансов падает на
// уникальности имени последовательности.
func (e *Emu) init(ctx context.Context) error {
	e.initMu.Lock()
	defer e.initMu.Unlock()
	if e.ready {
		return nil
	}
	err := pgx.BeginFunc(ctx, e.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7412001)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, schema); err != nil {
			return err
		}
		for _, u := range defaultUsers {
			if _, err := tx.Exec(ctx, `INSERT INTO emu_users (user_id, first_name) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, u.UserID, u.FirstName); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// Следующий запрос попробует снова: сбой мог быть сетевым.
		return fmt.Errorf("таблицы эмулятора: %w", err)
	}
	e.ready = true
	return nil
}

func randomID(prefix string) string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (e *Emu) toast(ctx context.Context, userID int64, kind, text string) error {
	_, err := e.pool.Exec(ctx, `INSERT INTO emu_toasts (user_id, kind, text) VALUES ($1, $2, $3)`, userID, kind, text)
	return err
}

// --- Sender: то, что бот делает с чатом --------------------------------------

type sender struct{ e *Emu }

var _ maxapi.Sender = sender{}

func keyboardJSON(m maxapi.NewMessage) []byte {
	kb := m.Keyboard()
	if len(kb) == 0 {
		return nil
	}
	raw, _ := json.Marshal(kb)
	return raw
}

func (s sender) Send(ctx context.Context, userID int64, m maxapi.NewMessage) (string, error) {
	var stopped bool
	if err := s.e.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM emu_stopped WHERE user_id = $1)`, userID).Scan(&stopped); err != nil {
		return "", err
	}
	if stopped {
		// Как настоящий MAX: пользователь остановил бота — 403.
		return "", &maxapi.Error{Status: http.StatusForbidden, Code: "chat.denied", Message: "пользователь остановил бота"}
	}
	mid := randomID("mid.bot.")
	_, err := s.e.pool.Exec(ctx, `INSERT INTO emu_messages (mid, user_id, from_bot, text, keyboard) VALUES ($1, $2, true, $3, $4)`,
		mid, userID, m.Text, keyboardJSON(m))
	return mid, err
}

// edit — правка по семантике MAX: attachments null оставляет клавиатуру,
// пустой массив убирает.
func (s sender) edit(ctx context.Context, mid string, m maxapi.NewMessage) error {
	tag, err := s.e.pool.Exec(ctx, `UPDATE emu_messages SET text = $2, edited = true,
		keyboard = CASE WHEN $3 THEN $4::jsonb ELSE keyboard END
		WHERE mid = $1 AND NOT deleted`, mid, m.Text, m.Attachments != nil, keyboardJSON(m))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &maxapi.Error{Status: http.StatusNotFound, Code: "not.found", Message: "сообщение не найдено"}
	}
	return nil
}

func (s sender) Edit(ctx context.Context, mid string, m maxapi.NewMessage) error {
	return s.edit(ctx, mid, m)
}

func (s sender) Answer(ctx context.Context, callbackID string, a maxapi.CallbackAnswer) error {
	var userID int64
	var mid string
	err := s.e.pool.QueryRow(ctx, `SELECT user_id, mid FROM emu_callbacks WHERE callback_id = $1`, callbackID).Scan(&userID, &mid)
	if errors.Is(err, pgx.ErrNoRows) {
		return &maxapi.Error{Status: http.StatusNotFound, Code: "not.found", Message: "нет такого callback_id"}
	}
	if err != nil {
		return err
	}
	if a.Message != nil {
		if err := s.edit(ctx, mid, *a.Message); err != nil {
			return err
		}
	}
	if a.Notification != "" {
		return s.e.toast(ctx, userID, "notification", a.Notification)
	}
	return nil
}

// --- Действия пользователя ---------------------------------------------------

type action struct {
	UserID  int64   `json:"user_id"`
	Text    string  `json:"text"`
	Payload *string `json:"payload"`
	MID     string  `json:"mid"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Name    string  `json:"first_name"`
}

func (e *Emu) user(ctx context.Context, id int64) (maxapi.User, error) {
	u := maxapi.User{UserID: id}
	err := e.pool.QueryRow(ctx, `SELECT first_name FROM emu_users WHERE user_id = $1`, id).Scan(&u.FirstName)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errBadRequest("нет такого пользователя эмулятора")
	}
	return u, err
}

// message — сообщение из ленты в формате Bot API.
func (e *Emu) message(ctx context.Context, mid string, u maxapi.User) (*maxapi.Message, error) {
	var fromBot bool
	var text string
	var ts time.Time
	var loc []float64
	err := e.pool.QueryRow(ctx, `SELECT from_bot, text, created_at, location FROM emu_messages WHERE mid = $1 AND user_id = $2`,
		mid, u.UserID).Scan(&fromBot, &text, &ts, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errBadRequest("нет такого сообщения")
	}
	if err != nil {
		return nil, err
	}
	botUser := maxapi.User{UserID: e.cfg.Bot.BotID, FirstName: "Траектория", Username: e.cfg.Bot.BotName, IsBot: true}
	m := &maxapi.Message{Timestamp: ts.UnixMilli(), Body: maxapi.MessageBody{MID: mid, Text: text}}
	if fromBot {
		m.Sender = &botUser
		m.Recipient = maxapi.Recipient{ChatID: u.UserID, ChatType: "dialog", UserID: u.UserID}
	} else {
		m.Sender = &u
		m.Recipient = maxapi.Recipient{ChatID: u.UserID, ChatType: "dialog", UserID: botUser.UserID}
	}
	if len(loc) == 2 {
		m.Body.Attachments = []maxapi.Attachment{{Type: "location", Latitude: loc[0], Longitude: loc[1]}}
	}
	return m, nil
}

type errBadRequest string

func (e errBadRequest) Error() string { return string(e) }

// act — действие из чата: запись в ленту и обработка ботом в этом же запросе.
func (e *Emu) act(ctx context.Context, name string, a action) error {
	u, err := e.user(ctx, a.UserID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	var upd maxapi.Update
	switch name {
	case "start":
		if _, err := e.pool.Exec(ctx, `DELETE FROM emu_stopped WHERE user_id = $1`, u.UserID); err != nil {
			return err
		}
		label := "▶ Начать"
		if a.Payload != nil && *a.Payload != "" {
			label += " (start=" + *a.Payload + ")"
		}
		if err := e.toast(ctx, u.UserID, "system", label); err != nil {
			return err
		}
		upd = maxapi.Update{UpdateType: maxapi.UpdateBotStarted, User: &u, ChatID: u.UserID, Payload: a.Payload}
	case "text", "geo":
		mid := randomID("mid.user.")
		var loc []byte
		text := strings.TrimSpace(a.Text)
		if name == "geo" {
			loc, _ = json.Marshal([]float64{a.Lat, a.Lon})
			text = ""
		} else if text == "" {
			return errBadRequest("пустое сообщение")
		}
		if _, err := e.pool.Exec(ctx, `INSERT INTO emu_messages (mid, user_id, from_bot, text, location) VALUES ($1, $2, false, $3, $4)`,
			mid, u.UserID, text, loc); err != nil {
			return err
		}
		msg, err := e.message(ctx, mid, u)
		if err != nil {
			return err
		}
		upd = maxapi.Update{UpdateType: maxapi.UpdateMessageCreated, Message: msg}
	case "click":
		if a.Payload == nil {
			return errBadRequest("нет payload кнопки")
		}
		msg, err := e.message(ctx, a.MID, u)
		if err != nil {
			return err
		}
		id := randomID("cb.")
		if _, err := e.pool.Exec(ctx, `INSERT INTO emu_callbacks (callback_id, user_id, mid) VALUES ($1, $2, $3)`, id, u.UserID, a.MID); err != nil {
			return err
		}
		upd = maxapi.Update{UpdateType: maxapi.UpdateMessageCallback, Message: msg,
			Callback: &maxapi.Callback{Timestamp: now, CallbackID: id, Payload: *a.Payload, User: u}}
	case "stop":
		if _, err := e.pool.Exec(ctx, `INSERT INTO emu_stopped (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.UserID); err != nil {
			return err
		}
		if err := e.toast(ctx, u.UserID, "system", "■ Бот остановлен"); err != nil {
			return err
		}
		upd = maxapi.Update{UpdateType: maxapi.UpdateBotStopped, User: &u, ChatID: u.UserID}
	case "clear":
		_, err := e.pool.Exec(ctx, `DELETE FROM emu_messages WHERE user_id = $1`, u.UserID)
		return err
	default:
		return errBadRequest("неизвестное действие " + name)
	}
	upd.Timestamp = now
	upd.UserLocale = "ru"
	e.handle(ctx, upd, u.UserID)
	return nil
}

// handle — обновление в бота. Ошибку или панику бота показываем в чате
// всплывашкой: для отладки это полезнее извинения, которое отправил бы
// настоящий обработчик.
func (e *Emu) handle(ctx context.Context, u maxapi.Update, userID int64) {
	start := time.Now()
	err := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("паника: %v", p)
			}
		}()
		return e.bot.Handle(ctx, u)
	}()
	if err != nil {
		slog.Error("эмулятор: обработка обновления", "update_type", u.UpdateType, "err", err)
		_ = e.toast(ctx, userID, "error", "Ошибка бота ("+u.UpdateType+"): "+err.Error())
		return
	}
	slog.Info("эмулятор: обновление", "update_type", u.UpdateType, "ms", time.Since(start).Milliseconds())
}

// --- Состояние чата ----------------------------------------------------------

type state struct {
	Messages []Message        `json:"messages"`
	Toasts   []Toast          `json:"toasts"`
	Users    []User           `json:"users"`
	Commands []maxapi.Command `json:"commands"`
	Stopped  bool             `json:"stopped"`
	Bot      map[string]any   `json:"bot"`
	WebApp   string           `json:"web_app"`
}

func (e *Emu) state(ctx context.Context, userID int64) (state, error) {
	st := state{Messages: []Message{}, Toasts: []Toast{}, Users: []User{}, Commands: bot.Commands(),
		Bot: map[string]any{"username": e.cfg.Bot.BotName, "user_id": e.cfg.Bot.BotID}, WebApp: e.cfg.WebApp}
	rows, err := e.pool.Query(ctx, `SELECT mid, from_bot, text, keyboard, location, created_at, edited, deleted
		FROM emu_messages WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var m Message
		var kb []byte
		var ts time.Time
		if err := rows.Scan(&m.MID, &m.FromBot, &m.Text, &kb, &m.Location, &ts, &m.Edited, &m.Deleted); err != nil {
			rows.Close()
			return st, err
		}
		if kb != nil {
			_ = json.Unmarshal(kb, &m.Keyboard)
		}
		m.Time = ts.UnixMilli()
		st.Messages = append(st.Messages, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}
	rows, err = e.pool.Query(ctx, `SELECT id, kind, text FROM (SELECT * FROM emu_toasts WHERE user_id = $1 ORDER BY id DESC LIMIT 20) t ORDER BY id`, userID)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var t Toast
		if err := rows.Scan(&t.ID, &t.Kind, &t.Text); err != nil {
			rows.Close()
			return st, err
		}
		st.Toasts = append(st.Toasts, t)
	}
	rows.Close()
	if st.Users, err = e.users(ctx); err != nil {
		return st, err
	}
	err = e.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM emu_stopped WHERE user_id = $1)`, userID).Scan(&st.Stopped)
	return st, err
}

func (e *Emu) users(ctx context.Context) ([]User, error) {
	rows, err := e.pool.Query(ctx, `SELECT user_id, first_name FROM emu_users ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[User])
}

func (e *Emu) addUser(ctx context.Context, name string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return User{}, errBadRequest("имя от 1 до 64 символов")
	}
	// id из дев-диапазона: у живых пользователей MAX таких не бывает.
	u := User{FirstName: name}
	err := e.pool.QueryRow(ctx, `INSERT INTO emu_users (user_id, first_name)
		SELECT greatest(coalesce(max(user_id), 0) + 1, $1), $2 FROM emu_users WHERE user_id >= $1
		RETURNING user_id`, firstNewUser, name).Scan(&u.UserID)
	return u, err
}

// removeUser убирает пользователя из списка эмулятора вместе с лентой.
// Траектория в базе приложения остаётся: её удаляет /delete.
func (e *Emu) removeUser(ctx context.Context, id int64) error {
	for _, u := range defaultUsers {
		if u.UserID == id {
			return errBadRequest("демо-пользователей не убрать")
		}
	}
	_, err := e.pool.Exec(ctx, `WITH m AS (DELETE FROM emu_messages WHERE user_id = $1),
		t AS (DELETE FROM emu_toasts WHERE user_id = $1) DELETE FROM emu_users WHERE user_id = $1`, id)
	return err
}

// --- HTTP --------------------------------------------------------------------

// ServeHTTP — страница чата и его API. Роутинг без шаблонов ServeMux: рантайм
// Cloud Functions по умолчанию собирает функцию со старым ServeMux.
func (e *Emu) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	if path == "" || path == "/index.html" {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-cache")
		_, _ = rw.Write(indexHTML)
		return
	}
	if path == "/health" {
		_, _ = rw.Write([]byte("ok"))
		return
	}
	name, ok := strings.CutPrefix(path, "/ui/")
	if !ok {
		http.NotFound(rw, req)
		return
	}
	if e.cfg.Key != "" && subtle.ConstantTimeCompare([]byte(req.Header.Get("X-Emu-Key")), []byte(e.cfg.Key)) != 1 {
		writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "нужен ключ эмулятора"})
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 25*time.Second)
	defer cancel()
	if err := e.init(ctx); err != nil {
		writeErr(rw, err)
		return
	}
	var a action
	if req.Method == http.MethodPost {
		if err := json.NewDecoder(http.MaxBytesReader(rw, req.Body, 64<<10)).Decode(&a); err != nil {
			writeErr(rw, errBadRequest("тело запроса: "+err.Error()))
			return
		}
	} else if _, err := fmt.Sscan(req.URL.Query().Get("user_id"), &a.UserID); err != nil && name != "state" {
		writeErr(rw, errBadRequest("нужен user_id"))
		return
	}
	var err error
	switch name {
	case "state":
	case "users/add":
		var u User
		if u, err = e.addUser(ctx, a.Name); err == nil {
			a.UserID = u.UserID
		}
	case "users/remove":
		err = e.removeUser(ctx, a.UserID)
		a.UserID = defaultUsers[0].UserID
	default:
		if req.Method != http.MethodPost {
			writeErr(rw, errBadRequest("нужен POST"))
			return
		}
		err = e.act(ctx, name, a)
	}
	if err != nil {
		writeErr(rw, err)
		return
	}
	st, err := e.state(ctx, a.UserID)
	if err != nil {
		writeErr(rw, err)
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"user_id": a.UserID, "state": st})
}

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(v)
}

func writeErr(rw http.ResponseWriter, err error) {
	var bad errBadRequest
	if errors.As(err, &bad) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	slog.Error("эмулятор", "err", err)
	writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
