// Package maxtest — поддельный отправитель для тестов бота, воркера и API:
// записывает сообщения вместо сети.
package maxtest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

type Sent struct {
	UserID int64
	MID    string
	Msg    maxapi.NewMessage
}

type Edited struct {
	MID string
	Msg maxapi.NewMessage
}

type Answered struct {
	CallbackID string
	Answer     maxapi.CallbackAnswer
}

type Fake struct {
	mu       sync.Mutex
	Sent     []Sent
	Edited   []Edited
	Answered []Answered
	// Fail — если задан, Send возвращает его вместо отправки.
	Fail func(userID int64) error
	n    int
}

var _ maxapi.Sender = (*Fake)(nil)

func (f *Fake) Send(_ context.Context, userID int64, m maxapi.NewMessage) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		if err := f.Fail(userID); err != nil {
			return "", err
		}
	}
	f.n++
	mid := fmt.Sprintf("mid.%d", f.n)
	f.Sent = append(f.Sent, Sent{UserID: userID, MID: mid, Msg: m})
	return mid, nil
}

func (f *Fake) Edit(_ context.Context, mid string, m maxapi.NewMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Edited = append(f.Edited, Edited{MID: mid, Msg: m})
	return nil
}

func (f *Fake) Answer(_ context.Context, callbackID string, a maxapi.CallbackAnswer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Answered = append(f.Answered, Answered{CallbackID: callbackID, Answer: a})
	return nil
}

// To — сообщения одному пользователю.
func (f *Fake) To(userID int64) []Sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Sent
	for _, s := range f.Sent {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out
}

// Last — последнее сообщение пользователю; пустое, если писем не было.
func (f *Fake) Last(userID int64) maxapi.NewMessage {
	s := f.To(userID)
	if len(s) == 0 {
		return maxapi.NewMessage{}
	}
	return s[len(s)-1].Msg
}

// Reset забывает всё записанное.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent, f.Edited, f.Answered = nil, nil, nil
}

// Payloads — payload всех callback-кнопок сообщения по порядку.
func Payloads(m maxapi.NewMessage) []string {
	var out []string
	for _, row := range m.Keyboard() {
		for _, b := range row {
			if b.Payload != "" || b.Type == "callback" {
				out = append(out, b.Payload)
			}
		}
	}
	return out
}

// Buttons — подписи всех кнопок через « | », для сообщений об ошибке.
func Buttons(m maxapi.NewMessage) string {
	var out []string
	for _, row := range m.Keyboard() {
		for _, b := range row {
			out = append(out, b.Text)
		}
	}
	return strings.Join(out, " | ")
}
