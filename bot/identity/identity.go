package identity

import (
	"astralHRBot/logger"

	"github.com/bwmarrin/discordgo"
)

var BotID string

// GetBotID returns the bot's ID
func GetBotID() string {
	return BotID
}

// SetupBotIdentity sets up the bot's identity
func SetupBotIdentity(discord *discordgo.Session) error {
	// Get bot's own user information
	user, err := discord.User("@me")
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "server_startup",
			"message": "Error getting bot user information",
			"error":   err.Error(),
		})
		return err
	}
	BotID = user.ID

	logger.Info(logger.LogData{
		"action":  "server_startup",
		"message": "Bot identity established",
		"bot_id":  BotID,
	})

	return nil
}
