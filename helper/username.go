package helper

import "github.com/bwmarrin/discordgo"

// GetDisplayName returns the best available display name for a user.
// It tries GlobalName first, then falls back to Username if GlobalName is empty.
// Note: For server-specific display names that include nicknames, use m.Member.DisplayName() instead.
func GetDisplayName(user *discordgo.User) string {
	// User objects don't have access to server nicknames, only Member objects do
	// So we can only check GlobalName and Username
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}
