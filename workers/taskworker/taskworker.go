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

		userAnalytics, err := db.GetUserAnalytics(ctx, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to get user analytics",
				"error":    err.Error(),
			})
			return
		}

		fmt.Println("User analytics", userAnalytics)

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
