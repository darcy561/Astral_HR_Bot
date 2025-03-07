package handlers

import (
	"astralHRBot/workers/eventWorker"
	"fmt"
	"slices"

	"github.com/bwmarrin/discordgo"
)

var guildMemberUpdateMiddleware = []GuildMemberUpdateMiddleware{}

func GuildMemberUpdateHandlers(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	for _, middleware := range guildMemberUpdateMiddleware {
		if !middleware(s, m) {
			return
		}
	}

	eventWorker.AddEvent(eventWorker.Event{
		UserID:  m.User.ID,
		Payload: m,
		Handler: func(payload interface{}) {
			handleRoleChanges(s, m)
		},
	})
}

func handleRoleChanges(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	oldRoles := []string{}
	newRoles := []string{}
	if m.BeforeUpdate != nil && m.BeforeUpdate.Roles != nil {
		oldRoles = m.BeforeUpdate.Roles
	} else {
		fmt.Println("No old roles to compare, assuming none existed.")
	}
	if m.Roles != nil {
		newRoles = m.Roles
	} else {
		fmt.Println("No new roles to compare, assuming none existed.")
	}

	addedRoles := []string{}
	for _, newRole := range newRoles {
		if !hasRole(oldRoles, newRole) {
			addedRoles = append(addedRoles, newRole)
		}
	}

	removedRoles := []string{}
	for _, oldRole := range oldRoles {
		if !hasRole(newRoles, oldRole) {
			removedRoles = append(removedRoles, oldRole)
		}
	}

	if len(addedRoles) > 0 {
		HandleRoleGained(s, m, addedRoles)
	}

	if len(removedRoles) > 0 {
		HandleRoleLost(s, m, removedRoles)
	}

}

func hasRole(roles []string, roleID string) bool {
	return slices.Contains(roles, roleID)
}
