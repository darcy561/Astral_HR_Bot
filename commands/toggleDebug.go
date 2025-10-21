package commands

import (
	"astralHRBot/globals"
	"astralHRBot/logger"

	"github.com/bwmarrin/discordgo"
)

// ToggleDebugCommand handles the /toggledebug slash command
func ToggleDebugCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "toggle_debug_command",
		"message": "ToggleDebug command executed",
		"user_id": i.Member.User.ID,
	})

	currentMode := globals.GetDebugMode()
	globals.SetDebugMode(!currentMode)

	content := ""
	if !currentMode {
		content = "Debug mode is now enabled"
	} else {
		content = "Debug mode is now disabled"
	}

	RespondToInteraction(s, i, content, true)
}

// GetToggleDebugCommandDefinition returns the toggledebug command definition
func GetToggleDebugCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "toggle-debug-mode",
		Description: "Toggle debug mode for the bot",
	}
}
