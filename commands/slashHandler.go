package commands

import (
	"astralHRBot/logger"

	"github.com/bwmarrin/discordgo"
)

// SlashCommandHandlers handles all slash command interactions
func SlashCommandHandlers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":     "slash_command_handler",
		"message":    "Received slash command",
		"user_id":    i.Member.User.ID,
		"command":    i.ApplicationCommandData().Name,
		"channel_id": i.ChannelID,
		"guild_id":   i.GuildID,
	})

	// Get the command name
	commandName := i.ApplicationCommandData().Name

	// Find the handler for this command
	handler, exists := commandHandlers[commandName]
	if !exists {
		logger.Error(logger.LogData{
			"action":  "slash_command_error",
			"message": "Unknown slash command",
			"command": commandName,
		})
		return
	}

	// Execute the command handler
	handler(s, i)
}
