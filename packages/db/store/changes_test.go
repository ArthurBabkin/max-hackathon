package store

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func seedChanges(t *testing.T, s *Store, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		if err := s.db.QueryRow(context.Background(), `
			INSERT INTO content_changes (entity, entity_id, summary)
			VALUES ('olympiad_profile', $1, 'stages') RETURNING id::text`,
			"p-"+strconv.Itoa(i)).Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// Строка в content_change_deliveries — это захват, а не отметка о доставке.
// Различать обязательно: пара, занятая соседним запуском, ещё не обслужена, и
// закрывать по ней изменение нельзя — иначе оно не достанется никому.
func TestClaimChangeDeliveries_DistinguishesBusyFromDelivered(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	ids := seedChanges(t, s, 3)

	claimed, unfinished, err := s.ClaimChangeDeliveries(ctx, ids, f.creatorMember)
	if err != nil || !slices.Equal(sorted(claimed), sorted(ids)) || len(unfinished) != 0 {
		t.Fatalf("свободные пары достаются нам целиком: claimed=%v unfinished=%v %v", claimed, unfinished, err)
	}

	// Тот же вызов ещё раз — так выглядит соседний запуск, ещё не дождавшийся
	// ответа MAX. Слать нельзя, но и обслуженными пары считать нельзя.
	claimed, unfinished, err = s.ClaimChangeDeliveries(ctx, ids, f.creatorMember)
	if err != nil || len(claimed) != 0 || !slices.Equal(sorted(unfinished), sorted(ids)) {
		t.Fatalf("занятые незавершённые: claimed=%v unfinished=%v %v", claimed, unfinished, err)
	}

	// Доставленная пара обслужена: ни занимать, ни ждать её больше не нужно.
	if err := s.FinishChangeDeliveries(ctx, ids[:1], f.creatorMember, "mid-1"); err != nil {
		t.Fatal(err)
	}
	claimed, unfinished, err = s.ClaimChangeDeliveries(ctx, ids, f.creatorMember)
	if err != nil || len(claimed) != 0 || !slices.Equal(sorted(unfinished), sorted(ids[1:])) {
		t.Fatalf("после доставки первой: claimed=%v unfinished=%v %v", claimed, unfinished, err)
	}

	// Освобождённая после сбоя — снова наша.
	if err := s.ReleaseChangeDeliveries(ctx, ids[1:2], f.creatorMember); err != nil {
		t.Fatal(err)
	}
	claimed, unfinished, err = s.ClaimChangeDeliveries(ctx, ids, f.creatorMember)
	if err != nil || !slices.Equal(claimed, ids[1:2]) || !slices.Equal(unfinished, ids[2:]) {
		t.Fatalf("после освобождения второй: claimed=%v unfinished=%v %v", claimed, unfinished, err)
	}
}

// Заблокировавший бота закрывается без id сообщения: повторять нечего, но и
// «в работе» пара висеть не должна.
func TestClaimChangeDeliveries_BlockedRecipientIsClosedWithoutMessage(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	ids := seedChanges(t, s, 1)

	if _, _, err := s.ClaimChangeDeliveries(ctx, ids, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishChangeDeliveries(ctx, ids, f.creatorMember, ""); err != nil {
		t.Fatal(err)
	}
	claimed, unfinished, err := s.ClaimChangeDeliveries(ctx, ids, f.creatorMember)
	if err != nil || len(claimed) != 0 || len(unfinished) != 0 {
		t.Fatalf("пара обслужена: claimed=%v unfinished=%v %v", claimed, unfinished, err)
	}
	var mid *string
	if err := s.db.QueryRow(ctx,
		`SELECT max_message_id FROM content_change_deliveries WHERE change_id = $1`, ids[0]).Scan(&mid); err != nil {
		t.Fatal(err)
	}
	if mid != nil {
		t.Errorf("сообщения не было, id взяться неоткуда: %q", *mid)
	}
}

// PendingContentChanges ничего не помечает — в этом вся суть переделки: пока
// доставка не подтверждена, изменение остаётся в очереди.
func TestPendingContentChanges_DoesNotMarkAnything(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	want := seedChanges(t, s, 2)

	for i := 0; i < 2; i++ {
		ids, _, err := s.PendingContentChanges(ctx, 100)
		if err != nil || !slices.Equal(sorted(ids), sorted(want)) {
			t.Fatalf("запуск %d: %v %v", i+1, ids, err)
		}
	}
	now := time.Now()
	if n, err := s.MarkChangesNotified(ctx, want[:1], now); err != nil || n != 1 {
		t.Fatalf("закрытие одного изменения: %d %v", n, err)
	}
	ids, _, err := s.PendingContentChanges(ctx, 100)
	if err != nil || !slices.Equal(ids, want[1:]) {
		t.Fatalf("закрытое больше не берём: %v %v", ids, err)
	}
	// Повторное закрытие — ничего не меняет.
	if n, _ := s.MarkChangesNotified(ctx, want[:1], now); n != 0 {
		t.Errorf("закрытие идемпотентно, изменено строк: %d", n)
	}
}

// Недоставляемое изменение не должно перепроверяться вечно, иначе очередь и
// таблица доставок растут без верхней границы.
func TestGiveUpOldChanges_AndPurge(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	ids := seedChanges(t, s, 2)
	now := time.Now()
	if _, err := s.db.Exec(ctx,
		`UPDATE content_changes SET created_at = $2 WHERE id = $1`, ids[0], now.Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	n, err := s.GiveUpOldChanges(ctx, now, now.Add(-7*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("сдаёмся только по старому: %d %v", n, err)
	}
	pending, _, _ := s.PendingContentChanges(ctx, 100)
	if !slices.Equal(pending, ids[1:]) {
		t.Fatalf("свежее остаётся в очереди: %v", pending)
	}

	// Чистка забирает только закрытое и давнее.
	if n, err := s.PurgeContentChanges(ctx, now.Add(-30*24*time.Hour)); err != nil || n != 0 {
		t.Fatalf("восьмидневное ещё не стираем: %d %v", n, err)
	}
	if n, err := s.PurgeContentChanges(ctx, now); err != nil || n != 1 {
		t.Fatalf("закрытое и давнее стёрто: %d %v", n, err)
	}
	var left int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM content_changes`).Scan(&left)
	if left != 1 {
		t.Fatalf("открытое изменение не трогаем: осталось %d", left)
	}
}

// Сдвинулись даты олимпиады, участие в которой закончилось итогом, —
// сообщать об этом семье незачем: сроков для неё больше нет.
func TestPendingContentChanges_SkipsClosedOlympiadStages(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	if _, err := s.db.Exec(ctx, `INSERT INTO content_changes (entity, entity_id, summary)
		VALUES ('olympiad_profile', $1, 'stages')`, bioProfile); err != nil {
		t.Fatal(err)
	}
	_, items, err := s.PendingContentChanges(ctx, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("до итога семья узнаёт о новых датах: %v %v", items, err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result)
		VALUES ($1, $2, 'failed')`, item, bioQual); err != nil {
		t.Fatal(err)
	}
	if _, items, _ = s.PendingContentChanges(ctx, 10); len(items) != 0 {
		t.Fatalf("после «не прошёл» о датах не сообщаем: %v", items)
	}
}
