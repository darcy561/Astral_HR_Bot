package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	"astralHRBot/users"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleLost(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) {
	if memberLeavesCorporation(s, m, r, e) {
		return
	}
	if memberLosesBlueRole(s, m, r, e) {
		return
	}
	if memberLosesRecruitRole(s, m, r, e) {
		return
	}
}

func memberLeavesCorporation(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) bool {
	if roles.HasRole(r, roles.GetMemberRoleID()) {
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

		monitoring.RemoveAllScenarios(m.User.ID)

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

		discordAPIWorker.NewRequest(e, func() error {

			_, err := s.ChannelMessageSend(channels.GetHRChannel(), fmt.Sprintf("%s, has left the corporation and their discord access has been removed.", m.User.GlobalName))
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
	if roles.HasRole(r, roles.GetBlueRoleID()) {
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

func memberLosesRecruitRole(s *discordgo.Session, m *discordgo.GuildMemberUpdate, r []string, e eventWorker.Event) bool {
	// Check if this role change was initiated by the bot
	if helper.WasRoleChangeInitiatedByBot(s, m.User.ID) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "skip_bot_initiated_removal",
			"member_id": m.User.ID,
			"message":   "Skipping lostRoleHandler for bot-initiated role removal",
		})
		return false
	}
	if roles.HasRole(r, roles.GetRecruitRoleID()) && !roles.HasRole(m.Roles, roles.GetMemberRoleID()) {

		err := users.RemoveRecruitmentDate(m.User.ID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "remove_recruitment_date",
				"member_id": m.User.ID,
			})
		}

		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "member_lose_recruit",
			"member_id": m.User.ID,
			"message":   "Member Loses Recruit Role Complete",
		})

		recruitmentChannelID := channels.GetRecruitmentForum()
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		if found {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_message_added",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
				})
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s has left the recruitment channel.", m.User.GlobalName))
				return err
			})
		}

		monitoring.RemoveScenario(m.User.ID, models.MonitoringScenarioRecruitmentProcess)

		return true
	}
	return false
}
