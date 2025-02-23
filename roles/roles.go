package roles

import (
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var RoleMap = make(map[string]string)
var ContentNotificationRoles = []string{
	"mining", "industry", "pve", "pvp", "fw",
}

func BuildGuildRoles(s *discordgo.Session) error {

	if len(s.State.Guilds) == 0 {
		log.Fatalf("Bot is not connected to any guild.")
		return nil
	}

	guildID, exists := os.LookupEnv("GUILD_ID")
	if !exists {
		log.Fatalf("No Guild ID Provided")
	}

	roles, err := s.GuildRoles(guildID)
	if err != nil {
		log.Fatalf("Error fetching roles: %v", err)
		return err
	}

	for _, role := range roles {
		processedRoleName := strings.ToLower(role.Name)
		processedRoleName = strings.ReplaceAll(processedRoleName, " ", "_")

		RoleMap[processedRoleName] = role.ID
	}

	log.Println("Roles fetched and mapped successfully.")
	return nil

}

// Enter role by name, all lowercase, use _ instead of spaces
func GetRoleID(roleName string) string {
	return RoleMap[roleName]
}
