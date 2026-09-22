package access

import "testing"

// Матрица ТЗ §3.1 целиком: пять типичных участников × десять флагов.
func TestPermissions_Matrix(t *testing.T) {
	cases := []struct {
		name string
		who  Member
		want Permissions
	}{
		{
			name: "ученик-создатель",
			who:  Member{Role: Kid, IsCreator: true, HasKid: true},
			want: Permissions{AddToTracker: true, RemoveFromTracker: true, ResolveProposals: true,
				ToggleRegistered: true, EditProfile: true, Invite: true, RemoveMembers: true,
				DeleteTrajectory: true},
		},
		{
			name: "приглашённый ученик",
			who:  Member{Role: Kid, HasKid: true},
			want: Permissions{AddToTracker: true, RemoveFromTracker: true, ResolveProposals: true,
				ToggleRegistered: true, EditProfile: true, Invite: true, Leave: true},
		},
		{
			name: "родитель при ученике в траектории",
			who:  Member{Role: Parent, HasKid: true},
			want: Permissions{Propose: true, ToggleRegistered: true, EditProfile: true,
				Invite: true, Leave: true},
		},
		{
			name: "родитель-создатель без ученика",
			who:  Member{Role: Parent, IsCreator: true},
			want: Permissions{AddToTracker: true, RemoveFromTracker: true, ToggleRegistered: true,
				EditProfile: true, Invite: true, RemoveMembers: true, DeleteTrajectory: true},
		},
		{
			name: "родитель-создатель, ученик подключился",
			who:  Member{Role: Parent, IsCreator: true, HasKid: true},
			want: Permissions{Propose: true, ToggleRegistered: true, EditProfile: true,
				Invite: true, RemoveMembers: true, DeleteTrajectory: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := For(tc.who); got != tc.want {
				t.Fatalf("\nполучили %+v\nожидали  %+v", got, tc.want)
			}
		})
	}
}
