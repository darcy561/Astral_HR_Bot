package commands

import (
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/roles"
	"astralHRBot/workers/eventWorker"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// AssignPingRolesCommand handles the /assign-ping-roles slash command
func AssignPingRolesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "assign_ping_roles_command",
		"message": "AssignPingRoles command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the target user from the command options
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		RespondToInteraction(s, i, "❌ **Error:** No user specified. Please provide a user to assign ping roles to.", true)
		return
	}

	// Get the user option
	userOption := options[0]
	if userOption.Type != discordgo.ApplicationCommandOptionUser {
		RespondToInteraction(s, i, "❌ **Error:** Invalid user parameter.", true)
		return
	}

	targetUserID := userOption.UserValue(s).ID
	targetUser := userOption.UserValue(s)

	// Create a mock event for role operations
	event := eventWorker.Event{
		TraceID: fmt.Sprintf("assign_ping_roles_%s", targetUserID),
	}

	// Get all ping role IDs
	pingRoleIDs := roles.ContentNotificationRoles

	// Add all ping roles to the user
	helper.AddRoles(s, i.GuildID, targetUserID, pingRoleIDs, event)

	successMessage := fmt.Sprintf("✅ **Successfully assigned ping roles to %s",
		targetUser.Username)

	RespondToInteraction(s, i, successMessage, true)

	logger.Info(logger.LogData{
		"action":         "ping_roles_assigned",
		"message":        "Ping roles assigned to user",
		"target_user":    targetUser.Username,
		"target_user_id": targetUserID,
		"assigned_by":    i.Member.User.Username,
		"assigned_by_id": i.Member.User.ID,
		"roles_count":    len(pingRoleIDs),
	})
}

// GetAssignPingRolesCommandDefinition returns the assign-ping-roles command definition
func GetAssignPingRolesCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "assign-ping-roles",
		Description: "Assigns all ping roles (Mining, Industry, PvE, PvP, Faction Warfare) to the specified user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The user to assign ping roles to",
				Required:    true,
			},
		},
	}
}
