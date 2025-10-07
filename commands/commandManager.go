package commands

import (
	"astralHRBot/logger"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// CommandManager manages slash command registration with Discord API
type CommandManager struct {
	commands []*discordgo.ApplicationCommand
	session  *discordgo.Session
}

// NewCommandManager creates a new command manager
func NewCommandManager(session *discordgo.Session) *CommandManager {
	return &CommandManager{
		commands: make([]*discordgo.ApplicationCommand, 0),
		session:  session,
	}
}

// AddCommand adds a command to the manager
func (r *CommandManager) AddCommand(cmd *discordgo.ApplicationCommand) {
	r.commands = append(r.commands, cmd)
}

// RegisterAllCommands registers all commands with Discord
func (r *CommandManager) RegisterAllCommands(guildID string) error {
	logger.Info(logger.LogData{
		"action":   "slash_command_registration",
		"message":  "Registering slash commands",
		"guild_id": guildID,
		"count":    len(r.commands),
	})

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register each command
	for _, cmd := range r.commands {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout while registering commands")
		default:
			// Register the command
			registeredCmd, err := r.session.ApplicationCommandCreate(r.session.State.User.ID, guildID, cmd)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "slash_command_registration_error",
					"message": "Failed to register command",
					"command": cmd.Name,
					"error":   err.Error(),
				})
				return err
			}

			logger.Info(logger.LogData{
				"action":  "slash_command_registered",
				"message": "Successfully registered command",
				"command": registeredCmd.Name,
				"id":      registeredCmd.ID,
			})
		}
	}

	logger.Info(logger.LogData{
		"action":  "slash_command_registration_complete",
		"message": "All slash commands registered successfully",
		"count":   len(r.commands),
	})

	return nil
}

// RegisterGlobalCommands registers commands globally (available in all guilds)
func (r *CommandManager) RegisterGlobalCommands() error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register each command globally
	for _, cmd := range r.commands {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout while registering global commands")
		default:
			// Register the command globally
			_, err := r.session.ApplicationCommandCreate(r.session.State.User.ID, "", cmd)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "global_slash_command_registration_error",
					"message": "Failed to register global command",
					"command": cmd.Name,
					"error":   err.Error(),
				})
				return err
			}
		}
	}

	logger.Info(logger.LogData{
		"action":  "global_slash_command_registration_complete",
		"message": "All global slash commands registered successfully",
		"count":   len(r.commands),
	})

	return nil
}

// UnregisterAllCommands removes all registered commands
func (r *CommandManager) UnregisterAllCommands(guildID string) error {
	logger.Info(logger.LogData{
		"action":   "slash_command_unregistration",
		"message":  "Unregistering slash commands",
		"guild_id": guildID,
	})

	// Get all registered commands
	commands, err := r.session.ApplicationCommands(r.session.State.User.ID, guildID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "slash_command_unregistration_error",
			"message": "Failed to get registered commands",
			"error":   err.Error(),
		})
		return err
	}

	// Delete each command
	for _, cmd := range commands {
		err := r.session.ApplicationCommandDelete(r.session.State.User.ID, guildID, cmd.ID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "slash_command_unregistration_error",
				"message": "Failed to unregister command",
				"command": cmd.Name,
				"error":   err.Error(),
			})
		} else {
			logger.Info(logger.LogData{
				"action":  "slash_command_unregistered",
				"message": "Successfully unregistered command",
				"command": cmd.Name,
			})
		}
	}

	logger.Info(logger.LogData{
		"action":  "slash_command_unregistration_complete",
		"message": "All slash commands unregistered successfully",
	})

	return nil
}
