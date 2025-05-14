package models

type UserAnalytics struct {
	UserID       string
	Messages     int64
	VoiceJoins   int64
	Invites      int64
	TopChannelID string
}
