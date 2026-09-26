package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type memberDTO struct {
	memberBrief
	IsCreator bool      `json:"is_creator"`
	IsMe      bool      `json:"is_me"`
	CanRemove bool      `json:"can_remove"`
	Color     *string   `json:"color"`
	JoinedAt  time.Time `json:"joined_at"`
}

type inviteDTO struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	URL       string    `json:"url"`
	Role      string    `json:"role"`
	CanRevoke bool      `json:"can_revoke"`
	CreatedAt time.Time `json:"created_at"`
}

type familyResponse struct {
	Trajectory trajectorySummary `json:"trajectory"`
	Members    []memberDTO       `json:"members"`
	Invites    []inviteDTO       `json:"invites"`
}

// inviteOf — ссылка глазами участника m: отозвать её может автор или
// создатель траектории (ТЗ §16).
func (s *Server) inviteOf(i store.Invite, m store.Member) inviteDTO {
	return inviteDTO{ID: i.ID, Token: i.Token, URL: notify.InviteURL(s.cfg.MaxBotName, i.Token), Role: i.Role,
		CanRevoke: m.IsCreator || i.CreatedBy == m.MemberID, CreatedAt: i.CreatedAt.UTC()}
}

// family — GET /family (F38, F43).
func (s *Server) family(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	members, err := s.store.FamilyMembers(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	invites, err := s.store.ActiveInvites(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	canRemove := permissionsOf(m).RemoveMembers
	out := familyResponse{Trajectory: summaryOf(t), Members: make([]memberDTO, len(members)),
		Invites: make([]inviteDTO, len(invites))}
	for i, x := range members {
		isMe := x.ID == m.MemberID
		out.Members[i] = memberDTO{
			memberBrief: memberBrief{ID: x.ID, Name: x.Name, Role: x.Role}, IsCreator: x.IsCreator, IsMe: isMe,
			CanRemove: canRemove && !isMe && !x.IsCreator, JoinedAt: x.JoinedAt.UTC(),
		}
	}
	for i, inv := range invites {
		out.Invites[i] = s.inviteOf(inv, m)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// createInvite — POST /family/invites (F39, F40): роль выбирает тот, кто
// приглашает; при ученике в траектории — только parent (409).
func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).Invite {
		return forbidden("Приглашать участников нельзя.")
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if body.Role != "kid" && body.Role != "parent" {
		return badRequest("Роль приглашённого — kid или parent.")
	}
	token, err := store.NewInviteToken()
	if err != nil {
		return err
	}
	inv, err := s.store.CreateInvite(ctx, m.TrajectoryID, m.MemberID, body.Role, token)
	if errors.Is(err, store.ErrKidExists) {
		return conflict("Ученик в траектории уже есть — пригласить можно только родителя.")
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, s.inviteOf(inv, m))
	return nil
}

var errNoInvite = notFound("Такой ссылки нет: её уже использовали или отозвали.")

// revokeInvite — DELETE /family/invites/{id} (ТЗ §16): неиспользованную
// ссылку отзывает её автор или создатель траектории.
func (s *Server) revokeInvite(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		return errNoInvite
	}
	err := s.store.RevokeInvite(ctx, m.TrajectoryID, id, m.MemberID, m.IsCreator)
	if errors.Is(err, store.ErrNotFound) {
		return errNoInvite
	}
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

var errNoMember = notFound("Такого участника нет.")

// removeMember — DELETE /family/members/{id} (F41): только создатель и не себя.
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).RemoveMembers {
		return forbidden("Удалить участника может только создатель траектории.")
	}
	id := r.PathValue("id")
	if id == m.MemberID {
		return forbidden("Себя удалить нельзя.")
	}
	if !uuidRe.MatchString(id) {
		return errNoMember
	}
	removed, err := s.store.RemoveMember(ctx, m.TrajectoryID, id, m.MemberID)
	if errors.Is(err, store.ErrNotFound) {
		return errNoMember
	}
	if err != nil {
		return err
	}
	s.tell(r, func(ctx context.Context) { s.notify.MemberRemoved(ctx, m.TrajectoryID, removed, m) })
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// leave — POST /family/leave (F48): создатель выйти не может.
func (s *Server) leave(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).Leave {
		return forbidden("Создатель не может выйти из траектории — только удалить её командой /delete в чате бота.")
	}
	left, err := s.store.LeaveTrajectory(ctx, m.TrajectoryID, m.MemberID)
	if err != nil {
		return err
	}
	s.tell(r, func(ctx context.Context) { s.notify.Left(ctx, m.TrajectoryID, left) })
	w.WriteHeader(http.StatusNoContent)
	return nil
}
