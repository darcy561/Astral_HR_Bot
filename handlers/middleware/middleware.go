package middleware

import (
	"astralHRBot/channels"
	"astralHRBot/logger"
	"astralHRBot/users"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func IgnoreBotMessages(discord *discordgo.Session, message *discordgo.MessageCreate, e eventWorker.Event) bool {
	return discord.State.User.ID == message.Author.ID
}

func SendMessageOnMemberJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd, e eventWorker.Event) bool {
	channelID := channels.GetLandingChannel()
	message := fmt.Sprintf("%s Joined The Server.", m.User.GlobalName)

	discordAPIWorker.NewRequest(e, func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})

	logger.Debug(logger.LogData{
		"trace_id":   e.TraceID,
		"action":     "middleware_pass",
		"middleware": "send_message_on_member_join",
		"member_id":  m.User.ID,
		"message":    "Passed",
	})

	return true
}

func SendMessageOnMemberLeave(s *discordgo.Session, m *discordgo.GuildMemberRemove, e eventWorker.Event) bool {
	channelID := channels.GetLeaversChannel()
	message := fmt.Sprintf("%s Left The Server.", m.User.GlobalName)

	discordAPIWorker.NewRequest(e, func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})

	logger.Debug(logger.LogData{
		"trace_id":   e.TraceID,
		"action":     "middleware_pass",
		"middleware": "send_message_on_member_leave",
		"member_id":  m.User.ID,
		"message":    "Passed",
	})

	return true
}

// CreateOrUpdateUserMiddleware sends an event to handle user creation/updates in Redis when a member joins
func CreateOrUpdateUserMiddleware(s *discordgo.Session, m *discordgo.GuildMemberAdd, e eventWorker.Event) bool {
	// Send the user creation event to the event worker

	eventWorker.Submit(m.User.ID, users.CreateOrUpdateUser, m.User)

	logger.Debug(logger.LogData{
		"trace_id":   e.TraceID,
		"action":     "middleware_pass",
		"middleware": "create_or_update_user",
		"member_id":  m.User.ID,
		"message":    "Passed",
	})

	return true
}

func MonitorUserActivity(s *discordgo.Session, m *discordgo.MessageCreate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":     "monitor_user_activity",
		"message":    "Received message event",
		"user_id":    m.Author.ID,
		"channel_id": m.ChannelID,
		"trace_id":   e.TraceID,
	})

	monitoring.SubmitEvent(m)
	return true
}

func MonitorVoiceJoin(s *discordgo.Session, v *discordgo.VoiceStateUpdate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":     "monitor_voice_join",
		"message":    "Received voice state update",
		"user_id":    v.UserID,
		"channel_id": v.ChannelID,
		"trace_id":   e.TraceID,
	})

	monitoring.SubmitEvent(v)
	return true
}

func MonitorInviteCreate(s *discordgo.Session, i *discordgo.InviteCreate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":     "monitor_invite_create",
		"message":    "Received invite create",
		"user_id":    i.Inviter.ID,
		"channel_id": i.ChannelID,
		"trace_id":   e.TraceID,
	})

	monitoring.SubmitEvent(i)
	return true
}
