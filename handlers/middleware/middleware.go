package middleware

import (
	"astralHRBot/channels"
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
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
