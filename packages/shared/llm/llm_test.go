package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON_SendsModelAndJSONModeAndReadsContent(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/completions" {
			t.Errorf("путь: %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"answer\":\"да\"}"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL+"/api/v1/", "key", "GigaChat/GigaChat-3-Pro")
	out, err := c.JSON(context.Background(), []Message{{Role: "system", Content: "s"}, {Role: "user", Content: "q"}})
	if err != nil || out != `{"answer":"да"}` {
		t.Fatalf("ответ: %q %v", out, err)
	}
	if auth != "Bearer key" || got["model"] != "GigaChat/GigaChat-3-Pro" ||
		got["response_format"].(map[string]any)["type"] != "json_object" || len(got["messages"].([]any)) != 2 {
		t.Fatalf("запрос: %s %v", auth, got)
	}
}

func TestJSON_Errors(t *testing.T) {
	for name, body := range map[string]string{"5xx": "", "empty": `{"choices":[]}`, "blank": `{"choices":[{"message":{"content":"  "}}]}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if body == "" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(body))
		}))
		if _, err := New(srv.URL, "k", "m").JSON(context.Background(), nil); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
		srv.Close()
	}
	if New("x", "", "m").Enabled() {
		t.Fatal("без ключа модель выключена")
	}
}
