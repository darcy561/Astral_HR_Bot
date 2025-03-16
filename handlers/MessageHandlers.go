package handlers

import (
	"astralHRBot/handlers/middleware"
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

var messageCreateMiddleware = []MessageCreateMiddleware{
	middleware.IgnoreBotMessages,
}

func MessageHandlers(s *discordgo.Session, m *discordgo.MessageCreate) {

	eventWorker.AddEvent(
		m.Author.ID,
		func(e eventWorker.Event) {
			p, t := e.Payload, e.TraceID

			if len(p) < 2 {
				logger.Error(t, "handle role changes: invalid arguments")
				return
			}

			s, ok1 := p[0].(*discordgo.Session)
			m, ok2 := p[1].(*discordgo.MessageCreate)

			if !ok1 || !ok2 {
				logger.Error(t, "handle role changes: type assertion failed")
				return
			}

			for _, middleware := range messageCreateMiddleware {
				if !middleware(s, m, e) {
					return
				}
			}

		},
		s, m,
	)

}
