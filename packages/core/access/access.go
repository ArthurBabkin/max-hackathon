// Package access — матрица прав ТЗ §3.1. Права не хранятся и не кладутся в
// токен: они зависят от того, есть ли в траектории ученик, а это меняется,
// когда кто-то подключается или удаляется. Поэтому считаются на каждый запрос.
package access

type Role string

const (
	Kid    Role = "kid"
	Parent Role = "parent"
)

// Member — всё, от чего зависят права участника.
type Member struct {
	Role      Role
	IsCreator bool
	// HasKid — в траектории есть активный участник с ролью ученика.
	HasKid bool
}

// Permissions повторяет схему Permissions контракта поле в поле.
type Permissions struct {
	AddToTracker      bool `json:"add_to_tracker"`
	RemoveFromTracker bool `json:"remove_from_tracker"`
	Propose           bool `json:"propose"`
	ResolveProposals  bool `json:"resolve_proposals"`
	ToggleRegistered  bool `json:"toggle_registered"`
	EditProfile       bool `json:"edit_profile"`
	Invite            bool `json:"invite"`
	RemoveMembers     bool `json:"remove_members"`
	Leave             bool `json:"leave"`
	DeleteTrajectory  bool `json:"delete_trajectory"`
}

func For(m Member) Permissions {
	kid := m.Role == Kid
	// Родитель решает за ученика, только пока ученика в траектории нет (ТЗ §3.2).
	ownsTracker := kid || !m.HasKid
	return Permissions{
		AddToTracker:      ownsTracker,
		RemoveFromTracker: ownsTracker,
		Propose:           !kid && m.HasKid,
		ResolveProposals:  kid,
		ToggleRegistered:  true,
		EditProfile:       true,
		Invite:            true,
		RemoveMembers:     m.IsCreator,
		Leave:             !m.IsCreator,
		DeleteTrajectory:  m.IsCreator,
	}
}
