package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func TestInvites_KidRoleOnlyWithoutKid(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000002, "parent")

	kidInv, err := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "kid", "kidInviteToken01")
	if err != nil || kidInv.Role != "kid" {
		t.Fatalf("без ученика роль kid доступна: %+v %v", kidInv, err)
	}
	uid, _ := s.UpsertUser(ctx, 900000001, "Артём")
	addMember(t, s, f.trajectoryID, uid, "kid", false)
	if _, err := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "kid", "kidInviteToken02"); !errors.Is(err, ErrKidExists) {
		t.Fatalf("при ученике — ErrKidExists: %v", err)
	}
	if _, err := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "parent", "kidInviteToken01"); !IsConflictOn(err, "invites_token_key") {
		t.Fatalf("токен уникален: %v", err)
	}
	invs, _ := s.ActiveInvites(ctx, f.trajectoryID)
	if len(invs) != 1 || invs[0].Token != "kidInviteToken01" {
		t.Fatalf("активные ссылки: %+v", invs)
	}
}

func TestRevokeInvite_AuthorOrCreator(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	olga := addMember(t, s, f.trajectoryID, uid, "parent", false)
	creators, _ := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "parent", "creatorInvite01")
	olgas, _ := s.CreateInvite(ctx, f.trajectoryID, olga, "parent", "olgaInviteTok01")
	if olgas.CreatedBy != olga {
		t.Fatalf("автор ссылки: %+v", olgas)
	}

	if err := s.RevokeInvite(ctx, f.trajectoryID, creators.ID, olga, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужую ссылку не автор и не создатель не отзывает: %v", err)
	}
	if err := s.RevokeInvite(ctx, f.trajectoryID, olgas.ID, olga, false); err != nil {
		t.Fatalf("автор отзывает свою: %v", err)
	}
	if err := s.RevokeInvite(ctx, f.trajectoryID, olgas.ID, f.creatorMember, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("отозванную второй раз — ErrNotFound: %v", err)
	}
	if err := s.RevokeInvite(ctx, f.trajectoryID, creators.ID, f.creatorMember, true); err != nil {
		t.Fatalf("создатель отзывает: %v", err)
	}
	if invs, _ := s.ActiveInvites(ctx, f.trajectoryID); len(invs) != 0 {
		t.Fatalf("отозванные не активны: %+v", invs)
	}
}

func TestRemoveAndLeave(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	olga := addMember(t, s, f.trajectoryID, uid, "parent", false)
	uid2, _ := s.UpsertUser(ctx, 900000003, "Игорь")
	igor := addMember(t, s, f.trajectoryID, uid2, "parent", false)
	if _, err := s.CreateInvite(ctx, f.trajectoryID, olga, "parent", "olgaInviteTok01"); err != nil {
		t.Fatal(err)
	}

	members, _ := s.FamilyMembers(ctx, f.trajectoryID)
	if len(members) != 3 || !members[0].IsCreator || members[1].Name != "Ольга" {
		t.Fatalf("создатель первым, дальше по времени входа: %+v", members)
	}

	if _, err := s.RemoveMember(ctx, f.trajectoryID, f.creatorMember, f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("создателя удалить нельзя: %v", err)
	}
	removed, err := s.RemoveMember(ctx, f.trajectoryID, olga, f.creatorMember)
	if err != nil || removed.Name != "Ольга" || removed.MaxUserID != 900000002 {
		t.Fatalf("удаление: %+v %v", removed, err)
	}
	if invs, _ := s.ActiveInvites(ctx, f.trajectoryID); len(invs) != 0 {
		t.Fatalf("ссылки удалённого отозваны: %+v", invs)
	}
	if _, err := s.RemoveMember(ctx, f.trajectoryID, olga, f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("второе удаление — ErrNotFound: %v", err)
	}
	if _, err := s.ActiveMember(ctx, olga); !errors.Is(err, ErrNotFound) {
		t.Fatalf("удалённый теряет доступ: %v", err)
	}

	if _, err := s.LeaveTrajectory(ctx, f.trajectoryID, f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("создатель выйти не может: %v", err)
	}
	if _, err := s.LeaveTrajectory(ctx, f.trajectoryID, igor); err != nil {
		t.Fatal(err)
	}
	if members, _ := s.FamilyMembers(ctx, f.trajectoryID); len(members) != 1 {
		t.Fatalf("остался создатель: %+v", members)
	}
}
