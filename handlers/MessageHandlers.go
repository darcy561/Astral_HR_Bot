package handlers

import (
	"astralHRBot/handlers/middleware"

	"github.com/bwmarrin/discordgo"
)

var messageCreateMiddleware = []MessageCreateMiddleware{
	middleware.IgnoreBotMessages,
}

func MessageHandlers(discord *discordgo.Session, message *discordgo.MessageCreate) {
	for _, middleware := range messageCreateMiddleware {
		if middleware(discord, message) {
			return
		}
	}

}
