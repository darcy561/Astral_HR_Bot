package handlers

import (
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"
	"slices"

	"github.com/bwmarrin/discordgo"
)

var guildMemberUpdateMiddleware = []GuildMemberUpdateMiddleware{}

func GuildMemberUpdateHandlers(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	eventWorker.Submit(m.User.ID, handleRoleChanges, s, m)
}

func handleRoleChanges(e eventWorker.Event) {
	p, t := e.Payload, e.TraceID

	if len(p) < 2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "invalid_args",
			"message":  "handle role changes: invalid arguments",
		})
		return
	}

	s, ok1 := p[0].(*discordgo.Session)
	m, ok2 := p[1].(*discordgo.GuildMemberUpdate)

	if !ok1 || !ok2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "type_assertion_failed",
			"message":  "handle role changes: type assertion failed",
		})
		return
	}

	for _, middleware := range guildMemberUpdateMiddleware {
		if !middleware(s, m, e) {
			return
		}
	}

	oldRoles, newRoles := []string{}, []string{}

	if m.BeforeUpdate != nil && m.BeforeUpdate.Roles != nil {
		oldRoles = m.BeforeUpdate.Roles
	} else {
		logger.Warn(logger.LogData{
			"trace_id":  t,
			"action":    "no_old_roles",
			"member_id": m.User.ID,
			"message":   "No old roles to compare, assuming none existed.",
		})
	}
	if m.Roles != nil {
		newRoles = m.Roles
	} else {
		logger.Warn(logger.LogData{
			"trace_id":  t,
			"action":    "no_new_roles",
			"member_id": m.User.ID,
			"message":   "No new roles to compare, assuming none existed.",
		})
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
		HandleRoleGained(s, m, addedRoles, e)
	}

	if len(removedRoles) > 0 {
		HandleRoleLost(s, m, removedRoles, e)
	}
}

func hasRole(roles []string, roleID string) bool {
	return slices.Contains(roles, roleID)
}
