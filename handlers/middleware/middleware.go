package middleware

import "github.com/bwmarrin/discordgo"

func IgnoreBotMessages(discord *discordgo.Session, message *discordgo.MessageCreate) bool {
	return discord.State.User.ID == message.Author.ID
}
