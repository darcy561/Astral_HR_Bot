package commands

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RebuildUserEventsCommand handles the /rebuilduserevents slash command
func RebuildUserEventsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "rebuild_user_events_command",
		"message": "RebuildUserEvents command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the user ID from the command options
	userID := i.ApplicationCommandData().Options[0].UserValue(s).ID

	ctx := context.Background()

	// Get user monitoring data
	monitoringData, err := db.GetUserMonitoring(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_user_events_command",
			"message": "Failed to get user monitoring",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, "Error retrieving monitoring data", true)
		return
	}

	if monitoringData == nil {
		RespondToInteraction(s, i, "User is not currently being monitored", true)
		return
	}

	// Check existing tasks
	existingTasks, err := db.GetTasksForUser(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_user_events_command",
			"message": "Failed to get existing tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, "Error retrieving existing tasks", true)
		return
	}

	// Remove existing tasks first
	tasksRemoved := 0
	for _, task := range existingTasks {
		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_user_events_command",
				"message": "Failed to delete existing task",
				"error":   err.Error(),
				"task_id": task.TaskID,
				"user_id": userID,
			})
		} else {
			tasksRemoved++
		}
	}

	// Recreate tasks using the monitoring system's recreation logic
	err = monitoring.RecreateTasksForUser(userID, monitoringData)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_user_events_command",
			"message": "Failed to recreate tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, fmt.Sprintf("Error recreating tasks: %s", err.Error()), true)
		return
	}

	// Get the new tasks to show what was created
	newTasks, err := db.GetTasksForUser(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_user_events_command",
			"message": "Failed to get new tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, "Tasks recreated but failed to retrieve details", true)
		return
	}

	// Get updated monitoring data to check for removed scenarios
	updatedMonitoringData, err := db.GetUserMonitoring(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_user_events_command",
			"message": "Failed to get updated monitoring data",
			"error":   err.Error(),
			"user_id": userID,
		})
	}

	// Get user name from Discord
	user, err := s.User(userID)
	userName := userID // fallback to user ID if we can't get the user
	if err == nil && user != nil {
		userName = user.Username
	}

	// Create response message
	response := fmt.Sprintf("✅ **Events rebuilt for %s**\n\n", userName)
	response += fmt.Sprintf("**Removed:** %d existing tasks\n", tasksRemoved)
	response += fmt.Sprintf("**Created:** %d new tasks\n\n", len(newTasks))

	if len(newTasks) > 0 {
		response += "**📋 New Tasks:**\n"
		for _, task := range newTasks {
			scheduledTime := time.Unix(task.ScheduledTime, 0).Format("2006-01-02 15:04:05")
			response += fmt.Sprintf("• **%s** - Scheduled: `%s`\n", string(task.FunctionName), scheduledTime)
		}
	}

	// Add monitoring info
	var activeScenarios []string
	if updatedMonitoringData != nil {
		scenarios := updatedMonitoringData.GetScenarios()
		scenarioNames := make([]string, len(scenarios))
		for i, scenario := range scenarios {
			scenarioNames[i] = string(scenario)
		}
		activeScenarios = scenarioNames
	}

	if len(activeScenarios) > 0 {
		response += fmt.Sprintf("\n**Active Scenarios:** `%s`", fmt.Sprintf("%v", activeScenarios))
	} else {
		response += "\n**⚠️ All scenarios expired and removed**"
	}

	RespondToInteraction(s, i, response, true)
}

// GetRebuildUserEventsCommandDefinition returns the rebuilduserevents command definition
func GetRebuildUserEventsCommandDefinition() *discordgo.ApplicationCommand {
	adminPerm := int64(discordgo.PermissionAdministrator)
	return &discordgo.ApplicationCommand{
		Name:                     "rebuilduserevents",
		Description:              "Rebuild events/tasks for a monitored user",
		DefaultMemberPermissions: &adminPerm,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The user to rebuild events for",
				Required:    true,
			},
		},
	}
}
