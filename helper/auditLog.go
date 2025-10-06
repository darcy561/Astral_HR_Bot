package helper

import (
	"github.com/bwmarrin/discordgo"
)

// WasAuditActionInitiatedByBot checks if a specific audit log action for a user was initiated by the bot
// Returns true if the bot initiated the change, false otherwise
func WasAuditActionInitiatedByBot(s *discordgo.Session, userID string, actionType discordgo.AuditLogAction) bool {
	// Get the guild ID from the session state (assuming single guild bot)
	if len(s.State.Guilds) == 0 {
		return false
	}
	guildID := s.State.Guilds[0].ID

	// Check Discord Audit Log to see who initiated the action
	auditLog, err := s.GuildAuditLog(guildID, "", "", int(actionType), 10)
	if err != nil || len(auditLog.AuditLogEntries) == 0 {
		return false
	}

	// Look for the most recent entry that matches this specific user
	for _, entry := range auditLog.AuditLogEntries {
		if entry.TargetID == userID {
			// Check if the bot initiated this change
			return entry.UserID == s.State.User.ID
		}
	}

	return false
}

// WasRoleChangeInitiatedByBot is a convenience function for role updates
// Returns true if the bot initiated the role change, false otherwise
func WasRoleChangeInitiatedByBot(s *discordgo.Session, userID string) bool {
	return WasAuditActionInitiatedByBot(s, userID, discordgo.AuditLogActionMemberRoleUpdate)
}
