package middleware

import (
	"astralHRBot/channels"
	"astralHRBot/helper"
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
	userName := helper.GetDisplayName(m.User)
	message := fmt.Sprintf("%s Joined The Server.", userName)

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
	userName := helper.GetDisplayName(m.User)
	message := fmt.Sprintf("%s Left The Server.", userName)

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

func MonitorMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_message_create",
		"message":  "Received message create",
		"user_id":  m.Author.ID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(m)
	return true
}

func MonitorMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_message_update",
		"message":  "Received message update",
		"user_id":  m.Author.ID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(m)
	return true
}

func MonitorMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_message_delete",
		"message":  "Received message delete",
		"user_id":  m.Author.ID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(m)
	return true
}

func MonitorVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_voice_state_update",
		"message":  "Received voice state update",
		"user_id":  v.UserID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(v)
	return true
}

func MonitorInviteCreate(s *discordgo.Session, i *discordgo.InviteCreate, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_invite_create",
		"message":  "Received invite create",
		"user_id":  i.Inviter.ID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(i)
	return true
}

func MonitorMessageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_message_reaction_add",
		"message":  "Received message reaction add",
		"user_id":  r.UserID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(r)
	return true
}

func MonitorMessageReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove, e eventWorker.Event) bool {
	logger.Debug(logger.LogData{
		"action":   "monitor_message_reaction_remove",
		"message":  "Received message reaction remove",
		"user_id":  r.UserID,
		"trace_id": e.TraceID,
	})

	monitoring.SubmitEvent(r)
	return true
}
