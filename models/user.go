package models

import (
	"time"
)

type User struct {
	DiscordID             string
	CurrentDisplayName    string
	CurrentJoinDate       time.Time
	PreviousJoinDate      time.Time
	PreviousLeaveDate     time.Time
	DateJoinedRecruitment time.Time
	LastMessageDate       time.Time
	LastMessageID         string
	Monitored             bool
}
