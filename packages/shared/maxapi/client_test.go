package maxapi

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recorded struct {
	method, path, query, auth string
	body                      map[string]any
}

func fakeMAX(t *testing.T, handle func(w http.ResponseWriter, r recorded)) (*Client, *[]recorded) {
	t.Helper()
	var calls []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")}
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rec.body); err != nil {
				t.Errorf("тело не JSON: %s", raw)
			}
		}
		calls = append(calls, rec)
		handle(w, rec)
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "secret-token")
	c.pause = func(int) time.Duration { return time.Millisecond }
	c.limit.every = 0
	return c, &calls
}

func TestSend_TokenInHeaderAndKeyboardShape(t *testing.T) {
	c, calls := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = w.Write([]byte(`{"message":{"body":{"mid":"mid.1","text":"x"},"recipient":{"chat_id":1,"chat_type":"dialog"}}}`))
	})
	kb := Keyboard{Row(CallbackButton("Я школьник", "role:kid"), CallbackButton("Я родитель", "role:parent")),
		Row(OpenAppButton("Открыть навигатор", "t356_hakaton_max_bot", 426643746, "home"))}
	mid, err := c.Send(context.Background(), 900000001, WithKeyboard("Кто ты?", kb))
	if err != nil || mid != "mid.1" {
		t.Fatalf("%q %v", mid, err)
	}
	got := (*calls)[0]
	if got.auth != "secret-token" {
		t.Fatalf("токен — в Authorization без префикса: %q", got.auth)
	}
	if got.method != "POST" || got.path != "/messages" || got.query != "user_id=900000001" {
		t.Fatalf("%+v", got)
	}
	if strings.Contains(got.query, "token") {
		t.Fatal("токен в query запрещён")
	}
	att := got.body["attachments"].([]any)[0].(map[string]any)
	rows := att["payload"].(map[string]any)["buttons"].([]any)
	first := rows[0].([]any)[0].(map[string]any)
	app := rows[1].([]any)[0].(map[string]any)
	if att["type"] != "inline_keyboard" || first["type"] != "callback" || first["payload"] != "role:kid" ||
		app["type"] != "open_app" || app["contact_id"] != float64(426643746) || app["web_app"] != "t356_hakaton_max_bot" {
		t.Fatalf("клавиатура: %v", got.body)
	}
}

func TestEdit_EmptyAttachmentsRemoveKeyboard(t *testing.T) {
	c, calls := fakeMAX(t, func(w http.ResponseWriter, _ recorded) { _, _ = w.Write([]byte(`{"success":true}`)) })
	if err := c.Edit(context.Background(), "mid.1", Text("Готово")); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	if got.method != "PUT" || got.query != "message_id=mid.1" {
		t.Fatalf("%+v", got)
	}
	if a, ok := got.body["attachments"].([]any); !ok || len(a) != 0 {
		t.Fatalf("пустой массив attachments убирает клавиатуру: %v", got.body)
	}
}

func TestAnswer_SuccessFalseIsError(t *testing.T) {
	c, calls := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = w.Write([]byte(`{"success":false,"message":"callback expired"}`))
	})
	err := c.Answer(context.Background(), "cb-1", CallbackAnswer{Notification: "Сохранено"})
	var e *Error
	if !errors.As(err, &e) || e.Message != "callback expired" {
		t.Fatalf("success:false — ошибка: %v", err)
	}
	if got := (*calls)[0]; got.path != "/answers" || got.query != "callback_id=cb-1" || got.body["notification"] != "Сохранено" {
		t.Fatalf("%+v", got)
	}
}

func TestRetryOn429Only(t *testing.T) {
	var n atomic.Int32
	c, _ := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		if n.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"message":{"body":{"mid":"mid.2"}}}`))
	})
	if mid, err := c.Send(context.Background(), 1, Text("x")); err != nil || mid != "mid.2" || n.Load() != 3 {
		t.Fatalf("после двух 429 — успех с третьей попытки: %q %v %d", mid, err, n.Load())
	}

	var m atomic.Int32
	c2, _ := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		m.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"internal","message":"oops"}`))
	})
	_, err := c2.Send(context.Background(), 1, Text("x"))
	var e *Error
	if !errors.As(err, &e) || e.Status != 500 || e.Code != "internal" || m.Load() != 1 {
		t.Fatalf("5xx не повторяем — сообщение могло уйти: %v, попыток %d", err, m.Load())
	}
}

func TestIsBlocked(t *testing.T) {
	c, _ := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"chat.denied","message":"bot is blocked"}`))
	})
	_, err := c.Send(context.Background(), 1, Text("x"))
	if !IsBlocked(err) {
		t.Fatalf("403 — пользователь остановил бота: %v", err)
	}
}

func TestUpdates_MarkerAndTypes(t *testing.T) {
	c, calls := fakeMAX(t, func(w http.ResponseWriter, _ recorded) {
		_, _ = w.Write([]byte(`{"updates":[{"update_type":"bot_started","timestamp":1,"chat_id":5,
			"user":{"user_id":7,"first_name":"Артём"},"payload":"inv_abc"}],"marker":42}`))
	})
	marker := int64(41)
	list, err := c.Updates(context.Background(), &marker, 30*time.Second, []string{"bot_started", "message_callback"})
	if err != nil || len(list.Updates) != 1 || list.Marker == nil || *list.Marker != 42 {
		t.Fatalf("%+v %v", list, err)
	}
	if q := (*calls)[0].query; !strings.Contains(q, "marker=41") || !strings.Contains(q, "timeout=30") ||
		!strings.Contains(q, "types=bot_started%2Cmessage_callback") {
		t.Fatalf("query: %s", q)
	}
	u := list.Updates[0]
	if u.Payload == nil || *u.Payload != "inv_abc" {
		t.Fatalf("payload ссылки: %+v", u)
	}
	if from, ok := u.From(); !ok || from.UserID != 7 {
		t.Fatalf("from: %+v", from)
	}
}

// Формы обновлений — как в документации и в заглушках SDK.
func TestUpdate_ShapesFromDocs(t *testing.T) {
	var cb Update
	_ = json.Unmarshal([]byte(`{"update_type":"message_callback","timestamp":1775025604499,
		"callback":{"timestamp":1,"callback_id":"cb-xyz","payload":"grade:9","user":{"user_id":123,"first_name":"John"}},
		"message":{"recipient":{"chat_id":182,"chat_type":"dialog","user_id":123},"timestamp":1,
		           "body":{"mid":"mid.abc","seq":1,"text":"В каком ты классе?"},
		           "sender":{"user_id":229,"first_name":"Bot","is_bot":true}}}`), &cb)
	if from, _ := cb.From(); from.UserID != 123 {
		t.Fatalf("у нажатия автор — пользователь, а не бот-отправитель сообщения: %+v", from)
	}
	if cb.DedupKey() != "cb:cb-xyz" {
		t.Fatalf("ключ нажатия: %s", cb.DedupKey())
	}

	var msg Update
	_ = json.Unmarshal([]byte(`{"update_type":"message_created","timestamp":2,
		"message":{"recipient":{"chat_id":182,"chat_type":"dialog"},"timestamp":2,
		           "body":{"mid":"mid.geo","seq":2,"text":"","attachments":[{"type":"location","latitude":55.79,"longitude":49.11}]},
		           "sender":{"user_id":123,"first_name":"John"}}}`), &msg)
	lat, lon, ok := msg.Message.Location()
	if !ok || lat != 55.79 || lon != 49.11 || msg.DedupKey() != "msg:mid.geo" {
		t.Fatalf("геопозиция: %v %v %v %s", lat, lon, ok, msg.DedupKey())
	}

	var start Update
	_ = json.Unmarshal([]byte(`{"update_type":"bot_started","timestamp":1775025604499,"chat_id":182,
		"user":{"user_id":123,"first_name":"John"},"user_locale":"ru","user_id":123}`), &start)
	if start.Payload != nil || start.DedupKey() != "bot_started:123:1775025604499" {
		t.Fatalf("старт без параметра: %+v %s", start, start.DedupKey())
	}
}

func TestRootPoolTrustsMinTsifryRoot(t *testing.T) {
	block, _ := pem.Decode(russianTrustedRootCA)
	if block == nil {
		t.Fatal("PEM не разобран")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(cert.Raw)
	const want = "d26d2d0231b7c39f92cc738512ba54103519e4405d68b5bd703e9788ca8ecf31"
	if hex.EncodeToString(sum[:]) != want || cert.Subject.CommonName != "Russian Trusted Root CA" {
		t.Fatalf("не тот корень: %s %s", cert.Subject, hex.EncodeToString(sum[:]))
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: rootPool()}); err != nil {
		t.Fatalf("корень Минцифры в пуле: %v", err)
	}
}
