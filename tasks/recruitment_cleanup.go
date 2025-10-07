package tasks

import (
	"astralHRBot/bot"
	"astralHRBot/channels"
	"astralHRBot/db"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// ProcessRecruitmentCleanup handles the recruitment cleanup task
func ProcessRecruitmentCleanup(task models.Task) {
	params, err := task.GetParams()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_cleanup",
			"message": "Failed to get params",
			"error":   err.Error(),
		})
		return
	}

	parms := params.(*models.RecruitmentCleanupParams)
	fmt.Println("Processing recruitment cleanup for user", parms.UserID)

	eventWorker.Submit(parms.UserID, func(e eventWorker.Event) {
		ctx := context.Background()

		// Get analytics for the recruitment process scenario
		analyticsKey := fmt.Sprintf("user:%s:analytics:recruitment_process", e.UserID)
		fields, err := db.GetRedisClient().HGetAll(ctx, analyticsKey).Result()
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "process_recruitment_cleanup",
				"message": "Failed to get recruitment process analytics",
				"error":   err.Error(),
			})
			return
		}

		// Check if user has been active (sent messages or created invites)
		hasActivity := false

		// Check messages count
		if messages, ok := fields["messages"]; ok {
			if val, err := strconv.ParseInt(messages, 10, 64); err == nil && val > 0 {
				hasActivity = true
			}
		}

		logger.Debug(logger.LogData{
			"trace_id":     e.TraceID,
			"action":       "process_recruitment_cleanup",
			"message":      "Checked recruitment process analytics",
			"user_id":      e.UserID,
			"analytics":    fields,
			"has_activity": hasActivity,
		})

		if hasActivity {
			logger.Debug(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_recruitment_cleanup",
				"message":  "User has activity in recruitment process scenario - keeping recruit role",
				"user_id":  e.UserID,
			})

			// Send confirmation message to recruitment thread
			recruitmentChannelID := channels.GetRecruitmentForum()
			recruitmentThread, found := helper.FindForumThreadByTitle(bot.Discord, recruitmentChannelID, e.UserID)

			if found {
				logger.Debug(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_recruitment_cleanup",
					"message":  "Found recruitment thread - sending automated check confirmation",
					"user_id":  e.UserID,
				})

				discordAPIWorker.NewRequest(e, func() error {
					_, err := bot.Discord.ChannelMessageSend(recruitmentThread.ID, "✅ Automated check passed - user has shown activity in recruitment process scenario. Keeping recruit role.")
					if err != nil {
						logger.Error(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Failed to send confirmation message to recruitment thread",
							"error":    err.Error(),
						})
					} else {
						logger.Debug(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Successfully sent automated check confirmation to recruitment thread",
							"user_id":  e.UserID,
						})
					}
					return err
				})
			} else {
				logger.Debug(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_recruitment_cleanup",
					"message":  "No recruitment thread found - skipping confirmation message",
					"user_id":  e.UserID,
				})
			}
		}

		if !hasActivity {
			logger.Debug(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_recruitment_cleanup",
				"message":  "User has no activity in recruitment process scenario - removing recruit role",
				"user_id":  e.UserID,
			})

			discordAPIWorker.NewRequest(e, func() error {
				err := bot.Discord.GuildMemberRoleRemove(bot.Discord.State.Guilds[0].ID, e.UserID, roles.GetRecruitRoleID())
				if err != nil {
					logger.Error(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "process_recruitment_cleanup",
						"message":  "Failed to remove recruit role",
						"error":    err.Error(),
					})
					return err
				} else {
					logger.Debug(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "process_recruitment_cleanup",
						"message":  "Successfully removed recruit role",
						"user_id":  e.UserID,
					})
				}
				return nil
			})

			recruitmentChannelID := channels.GetRecruitmentForum()
			recruitmentThread, found := helper.FindForumThreadByTitle(bot.Discord, recruitmentChannelID, e.UserID)

			if found {
				logger.Debug(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_recruitment_cleanup",
					"message":  "Found recruitment thread - sending removal notification and closing thread",
					"user_id":  e.UserID,
				})

				// Send removal notification
				discordAPIWorker.NewRequest(e, func() error {
					_, err := bot.Discord.ChannelMessageSend(recruitmentThread.ID, "❌ No activity in recruitment process scenario within the last 7 days. Flagged for removal.")
					if err != nil {
						logger.Error(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Failed to send message to recruitment thread",
							"error":    err.Error(),
						})
						return err
					} else {
						logger.Debug(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Successfully sent removal notification to recruitment thread",
							"user_id":  e.UserID,
						})
					}
					return nil
				})

				// Close the thread and apply tag
				discordAPIWorker.NewRequest(e, func() error {
					// Get the recruitment channel to find available tags
					recruitmentChannel, err := bot.Discord.Channel(recruitmentChannelID)
					if err != nil {
						logger.Error(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Failed to get recruitment channel for tags",
							"error":    err.Error(),
						})
						return err
					}

					// Find the "Newbie role removed" tag
					tagsToApply := []string{}
					if recruitmentChannel != nil {
						for _, tag := range recruitmentChannel.AvailableTags {
							if tag.Name == "Newbie role removed" {
								tagsToApply = append(tagsToApply, tag.ID)
								break
							}
						}
					}

					// Close the thread and apply tag
					isArchived := true
					_, err = bot.Discord.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
						Archived:    &isArchived,
						AppliedTags: &tagsToApply,
					})

					if err != nil {
						logger.Error(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Failed to close recruitment thread and apply tag",
							"error":    err.Error(),
						})
					} else {
						logger.Debug(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Successfully closed recruitment thread and applied 'Newbie role removed' tag",
							"user_id":  e.UserID,
						})
					}
					return err
				})
			} else {
				logger.Debug(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_recruitment_cleanup",
					"message":  "No recruitment thread found - skipping notification and closure",
					"user_id":  e.UserID,
				})
			}
		}

		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		monitoring.RemoveScenario(e.UserID, models.MonitoringScenarioRecruitmentProcess)

		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_recruitment_cleanup",
				"message":  "Failed to delete task from redis",
				"error":    err.Error(),
			})
			return
		}

	}, nil)
}
