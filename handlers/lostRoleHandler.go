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
	if hasRole(r, roles.GetMemberRoleID()) {
		for _, roleID := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "role_removed",
					"member_id": m.User.ID,
					"role_id":   roleID,
				})

				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roleID)
				return err
			})
		}

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "role_removed",
				"member_id": m.User.ID,
				"role":      "absentee",
			})
			err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetAbsenteeRoleID())
			return err
		})

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "role_added",
				"member_id": m.User.ID,
				"role":      "guest",
			})
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetGuestRoleID())
			return err
		})

		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "member_leave_corporation",
			"member_id": m.User.ID,
			"message":   "Member Leaves Corporation Complete",
		})

		return true
	}
	return false
}

func memberLosesBlueRole(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) bool {
	if hasRole(r, roles.GetBlueRoleID()) {
		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "role_added",
				"member_id": m.User.ID,
				"role":      "guest",
			})
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetGuestRoleID())
			return err
		})

		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "member_lose_blue",
			"member_id": m.User.ID,
			"message":   "Member Loses Blue Role Complete",
		})

		return true
	}
	return false
}
