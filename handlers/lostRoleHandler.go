package handlers

import (
	"astralHRBot/roles"
	discordAPIWorker "astralHRBot/workers/discordAPI"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleLost(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string) {
	if memberLeavesCorporation(s, m, r) {
		return
	}
	if memberLosesBlueRole(s, m, r) {
		return
	}

}

func memberLeavesCorporation(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string) bool {
	if hasRole(r, roles.GetRoleID("member-1054")) {

		for _, role := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(func() error {
				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID(role))
				return err
			})
		}

		discordAPIWorker.NewRequest(func() error {
			err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID("absentee"))
			return err
		})

		discordAPIWorker.NewRequest(func() error {
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetRoleID("guest"))
			return err
		})

		return true
	}
	return false
}

func memberLosesBlueRole(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string) bool {
	if hasRole(r, roles.GetRoleID("blue-2602")) {
		discordAPIWorker.NewRequest(func() error {
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetRoleID("guest"))
			return err
		})
		return true
	}
	return false
}
