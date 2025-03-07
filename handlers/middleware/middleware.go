package middleware

import (
	"astralHRBot/channels"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func IgnoreBotMessages(discord *discordgo.Session, message *discordgo.MessageCreate) bool {
	return discord.State.User.ID == message.Author.ID
}

func SendMessageOnMemberJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) bool {
	channelID := channels.GetChannelID("landing-1529")
	message := fmt.Sprintf("%s Joined The Server", m.User.GlobalName)
	discordAPIWorker.NewRequest(func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})
	return true
}

func SendMessageOnMemberLeave(s *discordgo.Session, m *discordgo.GuildMemberRemove) bool {
	channelID := channels.GetChannelID("leavers-5053")
	message := fmt.Sprintf("%s Left The Server", m.User.GlobalName)
	discordAPIWorker.NewRequest(func() error {
		_, err := s.ChannelMessageSend(channelID, message)
		return err
	})
	return true
}
