package handlers

import (
	"astralHRBot/logger"
	"astralHRBot/roles"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleLost(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) {
	if memberLeavesCorporation(s, m, r, e) {
		return
	}
	if memberLosesBlueRole(s, m, r, e) {
		return
	}

}

func memberLeavesCorporation(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) bool {
	if hasRole(r, roles.GetRoleID("member-1054")) {

		for _, role := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID, "role removed: "+role)

				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID(role))
				return err
			})
		}

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(e.TraceID, "role removed: absentee")
			err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID("absentee"))
			return err
		})

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(e.TraceID, "role added: guest")
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetRoleID("guest"))
			return err
		})

		logger.Debug(e.TraceID, "Member Leaves Corporation Complete")

		return true
	}
	return false
}

func memberLosesBlueRole(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) bool {
	if hasRole(r, roles.GetRoleID("blue-2602")) {
		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(e.TraceID, "role added: guest ")
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetRoleID("guest"))
			return err
		})

		logger.Debug(e.TraceID, "Member Loses Blue Role Complete")

		return true
	}
	return false
}
