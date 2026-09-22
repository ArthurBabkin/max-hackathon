package bot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(h http.Handler, secret, body string) int {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if secret != "" {
		req.Header.Set("X-Max-Bot-Api-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestHandler_WrongSecretIs403(t *testing.T) {
	h := NewHandler("s3cret", &Bot{})
	if code := post(h, "", `{}`); code != http.StatusForbidden {
		t.Fatalf("без секрета: %d", code)
	}
	if code := post(h, "wrong", `{}`); code != http.StatusForbidden {
		t.Fatalf("чужой секрет: %d", code)
	}
}

// Битое тело — наша проблема, не MAX: повтор не поможет, а не-200 копится
// в счётчик неудач, после которого MAX отписывает бота.
func TestHandler_BrokenBodyStill200(t *testing.T) {
	h := NewHandler("s3cret", &Bot{})
	if code := post(h, "s3cret", `{"update_type":`); code != http.StatusOK {
		t.Fatalf("битое тело: %d", code)
	}
}

func TestHandler_FailingUpdateStill200AndApologizes(t *testing.T) {
	hs := newHarness(t)
	hs.bot.store = nil // любой доступ к базе паникует
	h := NewHandler("s3cret", hs.bot)
	body := `{"update_type":"message_created","timestamp":1,"message":{"sender":{"user_id":900000001,"first_name":"Артём"},"recipient":{"chat_type":"dialog"},"body":{"mid":"m1","text":"/menu"}}}`
	if code := post(h, "s3cret", body); code != http.StatusOK {
		t.Fatalf("сбой обработки: %d", code)
	}
	if got := hs.lastText(artem); !strings.HasPrefix(got, "Что-то пошло не так.") {
		t.Fatalf("пользователь без ответа: %q", got)
	}
}
