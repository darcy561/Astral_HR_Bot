package handlers

import (
	"astralHRBot/handlers/middleware"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

var messageCreateMiddleware = []MessageCreateMiddleware{
	middleware.IgnoreBotMessages,
}

func MessageHandlers(s *discordgo.Session, m *discordgo.MessageCreate) {
	for _, middleware := range messageCreateMiddleware {
		if middleware(s, m) {
			return
		}
	}
	eventWorker.AddEvent(eventWorker.Event{
		UserID:  m.Author.ID,
		Payload: m,
		Handler: func(payload interface{}) {
		},
	})

}
