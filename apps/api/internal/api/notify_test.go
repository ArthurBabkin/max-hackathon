package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

const (
	artemMax = 900000001
	olgaMax  = 900000002
)

// Отметка регистрации из мини-приложения приходит остальным в чат (F46);
// снятие отметки и повторная отметка — молча.
func TestNotify_RegisteredGoesToOthersOnce(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	parent := e.login(olgaMax, "Ольга")
	id, _, err := e.st.AddTrackerItem(context.Background(), f.trajectoryID, "p669-8-informatika", f.kid.MemberID)
	if err != nil {
		t.Fatal(err)
	}
	e.do("PUT", "/api/v1/tracker/"+id+"/registered", parent, nil)
	e.do("PUT", "/api/v1/tracker/"+id+"/registered", parent, nil)
	e.do("DELETE", "/api/v1/tracker/"+id+"/registered", parent, nil)

	got := e.fake.To(artemMax)
	if len(got) != 1 || got[0].Msg.Text != "✅ Ольга отмечает: регистрация на «Высшая проба» пройдена." {
		t.Fatalf("ученику одно уведомление: %v", got)
	}
	if kb := got[0].Msg.Keyboard(); kb[0][0].Type != "open_app" || kb[0][0].ContactID != 42 || kb[0][0].Payload != "tracker" {
		t.Fatalf("кнопка в трекер: %s", maxtest.Buttons(got[0].Msg))
	}
	if n := len(e.fake.To(olgaMax)); n != 0 {
		t.Fatalf("автору отметки не пишем: %d", n)
	}
}

// Предложение родителя — ученику с кнопками ответа; ответ ученика —
// автору предложения (F45).
func TestNotify_ProposalRoundTrip(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	parent := e.login(olgaMax, "Ольга")

	id := e.do("POST", "/api/v1/proposals", parent, map[string]any{"olympiad_profile_id": "p669-8-informatika"}).body["id"].(string)
	e.do("POST", "/api/v1/proposals", parent, map[string]any{"olympiad_profile_id": "p669-8-informatika"})
	got := e.fake.To(artemMax)
	if len(got) != 1 || !strings.HasPrefix(got[0].Msg.Text, "📩 Ольга предлагает добавить в трекер «Высшая проба».") {
		t.Fatalf("ученику одно предложение: %v", got)
	}
	// Карточка открывается по профилю: GET /olympiads/{id} ждёт olympiad_profile_id.
	if b := maxtest.Payloads(got[0].Msg); len(b) != 3 || b[0] != "prop:acc:"+id || b[1] != "o_p669-8-informatika" || b[2] != "prop:dec:"+id {
		t.Fatalf("кнопки ответа: %v", b)
	}

	e.do("POST", "/api/v1/proposals/"+id+"/accept", kid, nil)
	if last := e.fake.Last(olgaMax); !strings.HasPrefix(last.Text, "✅ Артём добавляет «Высшая проба» в трекер.") {
		t.Fatalf("родителю — ответ ученика: %q", last.Text)
	}

	id2 := e.do("POST", "/api/v1/proposals", parent, map[string]any{"olympiad_profile_id": "vsosh-informatika"}).body["id"].(string)
	e.do("POST", "/api/v1/proposals/"+id2+"/decline", kid, nil)
	if last := e.fake.Last(olgaMax); !strings.HasPrefix(last.Text, "Артём пока не добавляет «") {
		t.Fatalf("родителю — отказ: %q", last.Text)
	}
}

func TestNotify_RemovedAndLeft(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	e.do("DELETE", "/api/v1/family/members/"+f.parent.MemberID, kid, nil)
	if last := e.fake.Last(olgaMax); last.Text != "Артём удаляет вас из траектории Артёма — олимпиады и напоминания больше не придут. Чтобы собрать свою подборку, отправьте /start." {
		t.Fatalf("удалённому: %q", last.Text)
	}

	uid, _ := e.st.UpsertUser(context.Background(), 900000003, "Игорь")
	if _, err := e.st.AddMember(context.Background(), f.trajectoryID, uid, "parent"); err != nil {
		t.Fatal(err)
	}
	igor := e.login(900000003, "Игорь")
	if r := e.do("POST", "/api/v1/family/leave", igor, nil); r.code != 204 {
		t.Fatalf("выход: %d %s", r.code, r.raw)
	}
	if last := e.fake.Last(artemMax); last.Text != "Игорь больше не в траектории." {
		t.Fatalf("остальным — о выходе: %q", last.Text)
	}
}

// Сбой MAX не ломает сценарий: изменение сохранено, ответ — 200.
func TestNotify_SendFailureDoesNotFailRequest(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	parent := e.login(olgaMax, "Ольга")
	id, _, _ := e.st.AddTrackerItem(context.Background(), f.trajectoryID, "p669-8-informatika", f.kid.MemberID)
	e.fake.Fail = func(int64) error { return errBoom }
	if r := e.do("PUT", "/api/v1/tracker/"+id+"/registered", parent, nil); r.code != 200 || r.body["registered_at"] == nil {
		t.Fatalf("отметка при недоступном MAX: %d %s", r.code, r.raw)
	}
}

var errBoom = errors.New("MAX недоступен")
