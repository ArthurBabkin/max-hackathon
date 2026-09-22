package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func TestProposals_CreateResolve(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	parent := addMember(t, s, f.trajectoryID, uid, "parent", false)

	id, created, err := s.CreateProposal(ctx, f.trajectoryID, "p669-8-informatika", parent)
	if err != nil || !created {
		t.Fatalf("создание: %v %v", created, err)
	}
	again, created, err := s.CreateProposal(ctx, f.trajectoryID, "p669-8-informatika", parent)
	if err != nil || created || again != id {
		t.Fatalf("второе, пока первое ждёт, — то же предложение: %q %v %v", again, created, err)
	}
	if _, _, err := s.CreateProposal(ctx, f.trajectoryID, "nope", parent); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный профиль: %v", err)
	}

	item, err := s.ResolveProposal(ctx, f.trajectoryID, id, f.creatorMember, true)
	if err != nil || item == "" {
		t.Fatalf("принятие: %q %v", item, err)
	}
	row, _ := s.TrackerItem(ctx, f.trajectoryID, item)
	if row.AddedBy == nil || row.AddedBy.Name != "Ольга" {
		t.Fatalf("автор пункта — тот, кто предложил: %+v", row.AddedBy)
	}
	p, _ := s.Proposal(ctx, f.trajectoryID, id)
	if p.Status != "accepted" || p.ResolvedAt == nil {
		t.Fatalf("предложение закрыто: %+v", p)
	}
	if _, err := s.ResolveProposal(ctx, f.trajectoryID, id, f.creatorMember, false); !errors.Is(err, ErrProposalClosed) {
		t.Fatalf("повторный ответ — ErrProposalClosed: %v", err)
	}
	if _, err := s.ResolveProposal(ctx, f.trajectoryID, "00000000-0000-4000-8000-000000000999", f.creatorMember, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("нет предложения — ErrNotFound: %v", err)
	}
	if _, _, err := s.CreateProposal(ctx, f.trajectoryID, "p669-8-informatika", parent); !errors.Is(err, ErrAlreadyInTracker) {
		t.Fatalf("уже в трекере: %v", err)
	}

	// После отказа ту же олимпиаду можно предложить снова.
	id2, _, _ := s.CreateProposal(ctx, f.trajectoryID, "vsosh-informatika", parent)
	if item, err := s.ResolveProposal(ctx, f.trajectoryID, id2, f.creatorMember, false); err != nil || item != "" {
		t.Fatalf("отказ: %q %v", item, err)
	}
	if _, created, err := s.CreateProposal(ctx, f.trajectoryID, "vsosh-informatika", parent); err != nil || !created {
		t.Fatalf("после отказа — новое предложение: %v %v", created, err)
	}
}
