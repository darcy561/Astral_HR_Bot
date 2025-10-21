package helper

import (
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

// AddRole adds a role to a guild member
func AddRole(s *discordgo.Session, guildID string, userID string, roleID string, event eventWorker.Event) {
	discordAPIWorker.NewRequest(event, func() error {
		logger.Debug(logger.LogData{
			"trace_id":  event.TraceID,
			"action":    "role_added",
			"member_id": userID,
			"role_id":   roleID,
		})

		err := s.GuildMemberRoleAdd(guildID, userID, roleID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id":  event.TraceID,
				"action":    "role_add_failed",
				"member_id": userID,
				"role_id":   roleID,
				"error":     err.Error(),
			})
		}
		return err
	})
}

// RemoveRole removes a role from a guild member
func RemoveRole(s *discordgo.Session, guildID string, userID string, roleID string, event eventWorker.Event) {
	discordAPIWorker.NewRequest(event, func() error {
		logger.Debug(logger.LogData{
			"trace_id":  event.TraceID,
			"action":    "role_removed",
			"member_id": userID,
			"role_id":   roleID,
		})

		err := s.GuildMemberRoleRemove(guildID, userID, roleID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id":  event.TraceID,
				"action":    "role_remove_failed",
				"member_id": userID,
				"role_id":   roleID,
				"error":     err.Error(),
			})
		}
		return err
	})
}

// AddRoles adds multiple roles to a guild member
func AddRoles(s *discordgo.Session, guildID string, userID string, roleIDs []string, event eventWorker.Event) {
	for _, roleID := range roleIDs {
		AddRole(s, guildID, userID, roleID, event)
	}
}

// RemoveRoles removes multiple roles from a guild member
func RemoveRoles(s *discordgo.Session, guildID string, userID string, roleIDs []string, event eventWorker.Event) {
	for _, roleID := range roleIDs {
		RemoveRole(s, guildID, userID, roleID, event)
	}
}
