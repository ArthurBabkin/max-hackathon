package api

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// Биология Всесибирской олимпиады в сиде: регистрация 12–30.10, отборочный 01.11,
// регистрация на заключительный до 05.03, финал 07.03.2027.
const (
	bioProfile = "p669-14-biologiya"
	bioReg1    = "p669-14-biologiya:registration:1"
	bioQual    = "p669-14-biologiya:qualifying:1"
	bioReg2    = "p669-14-biologiya:registration:2"
	bioFinal   = "p669-14-biologiya:final:1"
)

func stageByID(t *testing.T, item map[string]any, id string) map[string]any {
	t.Helper()
	for _, s := range list(t, item["stages"]) {
		if s["id"] == id {
			return s
		}
	}
	t.Fatalf("этапа %s нет: %v", id, item["stages"])
	return nil
}

func strs(v any) []string {
	out := []string{}
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func action(item map[string]any) string {
	a, ok := item["action"].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := a["stage_id"].(string)
	return a["type"].(string) + " " + id
}

func TestTracker_StagesAndStatus(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	e.track(f, bioProfile, false)

	item := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["items"])[0]
	if item["status"] != "open" || item["outcome"] != nil || action(item) != "register "+bioReg1 {
		t.Fatalf("до регистрации — open и строка «Зарегистрировался?»: %v %v %v", item["status"], item["outcome"], item["action"])
	}
	if n := len(list(t, item["stages"])); n != 4 {
		t.Fatalf("все четыре этапа по порядку: %d", n)
	}
	r1 := stageByID(t, item, bioReg1)
	if r1["state"] != "current" || r1["can_register"] != true || r1["registered"] != false ||
		r1["subtitle"] != "до 30 октября, онлайн" || r1["deadline_at"] == nil || len(strs(r1["results"])) != 0 {
		t.Fatalf("регистрация: %v", r1)
	}
	q := stageByID(t, item, bioQual)
	if !slices.Equal(strs(q["results"]), []string{"passed", "failed"}) || len(strs(q["results_allowed"])) != 0 ||
		q["can_register"] != false || q["result"] != nil || q["state"] != "future" {
		t.Fatalf("отборочный ещё не начался — итог не отметить: %v", q)
	}
	if fin := stageByID(t, item, bioFinal); !slices.Equal(strs(fin["results"]), []string{"winner", "prizer", "participant"}) {
		t.Fatalf("финал — дипломы: %v", fin["results"])
	}
}

func TestTracker_StageMarkFlow(t *testing.T) {
	e := newEnv(t)
	e.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	f := e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	parent := e.login(olgaMax, "Ольга")
	id := e.track(f, bioProfile, false)
	mark := func(token, stage string, body map[string]any) resp {
		return e.do("PUT", "/api/v1/tracker/"+id+"/stages/"+stage, token, body)
	}

	// Отборочный закончился, регистрация не отмечена — итог спрашиваем
	// только у участвующих, а строка — «Зарегистрировался?».
	item := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["items"])[0]
	if item["status"] != "finished" || item["outcome"] != "missed" || action(item) != "register "+bioReg1 {
		t.Fatalf("регистрация закрылась без отметки: %v %v %v", item["status"], item["outcome"], item["action"])
	}

	r := mark(parent, bioQual, map[string]any{"result": "passed"})
	if r.code != 200 || r.body["status"] != "active" || r.body["registered_at"] == nil ||
		stageByID(t, r.body, bioQual)["result"] != "passed" || action(r.body) != "register "+bioReg2 {
		t.Fatalf("прошёл отборочный — участвует, дальше регистрация на финал: %d %s", r.code, r.raw)
	}
	if got := e.fake.To(artemMax); len(got) != 1 || !strings.Contains(got[0].Msg.Text, "Ольга") ||
		!strings.Contains(got[0].Msg.Text, "прошёл") && !strings.Contains(got[0].Msg.Text, "пройден") {
		t.Fatalf("ученику — сообщение об отметке: %v", got)
	}

	if r := mark(kid, bioQual, map[string]any{"result": "winner"}); r.code != 400 || r.errCode() != "BAD_REQUEST" {
		t.Fatalf("диплом у отборочного — 400: %d %s", r.code, r.raw)
	}
	if r := mark(kid, bioQual, map[string]any{"result": "champion"}); r.code != 400 {
		t.Fatalf("неизвестный итог — 400: %d %s", r.code, r.raw)
	}
	if r := mark(kid, "p669-8-informatika:final:1", map[string]any{"registered": true}); r.code != 404 {
		t.Fatalf("этап другой олимпиады — 404: %d %s", r.code, r.raw)
	}
	if r := e.do("PUT", "/api/v1/tracker/nope/stages/"+bioQual, kid, map[string]any{"result": "passed"}); r.code != 404 {
		t.Fatalf("нет пункта — 404: %d", r.code)
	}

	if r := mark(kid, bioReg2, map[string]any{"registered": true}); r.code != 200 ||
		stageByID(t, r.body, bioReg2)["registered"] != true {
		t.Fatalf("регистрация на финал: %d %s", r.code, r.raw)
	}
	if r := mark(kid, bioQual, map[string]any{"result": "failed"}); r.code != 409 || r.errCode() != "CONFLICT" {
		t.Fatalf("«не прошёл» при регистрации на финал — 409: %d %s", r.code, r.raw)
	}
	if r := e.do("DELETE", "/api/v1/tracker/"+id+"/registered", kid, nil); r.code != 409 {
		t.Fatalf("снять регистрацию при итогах — 409: %d %s", r.code, r.raw)
	}

	mark(kid, bioReg2, map[string]any{"registered": false})
	r = mark(kid, bioQual, map[string]any{"result": "failed"})
	if r.code != 200 || r.body["status"] != "finished" || r.body["outcome"] != "failed" || r.body["action"] != nil ||
		stageByID(t, r.body, bioFinal)["state"] != "locked" || stageByID(t, r.body, bioReg2)["can_register"] != false {
		t.Fatalf("не прошёл — завершено, дальше серое: %d %s", r.code, r.raw)
	}
	r = mark(kid, bioQual, map[string]any{"result": nil})
	if r.code != 200 || r.body["status"] != "active" || stageByID(t, r.body, bioQual)["result"] != nil {
		t.Fatalf("итог снимается — всё возвращается: %d %s", r.code, r.raw)
	}
}

// Отборочный закончился, итога нет — строка действия спрашивает итог.
func TestTracker_ActionAsksResult(t *testing.T) {
	e := newEnv(t)
	e.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	f := e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	e.track(f, bioProfile, true)
	item := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["items"])[0]
	if action(item) != "result "+bioQual || stageByID(t, item, bioQual)["asking"] != true ||
		!slices.Equal(strs(stageByID(t, item, bioQual)["results_allowed"]), []string{"passed", "failed"}) {
		t.Fatalf("спрашиваем итог отборочного: %v", item)
	}
}

// Олимпиада без этапа-регистрации (химия МГУ: два тура отбора и два
// заключительных): «участвую» — та же галочка, без этапа.
func TestTracker_ActionWithoutRegistrationStage(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	kid := e.login(artemMax, "Артём")
	e.track(f, "p669-35-himiya", false)
	item := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["items"])[0]
	if item["status"] != "open" || action(item) != "register " {
		t.Fatalf("без регистрации — «участвую» без этапа: %v %v", item["status"], item["action"])
	}
}
