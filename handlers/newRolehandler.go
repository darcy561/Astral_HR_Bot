package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	"astralHRBot/users"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleGained(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) {
	if welcomeNewRecruit(s, m, a, e) {
		return
	}
	if recruitAuthenticated(s, m, a, e) {
		return
	}
	if newMemberOnboarding(s, m, a, e) {
		return
	}
	if memberRecievesGuestRole(s, m, a, e) {
		return
	}
}

func welcomeNewRecruit(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {
	if roles.HasRole(a, roles.GetRecruitRoleID()) && !roles.HasRole(m.Roles, roles.GetServerClownRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "welcome_new_recruit",
			"member_id": m.User.ID,
		})

		channelID := channels.GetRecruitmentChannel()
		message := fmt.Sprintf(globals.RecruitmentWelcomeMessage, m.User.ID)

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "welcome_message_sent",
				"member_id": m.User.ID,
				"channel":   channelID,
			})

			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		if roles.HasRole(m.Roles, roles.GetNewcomerRoleID()) {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "role_removed",
					"member_id": m.User.ID,
					"role":      "newcomer",
				})

				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetNewcomerRoleID())
				return err
			})
		}

		rtm := helper.NewRecruitmentThreadManager(s, e, m.User.ID)

		if !rtm.HasThread() {
			rtm.CreateThread(m.User.GlobalName, m.User.ID)
		} else {
			rtm.ReopenThread()
			rtm.SendMessage(fmt.Sprintf("%s Rejoined Recruitment", m.User.GlobalName))
			rtm.RemoveTags("")
		}

		// Update recruitment date in Redis
		err := users.UpdateRecruitmentDate(m.User.ID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "recruitment_date_update",
				"message":   "Failed to update recruitment date",
				"error":     err.Error(),
				"member_id": m.User.ID,
			})
		}

		params := &models.RecruitmentCleanupParams{UserID: m.User.ID}
		scheduledTime := time.Now().Add(time.Duration(globals.GetRecruitmentCleanupDelay()) * 24 * time.Hour).Unix()

		newTask, err := models.NewTaskWithScenario(
			models.TaskRecruitmentCleanup,
			params,
			scheduledTime,
			string(models.MonitoringScenarioRecruitmentProcess),
		)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "create_task",
				"error":    err.Error(),
			})
			return true
		}

		err = db.SaveTaskToRedis(context.Background(), *newTask)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "save_task_to_redis",
				"error":    err.Error(),
			})
			return true
		}

		monitoring.AddScenario(m.User.ID, models.MonitoringScenarioRecruitmentProcess)

		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "welcome_new_recruit",
			"member_id": m.User.ID,
		})
		return true
	}
	return false
}

func recruitAuthenticated(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {
	if roles.HasRole(m.Roles, roles.GetRecruitRoleID()) && roles.HasRole(a, roles.GetAuthenticatedGuestRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "recruit_authenticated",
			"member_id": m.User.ID,
		})

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "recruitment_message_sent",
				"member_id": m.User.ID,
				"channel":   channels.GetRecruitmentHub(),
			})

			_, err := s.ChannelMessageSend(channels.GetRecruitmentHub(), fmt.Sprintf("%s has completed the authentication steps.", m.Member.DisplayName()))
			if err != nil {
				return err
			}
			return nil
		})

		rtm := helper.NewRecruitmentThreadManager(s, e, m.User.ID)
		if rtm.HasThread() {
			updatedThreadTitle := fmt.Sprintf("%s - %s", m.Member.DisplayName(), m.User.ID)
			rtm.UpdateThreadTitle(updatedThreadTitle)
			rtm.SendMessage(fmt.Sprintf("%s Authentication Steps Complete.", m.Member.DisplayName()))
		} else {
			logger.Info(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "thread_not_found",
				"member_id": m.User.ID,
				"message":   "no existing recruitment thread found",
			})
		}
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "recruit_authenticated",
			"member_id": m.User.ID,
		})
		return true
	}
	return false
}

func newMemberOnboarding(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {
	if (roles.HasRole(m.Roles, roles.GetRecruitRoleID()) || roles.HasRole(m.Roles, roles.GetAuthenticatedGuestRoleID())) && roles.HasRole(a, roles.GetAuthenticatedMemberRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "new_member_onboarding",
			"member_id": m.User.ID,
		})

		rolesToRemove := []string{
			roles.GetNewcomerRoleID(), roles.GetRecruitRoleID(), roles.GetGuestRoleID(),
		}

		for _, roleID := range rolesToRemove {
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

		for _, roleID := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "role_added",
					"member_id": m.User.ID,
					"role_id":   roleID,
				})
				err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roleID)
				return err
			})
		}

		message := fmt.Sprintf(globals.MemberJoinWelcomeMessage, m.Member.DisplayName(), m.User.ID)

		channelID := channels.GetGeneralChannel()
		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "welcome_message_sent",
				"member_id": m.User.ID,
				"channel":   channelID,
			})
			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		rtm := helper.NewRecruitmentThreadManager(s, e, m.User.ID)

		if rtm.HasThread() {
			rtm.SendMessage("Character Joined Corporation.")

			params := &models.UserCheckinParams{UserID: m.User.ID}
			scheduledTime := time.Now().Add(time.Duration(globals.GetNewRecruitTrackingDays()) * 24 * time.Hour).Unix()

			newTask, err := models.NewTaskWithScenario(
				models.TaskUserCheckin,
				params,
				scheduledTime,
				string(models.MonitoringScenarioNewRecruit),
			)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "create_task",
					"error":    err.Error(),
				})
			} else {
				// Remove recruitment process scenario if it exists
				monitoring.RemoveScenario(m.User.ID, models.MonitoringScenarioRecruitmentProcess)

				err = db.SaveTaskToRedis(context.Background(), *newTask)
				if err != nil {
					logger.Error(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "save_task_to_redis",
						"error":    err.Error(),
					})
				}
			}

			monitoring.AddUserTracking(m.User.ID, models.MonitoringScenarioNewRecruit, time.Duration(int(globals.GetNewRecruitTrackingDays()))*24*time.Hour)

			rtm.SendMessage(fmt.Sprintf("User checkin scheduled for %d days time.", int(globals.GetNewRecruitTrackingDays())))
			rtm.CloseThread("Accepted")
		}

		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_complete",
			"process":   "new_member_onboarding",
			"member_id": m.User.ID,
		})
		return true
	}
	return false
}

func memberRecievesGuestRole(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {
	if roles.HasRole(a, roles.GetGuestRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "member_recieves_guest_role",
			"member_id": m.User.ID,
		})
		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "role_removed",
				"member_id": m.User.ID,
				"role":      "newcomer",
			})

			err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetNewcomerRoleID())
			return err
		})
		return true
	}
	return false
}
