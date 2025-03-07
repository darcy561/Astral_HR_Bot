package roles

import (
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var RoleMap = make(map[string]string)
var ContentNotificationRoles = []string{
	"mining-4737", "industry-8140", "pve-7472", "pvp-9618", "fw-8226",
}

func generateRoleKey(name, id string) string {
	processedName := strings.ToLower(name)
	processedName = strings.ReplaceAll(processedName, " ", "_")
	return processedName + "-" + id[len(id)-4:]
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
		AddRoleToMap(role)
	}

	log.Println("Roles fetched and mapped successfully.")
	return nil

}

// Enter role by name, all lowercase, use _ instead of spaces
func GetRoleID(roleName string) string {
	return RoleMap[roleName]
}

func RemoveRoleFromMap(roleID string) {
	for id := range RoleMap {
		if strings.Contains(id, "-"+roleID[len(roleID)-4:]) {
			delete(RoleMap, id)
			break
		}
	}

}

func AddRoleToMap(role *discordgo.Role) {
	key := generateRoleKey(role.Name, role.ID)
	RoleMap[key] = role.ID
}

func UpdateRole(role *discordgo.Role) {
	RemoveRoleFromMap(role.ID)
	AddRoleToMap(role)
}
