package helper

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
)

// GetGuildIDFromSession safely retrieves the guild ID from a Discord session,
// preferring environment variable over state
// Returns the guild ID and an error if it cannot be determined
func GetGuildIDFromSession(s *discordgo.Session) (string, error) {
	// First try to get from environment variable
	if guildID := os.Getenv("GUILD_ID"); guildID != "" {
		return guildID, nil
	}

	// Fall back to Discord state, but check bounds first
	if s == nil {
		return "", fmt.Errorf("Discord session is nil")
	}

	if len(s.State.Guilds) == 0 {
		return "", fmt.Errorf("no guilds available in Discord state")
	}

	return s.State.Guilds[0].ID, nil
}

// WasAuditActionInitiatedByBot checks if a specific audit log action for a user was initiated by the bot
// Returns true if the bot initiated the change, false otherwise
func WasAuditActionInitiatedByBot(s *discordgo.Session, userID string, actionType discordgo.AuditLogAction) bool {
	// Get the guild ID using the helper function
	guildID, err := GetGuildIDFromSession(s)
	if err != nil {
		return false
	}

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
