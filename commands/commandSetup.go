package commands

import (
	"astralHRBot/logger"

	"github.com/bwmarrin/discordgo"
)

// Local command registry to avoid import cycles
var commandHandlers = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))

// Command definitions with their handlers
var commandDefinitions = []struct {
	definition *discordgo.ApplicationCommand
	handler    func(s *discordgo.Session, i *discordgo.InteractionCreate)
}{
	{GetToggleDebugCommandDefinition(), ToggleDebugCommand},
	{GetSetRecruitmentCleanupDelayCommandDefinition(), SetRecruitmentCleanupDelayCommand},
	{GetSetNewRecruitTrackingDaysCommandDefinition(), SetNewRecruitTrackingDaysCommand},
	{GetUserStatusCommandDefinition(), UserStatusCommand},
	{GetRebuildUserEventsCommandDefinition(), RebuildUserEventsCommand},
	{GetRebuildAllUserEventsCommandDefinition(), RebuildAllUserEventsCommand},
	{GetMonitoringStatusCommandDefinition(), MonitoringStatusCommand},
	{GetRebuildNewRecruitScenariosCommandDefinition(), RebuildNewRecruitScenariosCommand},
	{GetRebuildRecruitmentProcessScenariosCommandDefinition(), RebuildRecruitmentProcessScenariosCommand},
	{GetRebuildAnalyticsCommandDefinition(), RebuildAnalyticsCommand},
	{GetAssignPingRolesCommandDefinition(), AssignPingRolesCommand},
	// Add more commands here as you create them
	// {GetAnotherCommandDefinition(), AnotherCommand},
}

// RegisterAllSlashCommands registers all slash commands with the bot
func RegisterAllSlashCommands() {
	// Auto-register all command handlers
	for _, cmd := range commandDefinitions {
		commandHandlers[cmd.definition.Name] = cmd.handler
	}

	logger.Info(logger.LogData{
		"action":  "slash_command_setup_complete",
		"message": "All slash command handlers registered",
	})
}

// GetCommandHandler returns the handler for a given command name
func GetCommandHandler(commandName string) (func(s *discordgo.Session, i *discordgo.InteractionCreate), bool) {
	handler, exists := commandHandlers[commandName]
	return handler, exists
}

// RegisterSlashCommandsWithDiscord registers slash commands with Discord
func RegisterSlashCommandsWithDiscord(session *discordgo.Session) error {
	// Create command manager
	manager := NewCommandManager(session)

	// Add all command definitions automatically
	for _, cmd := range commandDefinitions {
		manager.AddCommand(cmd.definition)
	}

	// Register commands globally (to all guilds)
	logger.Info(logger.LogData{
		"action":  "slash_command_registration",
		"message": "Registering commands globally to all guilds",
	})
	return manager.RegisterGlobalCommands()
}
