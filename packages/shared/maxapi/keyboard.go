package maxapi

// Лимиты клавиатуры (dev.max.ru, «Клавиатура»): до 7 кнопок в ряду, до 3 —
// если в ряду есть link, open_app, request_geo_location или request_contact;
// до 30 рядов и 210 кнопок; payload callback-кнопки — до 1024 байт.
const (
	MaxRowButtons     = 7
	MaxRowWideButtons = 3
	MaxRows           = 30
	MaxTextLen        = 4000
)

// Button — кнопка inline-клавиатуры. Заполняются только поля её типа.
type Button struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Payload   string `json:"payload,omitempty"`
	URL       string `json:"url,omitempty"`
	WebApp    string `json:"web_app,omitempty"`
	ContactID int64  `json:"contact_id,omitempty"`
	Quick     bool   `json:"quick,omitempty"`
}

func CallbackButton(text, payload string) Button {
	return Button{Type: "callback", Text: text, Payload: payload}
}

func LinkButton(text, url string) Button { return Button{Type: "link", Text: text, URL: url} }

// OpenAppButton открывает мини-приложение бота поверх чата (F11). web_app — имя
// бота, contact_id — его id; payload попадает в start_param initData.
func OpenAppButton(text, botName string, botID int64, payload string) Button {
	return Button{Type: "open_app", Text: text, WebApp: botName, ContactID: botID, Payload: payload}
}

// GeoButton просит геопозицию (F6); ответ придёт сообщением с вложением location.
func GeoButton(text string) Button { return Button{Type: "request_geo_location", Text: text} }

// Keyboard — ряды кнопок.
type Keyboard [][]Button

// Row — ряд из кнопок; удобство для литералов.
func Row(b ...Button) []Button { return b }

// OutAttachment — исходящее вложение. Боту нужна только клавиатура.
type OutAttachment struct {
	Type    string          `json:"type"`
	Payload keyboardPayload `json:"payload"`
}

type keyboardPayload struct {
	Buttons Keyboard `json:"buttons"`
}

// NewMessage — тело POST /messages, PUT /messages и ответа на нажатие.
// Attachments без omitempty намеренно: при редактировании nil (null)
// оставляет вложения как были, пустой срез убирает клавиатуру.
type NewMessage struct {
	Text        string          `json:"text"`
	Attachments []OutAttachment `json:"attachments"`
	Format      string          `json:"format,omitempty"` // markdown | html
}

// Text — сообщение без клавиатуры.
func Text(text string) NewMessage { return NewMessage{Text: text, Attachments: []OutAttachment{}} }

// WithKeyboard — сообщение с клавиатурой; пустая клавиатура — без неё.
func WithKeyboard(text string, kb Keyboard) NewMessage {
	m := Text(text)
	if len(kb) > 0 {
		m.Attachments = []OutAttachment{{Type: "inline_keyboard", Payload: keyboardPayload{Buttons: kb}}}
	}
	return m
}

// Keyboard — клавиатура сообщения, если есть.
func (m NewMessage) Keyboard() Keyboard {
	for _, a := range m.Attachments {
		if a.Type == "inline_keyboard" {
			return a.Payload.Buttons
		}
	}
	return nil
}

// CallbackAnswer — тело POST /answers: новое сообщение вместо того, на
// котором нажали кнопку, и/или всплывающее уведомление.
type CallbackAnswer struct {
	Message      *NewMessage `json:"message,omitempty"`
	Notification string      `json:"notification,omitempty"`
}
