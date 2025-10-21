package helper

import (
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

// SendChannelMessage sends a message to a specified channel
func SendChannelMessage(s *discordgo.Session, channelID string, message string, event eventWorker.Event) {
	discordAPIWorker.NewRequest(event, func() error {
		logger.Debug(logger.LogData{
			"trace_id": event.TraceID,
			"action":   "channel_message_sent",
			"channel":  channelID,
			"message":  message,
		})

		_, err := s.ChannelMessageSend(channelID, message)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": event.TraceID,
				"action":   "channel_message_failed",
				"channel":  channelID,
				"error":    err.Error(),
			})
		}
		return err
	})
}

// SendChannelEmbed sends an embed message to a specified channel
func SendChannelEmbed(s *discordgo.Session, channelID string, embed *discordgo.MessageEmbed, event eventWorker.Event) {
	discordAPIWorker.NewRequest(event, func() error {
		logger.Debug(logger.LogData{
			"trace_id": event.TraceID,
			"action":   "channel_embed_sent",
			"channel":  channelID,
		})

		_, err := s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": event.TraceID,
				"action":   "channel_embed_failed",
				"channel":  channelID,
				"error":    err.Error(),
			})
		}
		return err
	})
}
