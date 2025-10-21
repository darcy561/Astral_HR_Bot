package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	"astralHRBot/users"
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
		helper.RemoveRoles(s, m.GuildID, m.User.ID, roles.ContentNotificationRoles, e)

		monitoring.RemoveAllScenarios(m.User.ID)

		helper.RemoveRole(s, m.GuildID, m.User.ID, roles.GetAbsenteeRoleID(), e)
		helper.AddRole(s, m.GuildID, m.User.ID, roles.GetGuestRoleID(), e)

		message := fmt.Sprintf("%s, has left the corporation and their discord access has been removed.", m.User.GlobalName)
		helper.SendChannelMessage(s, channels.GetHRChannel(), message, e)

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
		helper.AddRole(s, m.GuildID, m.User.ID, roles.GetGuestRoleID(), e)

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

		rtm := helper.NewRecruitmentThreadManager(s, e, m.User.ID)
		rtm.SendMessage(fmt.Sprintf("%s has left the recruitment channel.", m.User.GlobalName))

		monitoring.RemoveScenario(m.User.ID, models.MonitoringScenarioRecruitmentProcess)

		return true
	}
	return false
}
