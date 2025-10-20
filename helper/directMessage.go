package helper

import (
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

// SendDirectMessage sends a direct message to a user
func SendDirectMessage(s *discordgo.Session, userID string, message string, event eventWorker.Event) {
	discordAPIWorker.NewRequest(event, func() error {
		dmChannel, err := s.UserChannelCreate(userID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": event.TraceID,
				"action":   "dm_channel_create_failed",
				"user_id":  userID,
				"error":    err.Error(),
			})
			return err
		}

		_, err = s.ChannelMessageSend(dmChannel.ID, message)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": event.TraceID,
				"action":   "dm_message_send_failed",
				"user_id":  userID,
				"error":    err.Error(),
			})
			return err
		}
		return nil
	})
}
