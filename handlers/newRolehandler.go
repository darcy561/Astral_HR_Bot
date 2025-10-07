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
	"encoding/json"
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

		recruitmentChannelID := channels.GetRecruitmentForum()
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		if !found {
			discordAPIWorker.NewRequest(e, func() error {
				newThreadTitle := fmt.Sprintf("%s - %s", m.User.GlobalName, m.User.ID)
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_created",
					"member_id": m.User.ID,
					"title":     newThreadTitle,
				})

				_, err := s.ForumThreadStart(recruitmentChannelID, newThreadTitle, 10080, fmt.Sprintf("%s Joined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})

		} else {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_reopened",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
				})

				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					AutoArchiveDuration: 0,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_message_added",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
				})
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Rejoined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})
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

		params, err := json.Marshal(models.RecruitmentCleanupParams{UserID: m.User.ID})
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "marshal_params",
				"error":    err.Error(),
			})
			return true
		}

		newTask := models.Task{
			FunctionName:  models.TaskRecruitmentCleanup,
			Params:        params,
			ScheduledTime: time.Now().Add(time.Duration(globals.GetRecruitmentCleanupDelay()) * 24 * time.Hour).Unix(),
			Status:        "pending",
			Retries:       0,
			CreatedBy:     "system",
			Scenario:      string(models.MonitoringScenarioRecruitmentProcess),
		}

		db.SaveTaskToRedis(context.Background(), newTask)

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

		recruitmentChannelID := channels.GetRecruitmentForum()
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)
		if found {
			updatedThreadTitle := fmt.Sprintf("%s - %s", m.Member.DisplayName(), m.User.ID)

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_updated",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
					"new_title": updatedThreadTitle,
				})

				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					Name: updatedThreadTitle,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_message_added",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
				})

				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Authentication Steps Complete.", m.Member.DisplayName()))
				if err != nil {
					return err
				}
				return nil
			})

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

		recruitmentChannelID := channels.GetRecruitmentForum()
		recruitmentChannel, err := s.Channel(recruitmentChannelID)
		if err != nil {
			logger.Warn(logger.LogData{
				"trace_id":  e.TraceID,
				"action":    "channel_fetch_failed",
				"member_id": m.User.ID,
				"error":     err.Error(),
			})
		}

		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		tagsToApply := []string{}
		if recruitmentChannel != nil {
			for _, tag := range recruitmentChannel.AvailableTags {
				if tag.Name == "Accepted" {
					tagsToApply = append(tagsToApply, tag.ID)
					break
				}
			}
		}

		if found {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_message_added",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
				})

				_, err := s.ChannelMessageSend(recruitmentThread.ID, "Character Joined Corporation.")
				return err
			})

			params, err := json.Marshal(models.UserCheckinParams{UserID: m.User.ID})
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "marshal_params",
					"error":    err.Error(),
				})
			}
			newTask := models.Task{
				FunctionName:  models.TaskUserCheckin,
				Params:        params,
				ScheduledTime: time.Now().Add(time.Duration(globals.GetNewRecruitTrackingDays()) * 24 * time.Hour).Unix(),
				Status:        "pending",
				Retries:       0,
				CreatedBy:     "system",
				Scenario:      string(models.MonitoringScenarioNewRecruit),
			}

			// Remove recruitment process scenario if it exists
			monitoring.RemoveScenario(m.User.ID, models.MonitoringScenarioRecruitmentProcess)

			err = db.SaveTaskToRedis(context.Background(), newTask)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "save_task_to_redis",
					"error":    err.Error(),
				})
			}

			monitoring.AddUserTracking(m.User.ID, models.MonitoringScenarioNewRecruit, time.Duration(int(globals.GetNewRecruitTrackingDays()))*24*time.Hour)

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "task_created",
					"member_id": m.User.ID,
				})
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("User checkin scheduled for %d days time.", int(globals.GetNewRecruitTrackingDays())))

				if err != nil {
					logger.Error(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "send_message",
						"error":    err.Error(),
					})
					return err
				}
				return nil
			})

			isArchived := true
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "thread_modified",
					"member_id": m.User.ID,
					"thread_id": recruitmentThread.ID,
					"archived":  true,
				})
				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					Archived:    &isArchived,
					AppliedTags: &tagsToApply,
				})
				if err != nil {
					logger.Error(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "edit_thread",
						"error":    err.Error(),
					})
					return err
				}
				return nil
			})
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
