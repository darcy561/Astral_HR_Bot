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
	"context"
	"fmt"
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
					_, err := bot.Discord.ChannelMessageSend(recruitmentThread.ID, "No activity within the last 7 days. Flagged for removal.")
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
