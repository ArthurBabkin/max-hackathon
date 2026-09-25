package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// MarkUpdateSeen запоминает ключ обновления MAX. false — это повторная
// доставка, обрабатывать второй раз нельзя.
func (s *Store) MarkUpdateSeen(ctx context.Context, key string) (bool, error) {
	tag, err := s.db.Exec(ctx, `INSERT INTO bot_updates_seen (dedup_key) VALUES ($1) ON CONFLICT DO NOTHING`, key)
	if err != nil {
		return false, wrap(err)
	}
	return tag.RowsAffected() == 1, nil
}

// PurgeSeenUpdates удаляет старые ключи: MAX повторяет доставку в пределах
// 8 часов, дольше их хранить незачем.
func (s *Store) PurgeSeenUpdates(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM bot_updates_seen WHERE seen_at < $1`, before)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// Draft — ответы онбординга до создания траектории.
type Draft struct {
	Name         string   `json:"name,omitempty"`
	Grade        int      `json:"grade,omitempty"`
	RegionCode   string   `json:"region_code,omitempty"`
	SubjectCodes []string `json:"subject_codes,omitempty"`
	// DirectionIDs — выбранные направления в порядке выбора (F8).
	DirectionIDs []string `json:"direction_ids,omitempty"`
	// AllDirections — открыт полный список направлений, а не только
	// подходящие под предметы.
	AllDirections bool `json:"all_directions,omitempty"`
	// Target — где ученик хочет учиться: код субъекта или TargetAny.
	Target        string   `json:"target,omitempty"`
	UniversityIDs []string `json:"university_ids,omitempty"`
	// District — открытый федеральный округ в списке регионов: на шаге
	// региона или «Где хочешь учиться?».
	District int `json:"district,omitempty"`
	// Offered — вузы на кнопках шага вузов: предложенные по городу и
	// направлениям и найденные поиском «Другой вуз».
	Offered []string `json:"offered,omitempty"`

	// Онбординг v2 (docs/onboarding-v2/SPEC.md, 1.4).

	// HomeCity — город ученика, если известен из поиска или геолокации.
	HomeCity string `json:"home_city,omitempty"`
	// PlaceOptions — варианты на кнопках шагов region и target.
	PlaceOptions []PlaceOption `json:"place_options,omitempty"`
	// Letter — открытая буква алфавита на шаге region; "*" — открыт список букв.
	Letter string `json:"letter,omitempty"`
	// Interests — коды В1 «что нравится», до двух.
	Interests []string `json:"interests,omitempty"`
	// Work — код В2 «какая работа ближе».
	Work string `json:"work,omitempty"`
	// Suggested — направления, которые бот предложил по интересам.
	Suggested  []string `json:"suggested,omitempty"`
	Experience string   `json:"experience,omitempty"`
	// Places — выбранные места «Где учиться»; PlacesAny — «Не важно».
	Places    []Place `json:"places,omitempty"`
	PlacesAny bool    `json:"places_any,omitempty"`
	// GoalByKid — родитель выбрал «Пусть ребёнок ответит».
	GoalByKid bool `json:"goal_by_kid,omitempty"`
	// OfferPage — сколько раз нажали «Показать ещё» на шаге вузов.
	OfferPage int `json:"offer_page,omitempty"`
	// Found — места, найденные текстом на шаге target, когда совпадений
	// несколько: кнопки выбора под отдельным сообщением.
	Found []PlaceOption `json:"found,omitempty"`
	// VuzAnywhere — «Все вузы с этим направлением»: подборка вузов без
	// фильтра по местам.
	VuzAnywhere bool `json:"vuz_anywhere,omitempty"`
	// OfferMore — у подборки вузов есть следующая страница.
	OfferMore bool `json:"offer_more,omitempty"`
	// Nearby — ближайшие вузы с направлением, когда в выбранных местах
	// его нет (кнопки «+ вуз»).
	Nearby []string `json:"nearby,omitempty"`
	// Similar — похожие направления, которые есть в выбранных местах.
	Similar []string `json:"similar,omitempty"`
	// Joined — приглашённый ученик отвечает на вопросы об интересах за
	// родителя (goal_by_kid): траектория уже есть, «Готово» правит её.
	Joined bool `json:"joined,omitempty"`
	// EditMenu — на карточке профиля открыто меню «Что поменять?».
	EditMenu bool `json:"edit_menu,omitempty"`
	// EditStage — номер шага (как в счётчике «Шаг N из 9»), который правят
	// из карточки профиля: когда диалог уходит дальше него, бот возвращает
	// к профилю. 0 — не правят.
	EditStage int `json:"edit_stage,omitempty"`
}

// Place — место «Где учиться»: регион целиком (City пусто) или город в нём.
type Place struct {
	RegionCode string `json:"region_code"`
	City       string `json:"city,omitempty"`
}

// PlaceOption — вариант места на кнопке: найденный поиском или
// предложенный ботом.
type PlaceOption struct {
	Kind       string `json:"kind"` // region | city
	RegionCode string `json:"region_code"`
	City       string `json:"city,omitempty"`
	Label      string `json:"label"`
}

// Place — место, которое означает вариант.
func (o PlaceOption) Place() Place {
	if o.Kind == "city" {
		return Place{RegionCode: o.RegionCode, City: o.City}
	}
	return Place{RegionCode: o.RegionCode}
}

// TargetAny — «не важно, где учиться».
const TargetAny = "any"

// Dialog — состояние разговора с ботом.
type Dialog struct {
	UserID        string
	Step          string
	Role          string // "" пока роль не выбрана
	Draft         Draft
	InviteID      *string
	SourcePayload *string
	PromptMID     *string
}

// ErrStale — кнопка из старого сообщения: диалог уже на другом шаге.
var ErrStale = errors.New("store: шаг диалога уже пройден")

const dialogColumns = `user_id::text, step, COALESCE(role, ''), draft, invite_id::text, source_payload, prompt_mid`

func scanDialog(row rowScanner) (Dialog, error) {
	var d Dialog
	var raw []byte
	if err := row.Scan(&d.UserID, &d.Step, &d.Role, &raw, &d.InviteID, &d.SourcePayload, &d.PromptMID); err != nil {
		return d, wrap(err)
	}
	if err := json.Unmarshal(raw, &d.Draft); err != nil {
		return d, err
	}
	return d, nil
}

// StartDialog начинает разговор заново: /start сбрасывает черновик.
func (s *Store) StartDialog(ctx context.Context, d Dialog) (Dialog, error) {
	raw, err := json.Marshal(d.Draft)
	if err != nil {
		return Dialog{}, err
	}
	var role *string
	if d.Role != "" {
		role = &d.Role
	}
	return scanDialog(s.db.QueryRow(ctx, `
		INSERT INTO bot_dialogs (user_id, step, role, draft, invite_id, source_payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET step = EXCLUDED.step, role = EXCLUDED.role, draft = EXCLUDED.draft,
		    invite_id = EXCLUDED.invite_id, source_payload = EXCLUDED.source_payload, prompt_mid = NULL,
		    updated_at = now()
		RETURNING `+dialogColumns, d.UserID, d.Step, role, raw, d.InviteID, d.SourcePayload))
}

func (s *Store) Dialog(ctx context.Context, userID string) (Dialog, error) {
	return scanDialog(s.db.QueryRow(ctx, `SELECT `+dialogColumns+` FROM bot_dialogs WHERE user_id = $1`, userID))
}

// UpdateDialog — переход онбординга под блокировкой строки. fn проверяет
// шаг (ErrStale — ничего не менять), правит диалог и может сделать в той
// же транзакции что-то ещё, например создать траекторию. Два быстрых
// нажатия не проскочат один шаг дважды.
func (s *Store) UpdateDialog(ctx context.Context, userID string, fn func(tx *Store, d *Dialog) error) (Dialog, error) {
	var out Dialog
	err := s.Tx(ctx, func(tx *Store) error {
		d, err := scanDialog(tx.db.QueryRow(ctx, `SELECT `+dialogColumns+` FROM bot_dialogs WHERE user_id = $1 FOR UPDATE`, userID))
		if err != nil {
			return err
		}
		if err := fn(tx, &d); err != nil {
			return err
		}
		raw, err := json.Marshal(d.Draft)
		if err != nil {
			return err
		}
		var role *string
		if d.Role != "" {
			role = &d.Role
		}
		out, err = scanDialog(tx.db.QueryRow(ctx, `
			UPDATE bot_dialogs SET step = $2, role = $3, draft = $4, updated_at = now()
			WHERE user_id = $1 RETURNING `+dialogColumns, userID, d.Step, role, raw))
		return err
	})
	return out, err
}

// SetPromptMID запоминает сообщение с текущим вопросом: его клавиатуру бот
// правит на месте при мультивыборе.
func (s *Store) SetPromptMID(ctx context.Context, userID, mid string) error {
	_, err := s.db.Exec(ctx, `UPDATE bot_dialogs SET prompt_mid = $2 WHERE user_id = $1`, userID, mid)
	return wrap(err)
}

// ErrInviteInvalid — ссылка использована, отозвана или траектория удалена (F39).
var ErrInviteInvalid = errors.New("store: приглашение недействительно")

// ErrAlreadyMember — пользователь уже участник этой траектории.
var ErrAlreadyMember = errors.New("store: уже участник траектории")

// JoinByInvite расходует одноразовую ссылку и подключает пользователя
// (F39, F42). Двое по одной ссылке гонятся в базе: условный UPDATE достаётся
// одному. Если подключить нельзя (уже участник, ученик уже есть), ссылка
// не расходуется — откат всей транзакции.
func (s *Store) JoinByInvite(ctx context.Context, token, userID string) (Member, error) {
	var out Member
	err := s.Tx(ctx, func(tx *Store) error {
		var inviteID, tid, role string
		err := tx.db.QueryRow(ctx, `
			UPDATE invites SET used_at = now(), used_by_user_id = $2
			WHERE token = $1 AND used_at IS NULL AND revoked_at IS NULL
			  AND trajectory_id IN (SELECT id FROM trajectories WHERE deleted_at IS NULL)
			RETURNING id::text, trajectory_id::text, role`, token, userID).Scan(&inviteID, &tid, &role)
		if errors.Is(wrap(err), ErrNotFound) {
			return ErrInviteInvalid
		}
		if err != nil {
			return wrap(err)
		}
		mid, err := tx.addMember(ctx, tid, userID, role, false)
		switch {
		case IsConflictOn(err, "members_active_uniq"):
			return ErrAlreadyMember
		case IsConflictOn(err, "members_single_active_kid_uniq"):
			return ErrKidExists
		case err != nil:
			return err
		}
		if err := tx.Audit(ctx, mid, "join", "invite", inviteID); err != nil {
			return err
		}
		out, err = tx.ActiveMember(ctx, mid)
		return err
	})
	return out, err
}

// InviterName — кто создал приглашение, для приветствия F42.
func (s *Store) InviterName(ctx context.Context, token string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx, `
		SELECT u.first_name FROM invites i
		JOIN members m ON m.id = i.created_by_member_id JOIN users u ON u.id = m.user_id
		WHERE i.token = $1`, token).Scan(&name)
	return name, wrap(err)
}

// ActiveRecipients — активные участники траектории, кроме except (пустая
// строка — все): кому разослать уведомление.
func (s *Store) ActiveRecipients(ctx context.Context, trajectoryID, except string) ([]Recipient, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id::text, u.max_user_id, u.first_name, m.role
		FROM members m JOIN users u ON u.id = m.user_id
		JOIN trajectories t ON t.id = m.trajectory_id AND t.deleted_at IS NULL
		WHERE m.trajectory_id = $1 AND m.left_at IS NULL AND m.removed_at IS NULL AND m.id::text <> $2
		ORDER BY m.joined_at`, trajectoryID, except)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Recipient, error) {
		var x Recipient
		return x, r.Scan(&x.MemberID, &x.MaxUserID, &x.Name, &x.Role)
	})
}

// DeleteTrajectory — /delete создателем (F50): доступ закрывается сразу,
// напоминания и ссылки гаснут; строки физически удаляет воркер
// (PurgeDeletedTrajectories), не позже суток.
func (s *Store) DeleteTrajectory(ctx context.Context, trajectoryID, creatorMemberID string) error {
	return s.Tx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			UPDATE trajectories SET deleted_at = now()
			WHERE id = $1 AND deleted_at IS NULL
			  AND EXISTS (SELECT 1 FROM members WHERE id = $2 AND trajectory_id = $1 AND is_creator)`,
			trajectoryID, creatorMemberID)
		if err != nil {
			return wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE reminders SET status = 'cancelled'
			WHERE status = 'planned' AND tracker_item_id IN (SELECT id FROM tracker_items WHERE trajectory_id = $1)`,
			trajectoryID); err != nil {
			return wrap(err)
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE invites SET revoked_at = now() WHERE trajectory_id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
			trajectoryID); err != nil {
			return wrap(err)
		}
		return tx.Audit(ctx, creatorMemberID, "delete", "trajectory", trajectoryID)
	})
}

// PurgeDeletedTrajectories физически удаляет траектории, помеченные раньше
// before, вместе со всем, что на них ссылается (ON DELETE CASCADE).
func (s *Store) PurgeDeletedTrajectories(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM trajectories WHERE deleted_at IS NOT NULL AND deleted_at < $1`, before)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// ToggleReminderOffset включает или выключает порог в /settings (F51).
// Возвращает пороги после изменения, от дальнего к ближнему.
func (s *Store) ToggleReminderOffset(ctx context.Context, memberID string, offset int) ([]int32, error) {
	var out []int32
	err := s.db.QueryRow(ctx, `
		UPDATE members SET reminder_offsets = CASE
		    WHEN $2 = ANY(reminder_offsets) THEN array_remove(reminder_offsets, $2)
		    ELSE ARRAY(SELECT x FROM unnest(reminder_offsets || $2) AS x ORDER BY x DESC)
		END
		WHERE id = $1 RETURNING reminder_offsets`, memberID, offset).Scan(&out)
	return out, wrap(err)
}

// EnableAllReminders — «Напоминать о сроках» (F11): все четыре порога.
func (s *Store) EnableAllReminders(ctx context.Context, memberID string) error {
	_, err := s.db.Exec(ctx, `UPDATE members SET reminder_offsets = '{30,7,3,1}' WHERE id = $1`, memberID)
	return wrap(err)
}

// SetDirections — «Изменить цель» приглашённого (F42).
func (s *Store) SetDirections(ctx context.Context, trajectoryID, memberID string, directionIDs []string) error {
	if directionIDs == nil {
		directionIDs = []string{}
	}
	return s.UpdateTrajectory(ctx, trajectoryID, memberID, TrajectoryPatch{DirectionIDs: directionIDs})
}

// Кнопки в сообщениях бота несут id пункта трекера, предложения или
// напоминания. Действовать нужно от имени участия пользователя именно в
// той траектории, к которой относится объект: родитель двоих детей
// состоит в двух.
var trajectoryOf = map[string]string{
	"tracker_item": `SELECT trajectory_id FROM tracker_items WHERE id = $2`,
	"proposal":     `SELECT trajectory_id FROM proposals WHERE id = $2`,
	"reminder": `SELECT ti.trajectory_id FROM reminders r JOIN tracker_items ti ON ti.id = r.tracker_item_id
	             WHERE r.id = $2`,
}

// MemberFor — активное участие пользователя MAX в траектории объекта.
// ErrNotFound — объекта нет или пользователь в той траектории не состоит.
func (s *Store) MemberFor(ctx context.Context, maxUserID int64, entity, id string) (Member, error) {
	sub, ok := trajectoryOf[entity]
	if !ok {
		return Member{}, ErrNotFound
	}
	return scanMember(s.db.QueryRow(ctx, `
		SELECT `+memberColumns+`
		FROM users u
		JOIN members m ON m.user_id = u.id
		JOIN trajectories t ON t.id = m.trajectory_id
		WHERE u.max_user_id = $1 AND t.id = (`+sub+`) AND `+activeMember, maxUserID, id))
}

// Reminder — напоминание по id с данными для текста: для «напомнить завтра».
func (s *Store) Reminder(ctx context.Context, id string) (DueReminder, error) {
	return scanDue(s.db.QueryRow(ctx, dueSelect+` WHERE r.id = $1`, id))
}
