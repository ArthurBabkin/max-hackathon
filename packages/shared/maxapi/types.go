// Package maxapi — клиент Bot API MAX и типы обновлений.
//
// Свой, а не официальный SDK: max-bot-api-client-go/v2 требует Go 1.24, а
// самый новый Go-рантайм Yandex Cloud Functions — 1.23. Поля и эндпоинты
// сверены с https://dev.max.ru/docs-api (схема 0.0.33) и с исходниками SDK
// (Apache 2.0, атрибуция в README).
package maxapi

import "strconv"

// Типы обновлений, которые обрабатывает бот (ТЗ §11.1).
const (
	UpdateMessageCreated  = "message_created"
	UpdateMessageCallback = "message_callback"
	UpdateBotStarted      = "bot_started"
	UpdateBotStopped      = "bot_stopped"
	UpdateDialogRemoved   = "dialog_removed"
)

// SubscribedTypes — что просим у вебхука и у long polling: остальное бот
// всё равно проигнорировал бы.
var SubscribedTypes = []string{UpdateMessageCreated, UpdateMessageCallback, UpdateBotStarted,
	UpdateBotStopped, UpdateDialogRemoved}

// Update — событие от MAX. Поля разных типов лежат на верхнем уровне:
// message у message_created и message_callback, callback — у нажатия,
// user, chat_id и payload — у bot_started.
type Update struct {
	UpdateType string    `json:"update_type"`
	Timestamp  int64     `json:"timestamp"` // Unix, миллисекунды
	Message    *Message  `json:"message,omitempty"`
	Callback   *Callback `json:"callback,omitempty"`
	User       *User     `json:"user,omitempty"`
	ChatID     int64     `json:"chat_id,omitempty"`
	// Payload — параметр ссылки https://max.ru/<bot>?start=<payload>, только у bot_started.
	Payload    *string `json:"payload,omitempty"`
	UserLocale string  `json:"user_locale,omitempty"`
}

type Callback struct {
	Timestamp  int64  `json:"timestamp"`
	CallbackID string `json:"callback_id"`
	Payload    string `json:"payload"`
	User       User   `json:"user"`
}

type Message struct {
	Sender    *User       `json:"sender,omitempty"`
	Recipient Recipient   `json:"recipient"`
	Timestamp int64       `json:"timestamp"`
	Body      MessageBody `json:"body"`
}

type MessageBody struct {
	MID         string       `json:"mid"`
	Seq         int64        `json:"seq"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment — входящее вложение. Координаты от кнопки request_geo_location
// лежат на верхнем уровне вложения, а не в payload.
type Attachment struct {
	Type      string  `json:"type"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type Recipient struct {
	ChatID   int64  `json:"chat_id"`
	ChatType string `json:"chat_type"` // dialog | chat | channel
	UserID   int64  `json:"user_id,omitempty"`
}

type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot,omitempty"`
}

// From — кто вызвал событие.
func (u Update) From() (User, bool) {
	switch {
	case u.Callback != nil:
		return u.Callback.User, true
	case u.Message != nil && u.Message.Sender != nil:
		return *u.Message.Sender, true
	case u.User != nil:
		return *u.User, true
	}
	return User{}, false
}

// Location — координаты из сообщения, если пользователь поделился геопозицией.
func (m *Message) Location() (lat, lon float64, ok bool) {
	if m == nil {
		return 0, 0, false
	}
	for _, a := range m.Body.Attachments {
		if a.Type == "location" {
			return a.Latitude, a.Longitude, true
		}
	}
	return 0, 0, false
}

// DedupKey — ключ повторной доставки. MAX пересылает обновление, если не
// получил 200 за 30 секунд, и бот не должен второй раз расходовать
// приглашение или ставить «напомнить завтра».
func (u Update) DedupKey() string {
	switch {
	case u.Callback != nil && u.Callback.CallbackID != "":
		return "cb:" + u.Callback.CallbackID
	case u.Message != nil && u.Message.Body.MID != "":
		return "msg:" + u.Message.Body.MID
	}
	from, _ := u.From()
	return u.UpdateType + ":" + strconv.FormatInt(from.UserID, 10) + ":" + strconv.FormatInt(u.Timestamp, 10)
}

// UpdateList — ответ GET /updates.
type UpdateList struct {
	Updates []Update `json:"updates"`
	Marker  *int64   `json:"marker"`
}

// Subscription — строка ответа GET /subscriptions.
type Subscription struct {
	URL         string   `json:"url"`
	Time        int64    `json:"time"`
	UpdateTypes []string `json:"update_types"`
}

// BotInfo — ответ GET /me.
type BotInfo struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	IsBot     bool   `json:"is_bot"`
}

// Command — подсказка в меню бота (PATCH /me/commands).
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
