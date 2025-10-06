package handlers

import (
	"astralHRBot/handlers/middleware"
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

var messageCreateMiddleware = []MessageCreateMiddleware{
	middleware.MonitorMessageCreate,
	middleware.IgnoreBotMessages,
}

func MessageHandlers(s *discordgo.Session, m *discordgo.MessageCreate) {
	logger.Debug(logger.LogData{
		"action":     "message_handler",
		"message":    "Received message",
		"user_id":    m.Author.ID,
		"channel_id": m.ChannelID,
	})

	eventWorker.Submit(
		m.Author.ID,
		func(e eventWorker.Event) {
			p, t := e.Payload, e.TraceID

			if len(p) < 2 {
				logger.Error(logger.LogData{
					"trace_id": t,
					"action":   "invalid_args",
					"message":  "handle message: invalid arguments",
				})
				return
			}

			s, ok1 := p[0].(*discordgo.Session)
			m, ok2 := p[1].(*discordgo.MessageCreate)

			if !ok1 || !ok2 {
				logger.Error(logger.LogData{
					"trace_id": t,
					"action":   "type_assertion_failed",
					"message":  "handle message: type assertion failed",
				})
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
