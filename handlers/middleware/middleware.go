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
	channelID := channels.GetChannelID("landing-1529")
	message := fmt.Sprintf("%s Joined The Server.", m.User.GlobalName)

	discordAPIWorker.NewRequest(e, func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})

	logger.Debug(e.TraceID, "send message on member join middleware: Passed")

	return true
}

func SendMessageOnMemberLeave(s *discordgo.Session, m *discordgo.GuildMemberRemove, e eventWorker.Event) bool {
	channelID := channels.GetChannelID("leavers-5053")
	message := fmt.Sprintf("%s Left The Server.", m.User.GlobalName)

	discordAPIWorker.NewRequest(e, func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})

	logger.Debug(e.TraceID, "send message on member leave middleware: Passed")

	return true
}
