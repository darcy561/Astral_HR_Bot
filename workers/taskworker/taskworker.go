package taskworker

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
	"time"

	"github.com/bwmarrin/discordgo"
)

// StartTaskProcessor starts a background goroutine that processes tasks every 5 seconds
func StartTaskProcessor() {
	go func() {
		// Wait for Discord to be ready
		<-bot.ReadyChan
		logger.Info(logger.LogData{
			"action":  "task_processor",
			"message": "Discord connection established, starting task processing",
		})

		for {
			tasks, err := db.FetchLatestTasks(context.Background())

			if err != nil {
				logger.Error(logger.LogData{
					"action":  "start_task_processor",
					"message": "Failed to get tasks",
					"error":   err.Error(),
				})
				time.Sleep(5 * time.Second)
				continue
			}

			for _, task := range tasks {
				//switch for task type
				switch task.FunctionName {
				case models.TaskRecruitmentCleanup:
					go processRecruitmentCleanup(task)
				case models.TaskUserCheckin:
					go processUserCheckin(task)
				default:
					logger.Error(logger.LogData{
						"action":  "start_task_processor",
						"message": "Unknown task type",
						"task":    task,
					})
				}
			}

			time.Sleep(5 * time.Second)
		}
	}()
}

func processRecruitmentCleanup(task models.Task) {

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

		user, err := db.GetUserFromRedis(ctx, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "process_recruitment_cleanup",
				"message": "Failed to get user from redis",
				"error":   err.Error(),
			})
			return
		}

		isZero := user.LastMessageDate.IsZero()

		if isZero {
			discordAPIWorker.NewRequest(e, func() error {
				err := bot.Discord.GuildMemberRoleRemove(bot.Discord.State.Guilds[0].ID, e.UserID, roles.GetRecruitRoleID())
				if err != nil {
					logger.Error(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "process_recruitment_cleanup",
						"message":  "Failed to remove recruit role",
						"error":    err.Error(),
					})
				}
				logger.Debug(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_recruitment_cleanup",
					"message":  "Recruit role removed",
					"user_id":  e.UserID,
				})
				return err
			})

			recruitmentChannelID := channels.GetRecruitmentForum()
			recruitmentThread, found := helper.FindForumThreadByTitle(bot.Discord, recruitmentChannelID, e.UserID)

			if found {
				discordAPIWorker.NewRequest(e, func() error {
					_, err := bot.Discord.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("No activity within the last 7 days. Flagged for removal."))
					if err != nil {
						logger.Error(logger.LogData{
							"trace_id": e.TraceID,
							"action":   "process_recruitment_cleanup",
							"message":  "Failed to send message to recruitment thread",
							"error":    err.Error(),
						})
					}
					logger.Debug(logger.LogData{
						"trace_id": e.TraceID,
						"action":   "process_recruitment_cleanup",
						"message":  "Message sent to recruitment thread",
						"user_id":  e.UserID,
					})
					return err
				})
			}
		}

		err = db.DeleteTaskFromRedis(ctx, task.TaskID)

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

func processUserCheckin(task models.Task) {
	params, err := task.GetParams()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_user_checkin",
			"message": "Failed to get params",
			"error":   err.Error(),
		})
		return
	}

	parms := params.(*models.UserCheckinParams)
	fmt.Println("Processing user checkin for user", parms.UserID)

	eventWorker.Submit(parms.UserID, func(e eventWorker.Event) {
		ctx := context.Background()

		// Get user info from Discord
		member, err := bot.Discord.GuildMember(bot.Discord.State.Guilds[0].ID, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to get member from Discord",
				"error":    err.Error(),
			})
			return
		}

		displayName := member.Nick
		if displayName == "" {
			displayName = member.User.GlobalName
			if displayName == "" {
				displayName = member.User.Username
			}
		}

		stats, err := db.GetUserAnalytics(ctx, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to get user analytics",
				"error":    err.Error(),
			})
			return
		}

		fmt.Println("User analytics", stats)

		embededMessage := discordgo.MessageEmbed{
			Title:       fmt.Sprintf("%s's First Week Analytics", displayName),
			Description: fmt.Sprintf("Here's how %s has been engaging with our community in their first week:", displayName),
			Color:       0x000000,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📝 Messages Sent",
					Value:  fmt.Sprintf("%d", stats.Messages),
					Inline: true,
				},
				{
					Name:   "🎙️ Voice Joins",
					Value:  fmt.Sprintf("%d", stats.VoiceJoins),
					Inline: true,
				},
				{
					Name:   "📨 Invites Created",
					Value:  fmt.Sprintf("%d", stats.Invites),
					Inline: true,
				},
				{
					Name:   "📌 Most Active Channel",
					Value:  fmt.Sprintf("<#%s>", stats.TopChannelID),
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "User activity tracker",
			},
		}

		// Send to recruitment hub
		discordAPIWorker.NewRequest(e, func() error {
			_, err := bot.Discord.ChannelMessageSendEmbed(channels.GetRecruitmentHub(), &embededMessage)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_user_checkin",
					"message":  "Failed to send message to recruitment hub",
					"error":    err.Error(),
				})
				return err
			}
			return nil
		})

		// Find and handle the recruitment thread
		recruitmentThread, found := helper.FindForumThreadByTitle(bot.Discord, channels.GetRecruitmentForum(), e.UserID)
		if !found {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to find recruitment thread",
				"user_id":  e.UserID,
			})
			return
		}

		// Reopen thread if needed
		discordAPIWorker.NewRequest(e, func() error {
			archived := false
			_, err := bot.Discord.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
				Archived: &archived,
			})
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_user_checkin",
					"message":  "Failed to reopen thread",
					"error":    err.Error(),
				})
				return err
			}
			return nil
		})

		// Send message to thread
		discordAPIWorker.NewRequest(e, func() error {
			_, err := bot.Discord.ChannelMessageSendEmbed(recruitmentThread.ID, &embededMessage)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_user_checkin",
					"message":  "Failed to send message to recruitment thread",
					"error":    err.Error(),
				})
				return err
			}
			return nil
		})

		// Re-archive thread
		discordAPIWorker.NewRequest(e, func() error {
			archived := true
			_, err := bot.Discord.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
				Archived: &archived,
			})
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "process_user_checkin",
					"message":  "Failed to re-archive thread",
					"error":    err.Error(),
				})
				return err
			}
			return nil
		})

		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to delete task from redis",
				"error":    err.Error(),
			})
			return
		}
		monitoring.RemoveUserTracking(e.UserID)
	})
}
