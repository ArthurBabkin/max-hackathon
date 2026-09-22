package api

import (
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type trajectorySummary struct {
	ID            string  `json:"id"`
	StudentName   string  `json:"student_name"`
	Grade         int     `json:"grade"`
	RegionCode    string  `json:"region_code"`
	RegionName    string  `json:"region_name"`
	DirectionID   *string `json:"direction_id"`
	DirectionName *string `json:"direction_name"`
	GoalStatus    string  `json:"goal_status"`
	HasKid        bool    `json:"has_kid"`
	MembersCount  int     `json:"members_count"`
}

func summaryOf(t store.Trajectory) trajectorySummary {
	region := t.RegionCode
	if r, ok := refdata.ByCode(t.RegionCode); ok {
		region = r.Name
	}
	return trajectorySummary{
		ID: t.ID, StudentName: t.StudentName, Grade: t.Grade, RegionCode: t.RegionCode,
		RegionName: region, DirectionID: t.DirectionID, DirectionName: t.DirectionName,
		GoalStatus: t.GoalStatus, HasKid: t.HasKid, MembersCount: t.MembersCount,
	}
}
