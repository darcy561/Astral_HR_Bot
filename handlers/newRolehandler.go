package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/db"
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
	if hasRole(a, roles.GetRecruitRoleID()) && !hasRole(m.Roles, roles.GetServerClownRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "welcome_new_recruit",
			"member_id": m.User.ID,
		})

		channelID := channels.GetRecruitmentChannel()
		message := fmt.Sprintf(
			"Welcome <@%s>! \n\n"+
				"A member of the recruitment team will be with you shortly. In the meantime, please follow these steps:\n\n"+
				"[Alliance Auth](https://auth.astralinc.space/)\n\n"+
				"* Follow the above link and register your character(s).\n"+
				"* In the **Char Link** tab, authorize each of your characters.\n"+
				"* In the **Member Audit** tab, register each of your characters.\n"+
				"* In the **Services** tab, click the checkbox to link your Discord account.\n\n"+
				"Once you've completed this, a green tick should appear next to your character name on Discord.",
			m.User.ID,
		)

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

		if hasRole(m.Roles, roles.GetNewcomerRoleID()) {
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
			ScheduledTime: time.Now().Add(time.Minute * 1).Unix(),
			Status:        "pending",
			Retries:       0,
			CreatedBy:     "system",
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
	if hasRole(m.Roles, roles.GetRecruitRoleID()) && hasRole(a, roles.GetAuthenticatedGuestRoleID()) {
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
	if (hasRole(m.Roles, roles.GetRecruitRoleID()) || hasRole(m.Roles, roles.GetAuthenticatedGuestRoleID())) && hasRole(a, roles.GetAuthenticatedMemberRoleID()) {
		logger.Debug(logger.LogData{
			"trace_id":  e.TraceID,
			"action":    "process_start",
			"process":   "new_member_onboarding",
			"member_id": m.User.ID,
		})

		rolesToRemove := []string{
			roles.GetNewcomerRoleID(), roles.GetRecruitRoleID(), roles.GetGuestRoleID(), roles.GetLegacyGuestRoleID(),
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

		message := fmt.Sprintf(
			"Welcome to Astral, %s <@%s> o/ \n\n"+
				"Please take a look at <#1229904357697261569> for guides, and specifically the newbro doc for info on our region.\n\n"+
				"If you need a hand moving your stuff around, feel free to head over to <#1082494747937087581> to speak with them directly.\n\n"+
				"Most importantly, head over to <#1161264045584822322> to opt out of the content pings that do not interest you.\n\n"+
				"Clear skies,\n"+
				"And KTF!",
			m.Member.DisplayName(), m.User.ID,
		)

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

				_, err := s.ChannelMessageSend(recruitmentThread.ID, "Character Joined Corp")
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
				ScheduledTime: time.Now().Add(time.Minute * 1).Unix(),
				Status:        "pending",
				Retries:       0,
				CreatedBy:     "system",
			}

			monitoring.AddScenario(m.User.ID, models.MonitoringScenarioNewRecruit)

			err = db.SaveTaskToRedis(context.Background(), newTask)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "save_task_to_redis",
					"error":    err.Error(),
				})
			}

			monitoring.AddUserTracking(m.User.ID, models.MonitoringScenarioNewRecruit, time.Hour*24*7)

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(logger.LogData{
					"trace_id":  e.TraceID,
					"action":    "task_created",
					"member_id": m.User.ID,
				})
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("Check in task created"))

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
