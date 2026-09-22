package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/llm"
)

type replyLLM struct{ reply string }

func (f replyLLM) JSON(context.Context, []llm.Message) (string, error) { return f.reply, nil }

func TestAI_AskAndPersonalHistory(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	e.srv.assistant.LLM = replyLLM{`{"answer": "Да, «Высшая проба» по информатике даёт БВИ в ВШЭ.", "card_ids": ["olympiad:p669-8-informatika"], "no_data": false}`}

	r := e.do("POST", "/api/v1/ai/messages", kid, map[string]any{"text": "  Что даёт «Высшая проба» в ВШЭ?  "})
	if r.code != 200 {
		t.Fatalf("вопрос: %d %s", r.code, r.raw)
	}
	q, a := r.body["question"].(map[string]any), r.body["answer"].(map[string]any)
	if q["role"] != "user" || q["text"] != "Что даёт «Высшая проба» в ВШЭ?" || a["role"] != "assistant" || a["refused"] != false {
		t.Fatalf("реплики: %s", r.raw)
	}
	refs := list(t, a["card_refs"])
	if len(refs) != 1 || refs[0]["type"] != "olympiad" || refs[0]["id"] != "p669-8-informatika" {
		t.Fatalf("кнопка карточки: %v", refs)
	}
	for _, src := range list(t, a["sources"]) {
		if src["url"] == "" || src["kind"] == "" {
			t.Fatalf("источник по контракту: %v", src)
		}
	}

	h := e.do("GET", "/api/v1/ai/messages", kid, nil)
	if items := list(t, h.body["items"]); h.code != 200 || len(items) != 2 || items[0]["role"] != "user" {
		t.Fatalf("история ученика: %d %s", h.code, h.raw)
	}
	if items := list(t, e.do("GET", "/api/v1/ai/messages", parent, nil).body["items"]); len(items) != 0 {
		t.Fatalf("чужой чат не виден (F37): %v", items)
	}
}

func TestAI_RefusalWithoutModel(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	r := e.do("POST", "/api/v1/ai/messages", kid, map[string]any{"text": "Какая завтра погода?"})
	a := r.body["answer"].(map[string]any)
	if r.code != 200 || a["refused"] != true || !strings.HasPrefix(a["text"].(string), "Данных нет") ||
		len(list(t, a["sources"])) == 0 || len(list(t, a["card_refs"])) != 0 {
		t.Fatalf("отказ со ссылкой на первоисточник: %d %s", r.code, r.raw)
	}
}

func TestAI_Validation(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	for _, text := range []string{"", "   ", strings.Repeat("я", 501)} {
		if r := e.do("POST", "/api/v1/ai/messages", kid, map[string]any{"text": text}); r.code != 400 || r.errCode() != "BAD_REQUEST" {
			t.Fatalf("%d символов — 400: %d %s", len([]rune(text)), r.code, r.raw)
		}
	}
	if r := e.do("POST", "/api/v1/ai/messages", kid, map[string]any{"text": strings.Repeat("я", 500)}); r.code != 200 {
		t.Fatalf("ровно 500 символов — можно: %d", r.code)
	}
	if r := e.do("GET", "/api/v1/ai/messages", "", nil); r.code != 401 {
		t.Fatalf("без токена: %d", r.code)
	}
}

func TestAI_RateLimit(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	ask := func() resp {
		return e.do("POST", "/api/v1/ai/messages", kid, map[string]any{"text": "что такое БВИ?"})
	}
	for i := 0; i < 5; i++ {
		if r := ask(); r.code != 200 {
			t.Fatalf("вопрос %d: %d", i+1, r.code)
		}
	}
	if r := ask(); r.code != 429 || r.errCode() != "RATE_LIMITED" {
		t.Fatalf("шестой подряд — 429: %d %s", r.code, r.raw)
	}
	e.now = e.now.Add(13 * time.Second)
	if r := ask(); r.code != 200 {
		t.Fatalf("через 12 секунд — снова можно: %d", r.code)
	}
}
