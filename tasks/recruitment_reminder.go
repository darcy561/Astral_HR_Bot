package tasks

import (
	"astralHRBot/bot"
	"astralHRBot/channels"
	"astralHRBot/db"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/roles"
	"astralHRBot/workers/eventWorker"
	"context"
	"fmt"
)

// ProcessRecruitmentReminder sends or logs a reminder for upcoming recruitment cleanup
func ProcessRecruitmentReminder(task models.Task) {
	paramsAny, err := task.GetParams()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_reminder",
			"message": "failed to parse task params",
			"error":   err.Error(),
			"task_id": task.TaskID,
		})
		return
	}

	params, ok := paramsAny.(*models.RecruitmentReminderParams)
	if !ok {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_reminder",
			"message": "invalid params type",
			"task_id": task.TaskID,
		})
		return
	}

	guildID, err := bot.GetGuildID()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_reminder",
			"message": "failed to get guild ID",
			"error":   err.Error(),
			"task_id": task.TaskID,
		})
		return
	}

	user, err := bot.Discord.GuildMember(guildID, params.UserID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_reminder",
			"message": "failed to get user",
			"error":   err.Error(),
			"user_id": params.UserID,
		})
		return
	}

	// Get message count from analytics
	analyticsKey := fmt.Sprintf("user:%s:analytics:recruitment_process", params.UserID)
	fields, err := db.GetRedisClient().HGetAll(context.Background(), analyticsKey).Result()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_recruitment_reminder",
			"message": "failed to get analytics",
			"error":   err.Error(),
		})
		return
	}

	// Check if user has sent any messages during recruitment process
	messageCount := 0
	if messageCountStr, exists := fields["messages"]; exists {
		fmt.Sscanf(messageCountStr, "%d", &messageCount)
	}

	// Check if user is authenticated
	isAuthenticated := roles.HasRole(user.Roles, roles.GetAuthenticatedGuestRoleID())

	// Early return if user is authenticated and has been active
	if isAuthenticated && messageCount > 0 {
		logger.Info(logger.LogData{
			"action":   "process_recruitment_reminder",
			"message":  "Authenticated user has been active - no reminder needed",
			"user_id":  params.UserID,
			"messages": messageCount,
		})
		// Mark task as done by removing it
		_ = db.DeleteTaskFromRedis(context.Background(), task.TaskID)
		return
	}

	// Handle reminder logic based on authentication status
	event := eventWorker.Event{
		TraceID: task.TaskID,
		UserID:  params.UserID,
	}

	if isAuthenticated {
		message := fmt.Sprintf(
			"<@%s> It looks like you have completed the authentication steps. If you are still interested in joining the corporation, please reach out to a recruiter in the recruitment channel.",
			params.UserID,
		)
		helper.SendChannelMessage(bot.Discord, channels.GetRecruitmentChannel(), message, event)
	} else {
		message := fmt.Sprintf(
			"<@%s> Are you still interested in joining the corporation? If so, please complete the authentication steps provided previously and reach out to a recruiter.",
			params.UserID,
		)
		helper.SendChannelMessage(bot.Discord, channels.GetRecruitmentChannel(), message, event)
	}

	// Mark task as done by removing it (idempotent with current queue model)
	_ = db.DeleteTaskFromRedis(context.Background(), task.TaskID)
}
