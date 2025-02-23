package models

import (
	"time"
)

type User struct {
	DiscordID             int
	PreviousJoinDate      time.Time
	PreviousLeaveDate     time.Time
	CurrentJoinDate       time.Time
	DateJoinedRecruitment time.Time
}
