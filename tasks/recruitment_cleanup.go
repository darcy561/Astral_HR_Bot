package tasks

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"strconv"
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
			rtm := helper.NewRecruitmentThreadManager(bot.Discord, e, e.UserID)
			rtm.SendMessage("✅ Automated check passed - user has shown activity in recruitment process scenario. Keeping recruit role.")
		}

		if !hasActivity {
			logger.Debug(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_recruitment_cleanup",
				"message":  "User has no activity in recruitment process scenario - removing recruit role",
				"user_id":  e.UserID,
			})

			helper.RemoveRole(bot.Discord, bot.Discord.State.Guilds[0].ID, e.UserID, roles.GetRecruitRoleID(), e)

			rtm := helper.NewRecruitmentThreadManager(bot.Discord, e, e.UserID)
			rtm.SendMessageAndClose("❌ No activity in recruitment process scenario within the last 7 days. Flagged for removal.", "Newbie role removed")
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
