package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/llm"
)

type replyLLM struct{ reply string }

func (f replyLLM) JSON(context.Context, []llm.Message) (string, error) { return f.reply, nil }

// recordLLM запоминает, что ушло в модель: так видно контекст разговора.
type recordLLM struct {
	reply string
	calls [][]llm.Message
}

func (f *recordLLM) JSON(_ context.Context, m []llm.Message) (string, error) {
	f.calls = append(f.calls, m)
	return f.reply, nil
}

const hsePrizeReply = `{"answer": "Да, «Высшая проба» по информатике даёт БВИ в ВШЭ.", "card_ids": ["olympiad:p669-8-informatika"], "no_data": false}`

// startChat — новый чат с первым вопросом; возвращает id чата.
func (e *env) startChat(token, question string) string {
	e.t.Helper()
	r := e.do("POST", "/api/v1/ai/chats", token, map[string]any{"text": question})
	if r.code != 201 {
		e.t.Fatalf("новый чат: %d %s", r.code, r.raw)
	}
	return r.body["chat"].(map[string]any)["id"].(string)
}

// F58, F59: чат создаётся первым вопросом и называется по дню в поясе
// траектории — 21:30 UTC 22 сентября в Москве уже 23 сентября.
func TestAI_NewChatNamedByDayInTrajectoryZone(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	e.now = time.Date(2026, 9, 22, 21, 30, 0, 0, time.UTC)
	kid := e.login(900000001, "Артём")
	e.srv.assistant.LLM = replyLLM{hsePrizeReply}

	r := e.do("POST", "/api/v1/ai/chats", kid, map[string]any{"text": "  Что даёт «Высшая проба» в ВШЭ?  "})
	if r.code != 201 {
		t.Fatalf("новый чат: %d %s", r.code, r.raw)
	}
	chat := r.body["chat"].(map[string]any)
	if chat["title"] != "Чат 23 сентября" || chat["id"] == "" {
		t.Fatalf("чат: %s", r.raw)
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
	h := e.do("GET", fmt.Sprintf("/api/v1/ai/chats/%s/messages", chat["id"]), kid, nil)
	if items := list(t, h.body["items"]); h.code != 200 || len(items) != 2 || items[0]["role"] != "user" {
		t.Fatalf("история чата: %d %s", h.code, h.raw)
	}
}

// F37: чаты личные. Чужой чат для участника не существует — 404, а не 403.
func TestAI_ChatsArePersonal(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	e.srv.assistant.LLM = replyLLM{hsePrizeReply}
	id := e.startChat(kid, "Что даёт «Высшая проба»?")

	if items := list(t, e.do("GET", "/api/v1/ai/chats", kid, nil).body["items"]); len(items) != 1 || items[0]["id"] != id {
		t.Fatalf("чаты ученика: %v", items)
	}
	if items := list(t, e.do("GET", "/api/v1/ai/chats", parent, nil).body["items"]); len(items) != 0 {
		t.Fatalf("у родителя своих чатов нет: %v", items)
	}
	for _, r := range []resp{
		e.do("GET", "/api/v1/ai/chats/"+id+"/messages", parent, nil),
		e.do("POST", "/api/v1/ai/chats/"+id+"/messages", parent, map[string]any{"text": "А мне?"}),
		e.do("PATCH", "/api/v1/ai/chats/"+id, parent, map[string]any{"title": "Моё"}),
	} {
		if r.code != 404 || r.errCode() != "NOT_FOUND" {
			t.Fatalf("чужой чат — 404: %d %s", r.code, r.raw)
		}
	}
	for _, path := range []string{"/api/v1/ai/chats/not-a-uuid/messages", "/api/v1/ai/chats/00000000-0000-4000-8000-000000000000/messages"} {
		if r := e.do("GET", path, kid, nil); r.code != 404 {
			t.Fatalf("%s — 404: %d %s", path, r.code, r.raw)
		}
	}
}

// F60: модель видит 10 предыдущих реплик своего чата — пять последних пар,
// и ничего из другого чата. Чат с новой репликой поднимается в списке.
func TestAI_ModelSeesLastTenMessagesOfThisChat(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	model := &recordLLM{reply: hsePrizeReply}
	e.srv.assistant.LLM = model
	ask := func(id, text string) resp {
		t.Helper()
		e.now = e.now.Add(13 * time.Second) // не упираться в лимит частоты
		r := e.do("POST", "/api/v1/ai/chats/"+id+"/messages", kid, map[string]any{"text": text})
		if r.code != 200 {
			t.Fatalf("вопрос %q: %d %s", text, r.code, r.raw)
		}
		return r
	}

	a := e.startChat(kid, "«Высшая проба» 1")
	for i := 2; i <= 6; i++ {
		ask(a, fmt.Sprintf("«Высшая проба» %d", i))
	}
	e.now = e.now.Add(13 * time.Second)
	b := e.startChat(kid, "Чем БВИ отличается от 100 баллов?")
	r := ask(a, "«Высшая проба» 7")

	sent := model.calls[len(model.calls)-1]
	if len(sent) != 12 {
		t.Fatalf("system + 10 реплик + вопрос, а ушло %d сообщений", len(sent))
	}
	if sent[1].Role != "user" || sent[1].Content != "«Высшая проба» 2" || sent[10].Role != "assistant" ||
		sent[11].Content != "«Высшая проба» 7" {
		t.Fatalf("история — пары 2–6 от старой к новой: %+v", sent[1:])
	}
	for _, m := range sent[1:] {
		if strings.Contains(m.Content, "БВИ отличается") {
			t.Fatal("в контекст попала реплика другого чата")
		}
	}
	if r.body["chat"].(map[string]any)["id"] != a {
		t.Fatalf("ответ несёт свой чат: %s", r.raw)
	}
	items := list(t, e.do("GET", "/api/v1/ai/chats", kid, nil).body["items"])
	if len(items) != 2 || items[0]["id"] != a || items[1]["id"] != b {
		t.Fatalf("чат с новой репликой — первым: %v", items)
	}
}

// F59: своё название — от 1 до 60 символов, пробелы по краям не считаются.
func TestAI_RenameChat(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	id := e.startChat(kid, "Какая завтра погода?")

	r := e.do("PATCH", "/api/v1/ai/chats/"+id, kid, map[string]any{"title": "  Высшая проба  "})
	if r.code != 200 || r.body["title"] != "Высшая проба" || r.body["id"] != id {
		t.Fatalf("переименование: %d %s", r.code, r.raw)
	}
	if items := list(t, e.do("GET", "/api/v1/ai/chats", kid, nil).body["items"]); items[0]["title"] != "Высшая проба" {
		t.Fatalf("новое название в списке: %v", items)
	}
	for _, title := range []string{"", "   ", strings.Repeat("я", 61)} {
		if r := e.do("PATCH", "/api/v1/ai/chats/"+id, kid, map[string]any{"title": title}); r.code != 400 || r.errCode() != "BAD_REQUEST" {
			t.Fatalf("%d символов — 400: %d %s", len([]rune(title)), r.code, r.raw)
		}
	}
	if r := e.do("PATCH", "/api/v1/ai/chats/"+id, kid, map[string]any{"title": strings.Repeat("я", 60)}); r.code != 200 {
		t.Fatalf("ровно 60 символов — можно: %d %s", r.code, r.raw)
	}
	if r := e.do("PATCH", "/api/v1/ai/chats/not-a-uuid", kid, map[string]any{"title": "x"}); r.code != 404 {
		t.Fatalf("не uuid — 404: %d", r.code)
	}
}

func TestAI_RefusalWithoutModel(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	r := e.do("POST", "/api/v1/ai/chats", kid, map[string]any{"text": "Какая завтра погода?"})
	if r.code != 201 {
		t.Fatalf("новый чат: %d %s", r.code, r.raw)
	}
	a := r.body["answer"].(map[string]any)
	if a["refused"] != true || !strings.HasPrefix(a["text"].(string), "Данных нет") ||
		len(list(t, a["sources"])) == 0 || len(list(t, a["card_refs"])) != 0 {
		t.Fatalf("отказ со ссылкой на первоисточник: %d %s", r.code, r.raw)
	}
}

func TestAI_Validation(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	id := e.startChat(kid, "Какая завтра погода?")
	for _, text := range []string{"", "   ", strings.Repeat("я", 501)} {
		if r := e.do("POST", "/api/v1/ai/chats", kid, map[string]any{"text": text}); r.code != 400 || r.errCode() != "BAD_REQUEST" {
			t.Fatalf("новый чат, %d символов — 400: %d %s", len([]rune(text)), r.code, r.raw)
		}
		if r := e.do("POST", "/api/v1/ai/chats/"+id+"/messages", kid, map[string]any{"text": text}); r.code != 400 {
			t.Fatalf("вопрос в чат, %d символов — 400: %d %s", len([]rune(text)), r.code, r.raw)
		}
	}
	if r := e.do("POST", "/api/v1/ai/chats/"+id+"/messages", kid, map[string]any{"text": strings.Repeat("я", 500)}); r.code != 200 {
		t.Fatalf("ровно 500 символов — можно: %d", r.code)
	}
	if items := list(t, e.do("GET", "/api/v1/ai/chats", kid, nil).body["items"]); len(items) != 1 {
		t.Fatalf("отклонённые вопросы не создают чатов: %v", items)
	}
	if r := e.do("GET", "/api/v1/ai/chats", "", nil); r.code != 401 {
		t.Fatalf("без токена: %d", r.code)
	}
}

func TestAI_RateLimit(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	id := e.startChat(kid, "что такое БВИ?")
	ask := func() resp {
		return e.do("POST", "/api/v1/ai/chats/"+id+"/messages", kid, map[string]any{"text": "что такое БВИ?"})
	}
	for i := 2; i <= 5; i++ {
		if r := ask(); r.code != 200 {
			t.Fatalf("вопрос %d: %d", i, r.code)
		}
	}
	if r := e.do("POST", "/api/v1/ai/chats", kid, map[string]any{"text": "что такое БВИ?"}); r.code != 429 || r.errCode() != "RATE_LIMITED" {
		t.Fatalf("шестой подряд, хоть и в новый чат, — 429: %d %s", r.code, r.raw)
	}
	e.now = e.now.Add(13 * time.Second)
	if r := ask(); r.code != 200 {
		t.Fatalf("через 12 секунд — снова можно: %d", r.code)
	}
}
