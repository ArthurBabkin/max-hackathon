package notifier

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

const (
	kidMax    = 900000001
	parentMax = 900000002
	// «Высшая проба», информатика — в трекере ученика.
	tracked = "p669-8-informatika"
	// «ТехноКубок», информатика — не в трекере, но у ВШЭ из профиля есть
	// по нему льгота, а информатика среди предметов ученика.
	byUniversity = "p669-57-informatika"
)

type env struct {
	db     *pgxpool.Pool
	st     *store.Store
	fake   *maxtest.Fake
	w      *Worker
	kid    store.Member
	parent store.Member
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db := dbtest.Open(t)
	st := store.New(db)
	kidUser, _ := st.UpsertUser(ctx, kidMax, "Артём")
	direction := "napr-09-03-04"
	kid, err := st.CreateTrajectory(ctx, store.NewTrajectory{CreatorUserID: kidUser, Role: "kid",
		StudentName: "Артём", Grade: 9, RegionCode: "16", TZ: "Europe/Moscow", GoalStatus: "known", DirectionID: &direction,
		SubjectCodes: []string{"inf"}, UniversityIDs: []string{"hse", "innopolis"}})
	if err != nil {
		t.Fatal(err)
	}
	parentUser, _ := st.UpsertUser(ctx, parentMax, "Ольга")
	parent, err := st.AddMember(ctx, kid.TrajectoryID, parentUser, "parent")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.AddTrackerItem(ctx, kid.TrajectoryID, tracked, kid.MemberID); err != nil {
		t.Fatal(err)
	}
	// Создание траектории могло породить события — начинаем с чистой очереди.
	if _, err := db.Exec(ctx, `DELETE FROM content_changes`); err != nil {
		t.Fatal(err)
	}
	fake := &maxtest.Fake{}
	w := New(st, fake, config.Bot{Name: "test_bot", ID: 42})
	w.now = func() time.Time { return time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC) }
	return &env{db: db, st: st, fake: fake, w: w, kid: kid, parent: parent}
}

func (e *env) run(t *testing.T) Result {
	t.Helper()
	res, err := e.w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// moveDeadline сдвигает первый срок профиля на сутки, как это сделал бы
// контент-администратор, и возвращает всё обратно после теста.
func (e *env) moveDeadline(t *testing.T, profile string) {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := e.db.QueryRow(ctx, `
		UPDATE stages SET deadline_at = deadline_at + interval '1 day'
		WHERE id = (SELECT id FROM stages WHERE olympiad_profile_id = $1 AND deadline_at IS NOT NULL
		            ORDER BY deadline_at LIMIT 1)
		RETURNING id::text`, profile).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = e.db.Exec(context.Background(),
			`UPDATE stages SET deadline_at = deadline_at - interval '1 day' WHERE id::text = $1`, id)
	})
}

// raiseEgeMin меняет минимальный балл ЕГЭ для льготы профиля в вузе.
func (e *env) raiseEgeMin(t *testing.T, profile, university string) {
	t.Helper()
	ctx := context.Background()
	tag, err := e.db.Exec(ctx, `UPDATE benefits SET ege_min = ege_min + 5
		WHERE olympiad_profile_id = $1 AND university_id = $2`, profile, university)
	if err != nil || tag.RowsAffected() == 0 {
		t.Fatalf("льгота %s@%s: %v", profile, university, err)
	}
	t.Cleanup(func() {
		_, _ = e.db.Exec(context.Background(), `UPDATE benefits SET ege_min = ege_min - 5
			WHERE olympiad_profile_id = $1 AND university_id = $2`, profile, university)
	})
}

func buttons(m maxapi.NewMessage) []maxapi.Button {
	var out []maxapi.Button
	for _, row := range m.Keyboard() {
		out = append(out, row...)
	}
	return out
}

func TestRun_StageChangeGoesToEveryMemberOnce(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)

	res := e.run(t)
	if res.Changes != 1 || res.Trajectories != 1 || res.Sent != 2 {
		t.Fatalf("первый запуск: %+v", res)
	}
	kid := e.fake.Last(kidMax)
	if !strings.Contains(kid.Text, "из твоей траектории") ||
		!strings.Contains(kid.Text, "• «Высшая проба», информатика — сроки этапов") {
		t.Fatalf("ученику: %q", kid.Text)
	}
	if parent := e.fake.Last(parentMax); !strings.Contains(parent.Text, "из траектории Артёма") {
		t.Fatalf("родителю — про траекторию ученика: %q", parent.Text)
	}
	bs := buttons(kid)
	if len(bs) != 2 {
		t.Fatalf("кнопки: %s", maxtest.Buttons(kid))
	}
	if card := bs[0]; card.Type != "open_app" || card.Text != "Открыть карточку" || card.Payload != "o_"+tracked {
		t.Fatalf("карточка олимпиады: %+v", card)
	}
	if site := bs[1]; site.Type != "link" || !strings.HasPrefix(site.URL, "https://") {
		t.Fatalf("первоисточник: %+v", site)
	}

	if res := e.run(t); res.Changes != 0 || res.Sent != 0 {
		t.Fatalf("повторный запуск ничего не шлёт: %+v", res)
	}
	if n := len(e.fake.Sent); n != 2 {
		t.Fatalf("всего сообщений: %d", n)
	}
}

func TestRun_SameValuesAreNotAChange(t *testing.T) {
	e := setup(t)
	// Повторный прогон сида пишет те же значения — это не изменение.
	if _, err := e.db.Exec(context.Background(),
		`UPDATE stages SET deadline_at = deadline_at WHERE olympiad_profile_id = $1`, tracked); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(context.Background(),
		`UPDATE benefits SET ege_min = ege_min WHERE olympiad_profile_id = $1`, tracked); err != nil {
		t.Fatal(err)
	}
	if res := e.run(t); res.Changes != 0 || res.Sent != 0 {
		t.Fatalf("событий нет: %+v", res)
	}
}

func TestRun_ChangesAreGroupedIntoOneMessage(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	e.raiseEgeMin(t, tracked, "innopolis")
	e.raiseEgeMin(t, byUniversity, "hse")

	res := e.run(t)
	if res.Changes != 3 || res.Trajectories != 1 || res.Sent != 2 {
		t.Fatalf("одна рассылка на траекторию: %+v", res)
	}
	kid := e.fake.Last(kidMax)
	for _, line := range []string{
		"• «Высшая проба», информатика — сроки этапов, льготы (Иннополис)",
		"• «ТехноКубок», информатика — льготы (ВШЭ)",
	} {
		if !strings.Contains(kid.Text, line) {
			t.Fatalf("нет строки %q в %q", line, kid.Text)
		}
	}
	got := maxtest.Buttons(kid)
	want := "Карточка «Высшая проба» | Сайт олимпиады | Карточка «ТехноКубок» | Правила приёма ВШЭ"
	if got != want {
		t.Fatalf("кнопки:\n got %s\nwant %s", got, want)
	}
	if bs := buttons(kid); bs[2].Payload != "o_"+byUniversity || !strings.HasPrefix(bs[3].URL, "https://ba.hse.ru/") {
		t.Fatalf("карточка и правила вуза: %+v %+v", bs[2], bs[3])
	}
}

func TestRun_UnrelatedChangeIsMarkedButSilent(t *testing.T) {
	e := setup(t)
	// МГУ нет ни в профиле, ни «ТехноКубка» в трекере.
	e.raiseEgeMin(t, byUniversity, "msu")
	if res := e.run(t); res.Changes != 1 || res.Sent != 0 || res.Trajectories != 0 {
		t.Fatalf("никого не касается: %+v", res)
	}
	var pending int
	_ = e.db.QueryRow(context.Background(), `SELECT count(*) FROM content_changes WHERE notified_at IS NULL`).Scan(&pending)
	if pending != 0 {
		t.Fatalf("событие закрыто: %d в очереди", pending)
	}
}

func TestRun_BenefitOfOtherSubjectIsSilent(t *testing.T) {
	e := setup(t)
	// ВШЭ в профиле, но математики нет среди предметов ученика.
	e.raiseEgeMin(t, "p669-8-matematika", "hse")
	if res := e.run(t); res.Sent != 0 {
		t.Fatalf("чужой предмет: %+v", res)
	}
}

func TestRun_DeletedTrajectoryIsSilent(t *testing.T) {
	e := setup(t)
	if err := e.st.DeleteTrajectory(context.Background(), e.kid.TrajectoryID, e.kid.MemberID); err != nil {
		t.Fatal(err)
	}
	e.moveDeadline(t, tracked)
	if res := e.run(t); res.Sent != 0 {
		t.Fatalf("удалённой траектории не пишем: %+v", res)
	}
}

func TestRun_BlockedRecipientIsSkipped(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			return &maxapi.Error{Status: 403, Message: "chat.denied"}
		}
		return nil
	}
	if res := e.run(t); res.Sent != 1 || res.Skipped != 1 {
		t.Fatalf("остановивший бота пропущен: %+v", res)
	}
}

func TestRun_WithoutTokenChangesWait(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	e.w.max = nil
	if res := e.run(t); res.Changes != 0 || res.Sent != 0 {
		t.Fatalf("без токена события не забираются: %+v", res)
	}
	e.w.max = e.fake
	if res := e.run(t); res.Changes != 1 || res.Sent != 2 {
		t.Fatalf("с токеном — дошли: %+v", res)
	}
}
