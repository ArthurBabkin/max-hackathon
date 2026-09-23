package notifier

import (
	"context"
	"strings"
	"sync"
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
		StudentName: "Артём", Grade: 9, RegionCode: "16", TZ: "Europe/Moscow", DirectionIDs: []string{direction},
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

// second — ещё одна траектория с той же олимпиадой в трекере: нужна, чтобы
// в запуске было две группы и обрыв на первой был заметен на второй.
func (e *env) second(t *testing.T, maxUser int64) store.Member {
	t.Helper()
	ctx := context.Background()
	direction := "napr-09-03-04"
	u, err := e.st.UpsertUser(ctx, maxUser, "Игорь")
	if err != nil {
		t.Fatal(err)
	}
	m, err := e.st.CreateTrajectory(ctx, store.NewTrajectory{CreatorUserID: u, Role: "kid",
		StudentName: "Игорь", Grade: 10, RegionCode: "16", TZ: "Europe/Moscow",
		DirectionIDs: []string{direction}, SubjectCodes: []string{"inf"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.st.AddTrackerItem(ctx, m.TrajectoryID, tracked, m.MemberID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(ctx, `DELETE FROM content_changes`); err != nil {
		t.Fatal(err)
	}
	return m
}

func (e *env) pending(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(context.Background(),
		`SELECT count(*) FROM content_changes WHERE notified_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Ради этого всё и затевалось: сбой отправки одному участнику оставляет
// изменение в очереди, а следующий запуск дошлёт только ему — тот, кому уже
// ушло, второго сообщения не получит.
func TestRun_FailedRecipientIsRetriedWithoutRepeatingTheOther(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			return &maxapi.Error{Status: 503, Message: "temporarily unavailable"}
		}
		return nil
	}
	res := e.run(t)
	if res.Sent != 1 || res.Failed != 1 || res.Postponed != 1 || res.Dropped != 0 {
		t.Fatalf("первый запуск: %+v", res)
	}
	if n := e.pending(t); n != 1 {
		t.Fatalf("недоставленное остаётся в очереди, а не сгорает: %d", n)
	}

	e.fake.Fail = nil
	if res := e.run(t); res.Sent != 1 || res.Failed != 0 || res.Postponed != 0 {
		t.Fatalf("второй запуск досылает только отставшему: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("все обслужены — изменение закрыто: %d в очереди", n)
	}
	if n := len(e.fake.To(kidMax)); n != 1 {
		t.Errorf("ученику ровно одно сообщение, а не повтор: %d", n)
	}
	if n := len(e.fake.To(parentMax)); n != 1 {
		t.Errorf("родителю ровно одно сообщение: %d", n)
	}
}

// Остановивший бота — случай терминальный: повторять нечего, и изменение по
// нему закрывается, а не висит до истечения срока попыток.
func TestRun_BlockedRecipientIsNotRetried(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			return &maxapi.Error{Status: 403, Message: "chat.denied"}
		}
		return nil
	}
	if res := e.run(t); res.Sent != 1 || res.Skipped != 1 || res.Postponed != 0 {
		t.Fatalf("первый запуск: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("повторять нечего — изменение закрыто: %d в очереди", n)
	}
	e.fake.Fail = nil
	if res := e.run(t); res.Changes != 0 || res.Sent != 0 {
		t.Fatalf("второй запуск молчит: %+v", res)
	}
}

// Таймаут функции больше не теряет необработанное: группы, до которых обход
// не дошёл, остаются в очереди и уходят следующим запуском.
func TestRun_TimeoutPostponesInsteadOfLosing(t *testing.T) {
	e := setup(t)
	e.second(t, 900000003)
	e.moveDeadline(t, tracked)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Обрываем запуск на первой же отправке: до второй группы обход не дойдёт.
	e.fake.Fail = func(int64) error { cancel(); return nil }
	res, err := e.w.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Postponed != 1 || res.Dropped != 0 {
		t.Fatalf("необработанное отложено, а не потеряно: %+v", res)
	}
	if n := e.pending(t); n != 1 {
		t.Fatalf("изменение осталось в очереди: %d", n)
	}
	e.fake.Fail = nil
	if res := e.run(t); res.Postponed != 0 {
		t.Fatalf("второй запуск добирает остаток: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("всё обслужено — изменение закрыто: %d в очереди", n)
	}
	if n := len(e.fake.Sent); n != 3 {
		t.Fatalf("три получателя, по одному сообщению каждому: %d", n)
	}
	for _, u := range []int64{kidMax, parentMax, 900000003} {
		if n := len(e.fake.To(u)); n != 1 {
			t.Errorf("получателю %d ровно одно сообщение: %d", u, n)
		}
	}
}

// Два запуска сразу — ровно то, чего serverless не запрещает. Каждая пара
// достаётся кому-то одному, поэтому дубля не будет ни у кого.
func TestRun_TwoRunsInParallelSendOnceEach(t *testing.T) {
	e := setup(t)
	e.second(t, 900000003)
	e.moveDeadline(t, tracked)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.w.Run(context.Background())
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []int64{kidMax, parentMax, 900000003} {
		if n := len(e.fake.To(u)); n != 1 {
			t.Errorf("получателю %d ровно одно сообщение, а не дубль: %d", u, n)
		}
	}
	// Пары, занятые соседом, обслуженными не считаются, поэтому изменение
	// могло остаться в очереди. Оно не потеряно — следующий запуск закроет
	// его, никому ничего не прислав.
	if res := e.run(t); res.Sent != 0 {
		t.Fatalf("третий запуск не шлёт ничего: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("изменение закрыто: %d в очереди", n)
	}
	if n := len(e.fake.Sent); n != 3 {
		t.Fatalf("всего сообщений: %d", n)
	}
}

// Контекст запуска мог истечь сразу после последней отправки. Обслуженное
// уже обслужено, и закрыть изменение обязаны: иначе запуск вернул бы ошибку,
// а событие осталось бы в очереди и перепроверялось до истечения срока.
func TestRun_ClosesServedChangesEvenIfContextExpired(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Родитель — последний получатель: к моменту отмены разослано всё.
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			cancel()
		}
		return nil
	}
	res, err := e.w.Run(ctx)
	if err != nil {
		t.Fatalf("отмена после отправки не должна ронять запуск: %v", err)
	}
	if res.Sent != 2 || res.Postponed != 0 {
		t.Fatalf("оба получателя обслужены: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("обслуженное изменение закрыто: %d в очереди", n)
	}
}

func (e *env) firstChange(t *testing.T) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(context.Background(),
		`SELECT id::text FROM content_changes WHERE notified_at IS NULL ORDER BY created_at LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Захват, по которому результат так и не записали, иначе висел бы вечно: пара
// выглядела бы занятой соседом, изменение не закрылось бы никогда, а через
// неделю попало бы в отчёт о потерях — хотя сообщение ушло.
func TestRun_StaleClaimIsReapedAndRedelivered(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	cid := e.firstChange(t)

	// Так выглядит запуск, умерший между отправкой и записью результата.
	if _, err := e.db.Exec(context.Background(), `
		INSERT INTO content_change_deliveries (change_id, member_id, claimed_at) VALUES ($1, $2, $3)`,
		cid, e.parent.MemberID, e.w.now().Add(-staleClaimTTL-time.Minute)); err != nil {
		t.Fatal(err)
	}

	res := e.run(t)
	if res.Stale != 1 {
		t.Fatalf("брошенный захват освобождён и посчитан: %+v", res)
	}
	if res.Sent != 2 || res.Postponed != 0 {
		t.Fatalf("после освобождения сводка уходит обоим: %+v", res)
	}
	if n := e.pending(t); n != 0 {
		t.Fatalf("изменение закрыто: %d в очереди", n)
	}
}

// Свежий захват принадлежит работающему прямо сейчас запуску: отнять его —
// значит прислать человеку сводку дважды.
func TestRun_FreshClaimIsNotReaped(t *testing.T) {
	e := setup(t)
	e.moveDeadline(t, tracked)
	cid := e.firstChange(t)

	if _, err := e.db.Exec(context.Background(),
		`INSERT INTO content_change_deliveries (change_id, member_id) VALUES ($1, $2)`,
		cid, e.parent.MemberID); err != nil {
		t.Fatal(err)
	}
	res := e.run(t)
	if res.Stale != 0 {
		t.Fatalf("свежий захват не трогаем: %+v", res)
	}
	if res.Sent != 1 || res.Postponed != 1 {
		t.Fatalf("занятому получателю не шлём, изменение откладываем: %+v", res)
	}
	if n := e.pending(t); n != 1 {
		t.Fatalf("изменение осталось в очереди: %d", n)
	}
}
