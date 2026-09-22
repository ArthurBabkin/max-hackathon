package store

import (
	"context"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func TestAiExchange_HistoryIsPersonalAndOrdered(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	parent, err := s.AddMember(ctx, f.trajectoryID, uid, "parent")
	if err != nil {
		t.Fatal(err)
	}
	src := Source{ID: "src-x", Kind: "rules", Title: "ВШЭ: правила", URL: "https://example.org"}
	for _, q := range []string{"первый", "второй"} {
		if _, _, err := s.SaveAiExchange(ctx, f.creatorMember, q, AiMessage{Text: "ответ на " + q,
			CardRefs: []AiCardRef{{Type: "university", ID: "hse", Title: "ВШЭ"}}, Sources: []Source{src}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.SaveAiExchange(ctx, parent.MemberID, "чужой", AiMessage{Text: "нет", Refused: true}); err != nil {
		t.Fatal(err)
	}

	got, err := s.AiMessages(ctx, f.creatorMember, 50)
	if err != nil || len(got) != 4 {
		t.Fatalf("история ученика: %v %v", got, err)
	}
	if got[0].Text != "первый" || got[1].Text != "ответ на первый" || got[3].Role != "assistant" ||
		got[1].CardRefs[0].ID != "hse" || got[1].Sources[0].URL != "https://example.org" || len(got[0].CardRefs) != 0 {
		t.Fatalf("порядок и поля: %+v", got)
	}
	if last2, _ := s.AiMessages(ctx, f.creatorMember, 2); len(last2) != 2 || last2[0].Text != "второй" {
		t.Fatalf("последние две — от старой к новой: %+v", last2)
	}
	if other, _ := s.AiMessages(ctx, parent.MemberID, 50); len(other) != 2 || !other[1].Refused {
		t.Fatalf("у родителя своя история: %+v", other)
	}
}

func TestAiLookups(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	names, err := s.OlympiadNames(ctx)
	if err != nil || len(names) < 70 {
		t.Fatalf("олимпиады: %d %v", len(names), err)
	}
	if src, err := s.OrderSource(ctx); err != nil || src.Kind != "order" {
		t.Fatalf("приказ: %+v %v", src, err)
	}
	if src, err := s.UniversityRules(ctx, "hse"); err != nil || src == nil || src.Title != "ВШЭ: правила приёма" {
		t.Fatalf("правила ВШЭ: %+v %v", src, err)
	}
}
