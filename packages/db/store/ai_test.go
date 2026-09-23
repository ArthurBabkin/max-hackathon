package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// aiFamily — ученик-создатель и родитель одной траектории: у каждого свои чаты (F37).
func aiFamily(t *testing.T, s *Store) (kid, parent string) {
	t.Helper()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, err := s.UpsertUser(context.Background(), 900000002, "Ольга")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.AddMember(context.Background(), f.trajectoryID, uid, "parent")
	if err != nil {
		t.Fatal(err)
	}
	return f.creatorMember, p.MemberID
}

func startChat(t *testing.T, s *Store, member, title, question string) AiChat {
	t.Helper()
	ex, err := s.StartAiChat(context.Background(), member, title, question, AiMessage{Text: "ответ на " + question})
	if err != nil {
		t.Fatal(err)
	}
	return ex.Chat
}

func TestStartAiChat_CreatesChatWithFirstExchange(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, _ := aiFamily(t, s)
	src := Source{ID: "src-x", Kind: "rules", Title: "ВШЭ: правила", URL: "https://example.org"}
	ex, err := s.StartAiChat(ctx, kid, "Чат 21 сентября", "первый", AiMessage{Text: "ответ",
		CardRefs: []AiCardRef{{Type: "university", ID: "hse", Title: "ВШЭ"}}, Sources: []Source{src}})
	if err != nil {
		t.Fatal(err)
	}
	if ex.Chat.ID == "" || ex.Chat.Title != "Чат 21 сентября" || ex.Question.Text != "первый" || ex.Answer.Role != "assistant" {
		t.Fatalf("новый чат: %+v", ex)
	}
	got, err := s.AiChatMessages(ctx, kid, ex.Chat.ID, 50)
	if err != nil || len(got) != 2 {
		t.Fatalf("история чата: %+v %v", got, err)
	}
	if got[0].Text != "первый" || got[1].Text != "ответ" || got[1].CardRefs[0].ID != "hse" ||
		got[1].Sources[0].URL != "https://example.org" || len(got[0].CardRefs) != 0 {
		t.Fatalf("порядок и поля: %+v", got)
	}
}

func TestAiChats_ArePersonalAndSeparate(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, parent := aiFamily(t, s)
	a := startChat(t, s, kid, "Чат А", "про олимпиады")
	b := startChat(t, s, kid, "Чат Б", "про вузы")
	c := startChat(t, s, parent, "Чат родителя", "родительский")

	mine, err := s.AiChats(ctx, kid)
	if err != nil || len(mine) != 2 || mine[0].ID != b.ID || mine[1].ID != a.ID {
		t.Fatalf("чаты ученика, свежий сверху: %+v %v", mine, err)
	}
	if theirs, _ := s.AiChats(ctx, parent); len(theirs) != 1 || theirs[0].ID != c.ID {
		t.Fatalf("у родителя свой чат: %+v", theirs)
	}
	if msgs, _ := s.AiChatMessages(ctx, kid, b.ID, 50); len(msgs) != 2 || msgs[0].Text != "про вузы" {
		t.Fatalf("в чате Б только его реплики: %+v", msgs)
	}
	if _, err := s.AiChat(ctx, parent, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой чат должен быть не найден: %v", err)
	}
	if msgs, _ := s.AiChatMessages(ctx, parent, a.ID, 50); len(msgs) != 0 {
		t.Fatalf("реплики чужого чата не отдаются: %+v", msgs)
	}
	if _, err := s.SaveAiExchange(ctx, parent, a.ID, "влезть", AiMessage{Text: "нет"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("писать в чужой чат нельзя: %v", err)
	}
	if msgs, _ := s.AiChatMessages(ctx, kid, a.ID, 50); len(msgs) != 2 {
		t.Fatalf("в чат ученика ничего не добавилось: %+v", msgs)
	}
}

func TestSaveAiExchange_MovesChatToTop(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, _ := aiFamily(t, s)
	a := startChat(t, s, kid, "Чат А", "первый")
	startChat(t, s, kid, "Чат Б", "второй")

	ex, err := s.SaveAiExchange(ctx, kid, a.ID, "снова в А", AiMessage{Text: "ответ", Refused: true})
	if err != nil {
		t.Fatal(err)
	}
	if ex.Chat.ID != a.ID || !ex.Chat.LastMessageAt.After(a.LastMessageAt) || !ex.Answer.Refused {
		t.Fatalf("обмен в чате А: %+v", ex)
	}
	if mine, _ := s.AiChats(ctx, kid); mine[0].ID != a.ID {
		t.Fatalf("чат с новой репликой — первый в списке: %+v", mine)
	}
}

func TestRenameAiChat_OnlyOwn(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, parent := aiFamily(t, s)
	a := startChat(t, s, kid, "Чат 21 сентября", "первый")

	if _, err := s.RenameAiChat(ctx, parent, a.ID, "Чужое"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("переименовать чужой чат нельзя: %v", err)
	}
	got, err := s.RenameAiChat(ctx, kid, a.ID, "Высшая проба")
	if err != nil || got.Title != "Высшая проба" || !got.LastMessageAt.Equal(a.LastMessageAt) {
		t.Fatalf("переименование не двигает чат в списке: %+v %v", got, err)
	}
	if again, _ := s.AiChat(ctx, kid, a.ID); again.Title != "Высшая проба" {
		t.Fatalf("название не сохранилось: %+v", again)
	}
}

// Контекст модели — последние N реплик чата, от старых к новым (F60).
func TestAiChatMessages_LastNOldestFirst(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, _ := aiFamily(t, s)
	a := startChat(t, s, kid, "Чат", "в1")
	for _, q := range []string{"в2", "в3", "в4", "в5", "в6"} {
		if _, err := s.SaveAiExchange(ctx, kid, a.ID, q, AiMessage{Text: "ответ на " + q}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.AiChatMessages(ctx, kid, a.ID, 10)
	if err != nil || len(got) != 10 {
		t.Fatalf("последние 10: %d %v", len(got), err)
	}
	if got[0].Text != "в2" || got[1].Text != "ответ на в2" || got[9].Text != "ответ на в6" {
		t.Fatalf("пять последних пар от старой к новой: %q … %q", got[0].Text, got[9].Text)
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
