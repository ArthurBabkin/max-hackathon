// Package maxapi — клиент Bot API MAX и типы обновлений.
// Документация: https://dev.max.ru/docs-api
package maxapi

// Update — событие, которое MAX присылает на вебхук.
// Полный список типов описан в разделе Update документации.
type Update struct {
	UpdateType string   `json:"update_type"`
	Timestamp  int64    `json:"timestamp"`
	Message    *Message `json:"message,omitempty"`
	User       *User    `json:"user,omitempty"`
	ChatID     int64    `json:"chat_id,omitempty"`
	CallbackID string   `json:"callback_id,omitempty"`
}

type Message struct {
	Sender    *User      `json:"sender,omitempty"`
	Recipient *Recipient `json:"recipient,omitempty"`
	Body      *Body      `json:"body,omitempty"`
}

type Body struct {
	MID  string `json:"mid,omitempty"`
	Text string `json:"text,omitempty"`
}

type Recipient struct {
	ChatID int64 `json:"chat_id,omitempty"`
	UserID int64 `json:"user_id,omitempty"`
}

type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot,omitempty"`
}

// Типы обновлений, которые нам нужны по ТЗ.
const (
	UpdateMessageCreated  = "message_created"
	UpdateMessageCallback = "message_callback"
	UpdateBotStarted      = "bot_started"
	UpdateBotAdded        = "bot_added"
	UpdateBotRemoved      = "bot_removed"
)
