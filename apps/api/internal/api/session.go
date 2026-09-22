package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/access"
	"github.com/ArthurBabkin/max-hackathon/packages/core/auth"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type sessionRequest struct {
	InitData   string  `json:"init_data"`
	StartParam *string `json:"start_param"`
}

type sessionResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Session   session   `json:"session"`
}

type session struct {
	User struct {
		ID        string `json:"id"`
		MaxUserID int64  `json:"max_user_id"`
		FirstName string `json:"first_name"`
	} `json:"user"`
	Member struct {
		ID              string  `json:"id"`
		Role            string  `json:"role"`
		IsCreator       bool    `json:"is_creator"`
		ReminderOffsets []int32 `json:"reminder_offsets"`
	} `json:"member"`
	Trajectory  trajectorySummary  `json:"trajectory"`
	Permissions access.Permissions `json:"permissions"`
	StartParam  *string            `json:"start_param"`
}

// createSession — POST /session (F12). Ничего не создаёт: траекторию и
// создателя определяет онбординг в боте, а не открытие мини-приложения.
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) error {
	var req sessionRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.InitData == "" {
		return badRequest("Нет строки запуска init_data.")
	}
	if req.StartParam != nil && len(*req.StartParam) > 512 {
		return badRequest("start_param длиннее 512 символов.")
	}
	data, err := s.policy.Authenticate(req.InitData, s.now())
	if err != nil {
		slog.Info("вход отклонён", "request_id", reqID(r.Context()), "reason", err.Error())
		return &Error{http.StatusUnauthorized, "UNAUTHORIZED",
			"Не удалось проверить вход. Откройте приложение из чата с ботом."}
	}
	if data.Unsigned {
		slog.Warn("вход по неподписанной строке (дев-обход)", "request_id", reqID(r.Context()),
			"max_user_id", data.User.ID)
	}

	ctx := r.Context()
	m, err := s.store.CurrentMember(ctx, data.User.ID)
	if errors.Is(err, store.ErrNotFound) {
		return notFound("Сначала пройдите онбординг в чате бота.")
	}
	if err != nil {
		return err
	}
	if name := strings.TrimSpace(data.User.FirstName); name != "" && name != m.FirstName {
		if _, err := s.store.UpsertUser(ctx, data.User.ID, name); err != nil {
			return err
		}
		m.FirstName = name
	}

	token, expires, err := auth.Issue(auth.Claims{
		UserID: m.UserID, MaxUserID: m.MaxUserID, MemberID: m.MemberID,
		TrajectoryID: m.TrajectoryID, Role: m.Role, IsCreator: m.IsCreator,
	}, []byte(s.cfg.JWTSecret), s.cfg.JWTTTL, s.now())
	if err != nil {
		return err
	}
	out, err := s.sessionFor(ctx, m)
	if err != nil {
		return err
	}
	out.StartParam = req.StartParam
	if out.StartParam == nil && data.StartParam != "" {
		sp := data.StartParam
		out.StartParam = &sp
	}
	writeJSON(w, http.StatusOK, sessionResponse{Token: token, ExpiresAt: expires.UTC(), Session: out})
	return nil
}

func (s *Server) sessionFor(ctx context.Context, m store.Member) (session, error) {
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return session{}, err
	}
	var out session
	out.User.ID, out.User.MaxUserID, out.User.FirstName = m.UserID, m.MaxUserID, m.FirstName
	out.Member.ID, out.Member.Role, out.Member.IsCreator = m.MemberID, m.Role, m.IsCreator
	out.Member.ReminderOffsets = m.ReminderOffsets
	if out.Member.ReminderOffsets == nil {
		out.Member.ReminderOffsets = []int32{}
	}
	out.Trajectory = summaryOf(t)
	out.Permissions = permissionsOf(m)
	return out, nil
}

func permissionsOf(m store.Member) access.Permissions {
	return access.For(access.Member{Role: access.Role(m.Role), IsCreator: m.IsCreator, HasKid: m.HasKid})
}

// --- Аутентификация остальных ручек ---------------------------------------

type sessionKey struct{}

// authed проверяет JWT и заново читает участие из базы. Если участника
// удалили или траекторию удалили, токен ещё жив, но ответ — 401: клиент
// повторит POST /session и получит 404 «пройдите онбординг» (F41, F50).
func (s *Server) authed(h handlerFunc) http.HandlerFunc {
	return s.handle(func(w http.ResponseWriter, r *http.Request) error {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			return errUnauthorized
		}
		claims, err := auth.Parse(raw, []byte(s.cfg.JWTSecret), s.now())
		if err != nil {
			return errUnauthorized
		}
		m, err := s.store.ActiveMember(r.Context(), claims.MemberID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && m.UserID != claims.UserID) {
			return errUnauthorized
		}
		if err != nil {
			return err
		}
		return h(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, m)))
	})
}

// me — участник текущего запроса; вызывать только внутри authed.
func me(r *http.Request) store.Member {
	return r.Context().Value(sessionKey{}).(store.Member)
}
