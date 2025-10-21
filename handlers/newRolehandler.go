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

		message := fmt.Sprintf(globals.RecruitmentWelcomeMessage, m.User.ID)
		helper.SendChannelMessage(s, channels.GetRecruitmentChannel(), message, e)

		if roles.HasRole(m.Roles, roles.GetNewcomerRoleID()) {
			helper.RemoveRole(s, m.GuildID, m.User.ID, roles.GetNewcomerRoleID(), e)
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

		// Also schedule midpoint reminder for recruitment process if it's in the future
		startTime := time.Now()
		if err := monitoring.CreateRecruitmentReminderAtMidpoint(context.Background(), m.User.ID, startTime, models.MonitoringScenarioRecruitmentProcess); err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "create_recruitment_reminder",
				"message":  "Failed to create recruitment reminder",
				"error":    err.Error(),
				"user_id":  m.User.ID,
			})
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

		helper.SendChannelMessage(s, channels.GetRecruitmentHub(), fmt.Sprintf("%s has completed the authentication steps.", m.Member.DisplayName()), e)

		helper.SendDirectMessage(s, m.User.ID,
			"The authentication steps for Astral Acquisitions Inc have been completed. Please reach out to a recruiter in the recruitment channel.", e)

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
		helper.RemoveRoles(s, m.GuildID, m.User.ID, rolesToRemove, e)
		helper.AddRoles(s, m.GuildID, m.User.ID, roles.ContentNotificationRoles, e)

		message := fmt.Sprintf(globals.MemberJoinWelcomeMessage, m.Member.DisplayName(), m.User.ID)
		helper.SendChannelMessage(s, channels.GetGeneralChannel(), message, e)
		helper.SendDirectMessage(s, m.User.ID, message, e)

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
		helper.RemoveRole(s, m.GuildID, m.User.ID, roles.GetNewcomerRoleID(), e)
		return true
	}
	return false
}
