package commands

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// MonitoringStatusCommand handles the /monitoring-status slash command
func MonitoringStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "monitoring_status_command",
		"message": "MonitoringStatus command executed",
		"user_id": i.Member.User.ID,
	})

	RespondToInteraction(s, i, "🔄 **Fetching all monitored users...**", true)

	ctx := context.Background()

	// Get all tracked users
	trackedUsers, err := db.GetTrackedUsers(ctx)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "monitoring_status_command",
			"message": "Failed to get tracked users",
			"error":   err.Error(),
		})
		FollowUpMessage(s, i, fmt.Sprintf("Error getting tracked users: %s", err.Error()), true)
		return
	}

	if len(trackedUsers) == 0 {
		FollowUpMessage(s, i, "No users are currently being monitored", true)
		return
	}

	// Process each user
	userDetails := []string{}
	activeUsers := 0
	expiredUsers := 0

	for _, userID := range trackedUsers {
		// Get user monitoring data
		monitoringData, err := db.GetUserMonitoring(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "monitoring_status_command",
				"message": "Failed to get user monitoring data",
				"error":   err.Error(),
				"user_id": userID,
			})
			// Get user name for error message
			user, err := s.User(userID)
			userName := userID // fallback to user ID if we can't get the user
			if err == nil && user != nil {
				userName = user.Username
			}
			userDetails = append(userDetails, fmt.Sprintf("**%s** - ❌ Error: %s", userName, err.Error()))
			continue
		}

		if monitoringData == nil {
			logger.Warn(logger.LogData{
				"action":  "monitoring_status_command",
				"message": "No monitoring data found for user",
				"user_id": userID,
			})
			// Get user name for no monitoring data message
			user, err := s.User(userID)
			userName := userID // fallback to user ID if we can't get the user
			if err == nil && user != nil {
				userName = user.Username
			}
			userDetails = append(userDetails, fmt.Sprintf("**%s** - ⚠️ No monitoring data", userName))
			continue
		}

		// Get user's tasks
		tasks, err := db.GetTasksForUser(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "monitoring_status_command",
				"message": "Failed to get tasks for user",
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

		// Build user detail
		userDetail := fmt.Sprintf("**%s**", userName)

		// Add scenario information
		scenarios := monitoringData.GetScenarios()
		if len(scenarios) > 0 {
			scenarioNames := make([]string, len(scenarios))
			for i, scenario := range scenarios {
				scenarioNames[i] = string(scenario)
			}
			userDetail += fmt.Sprintf(" | Scenarios: `%s`", fmt.Sprintf("%v", scenarioNames))
		} else {
			userDetail += " | ⚠️ No active scenarios"
		}

		// Add expiration information
		if monitoringData.ExpiresAt > 0 {
			expirationTime := time.Unix(monitoringData.ExpiresAt, 0)
			now := time.Now()

			if expirationTime.After(now) {
				// Future expiration
				timeUntilExpiry := expirationTime.Sub(now)
				userDetail += fmt.Sprintf(" | Expires: `%s` (%s)",
					expirationTime.Format("2006-01-02 15:04:05"),
					formatDuration(timeUntilExpiry))
				activeUsers++
			} else {
				// Past expiration
				timeSinceExpiry := now.Sub(expirationTime)
				userDetail += fmt.Sprintf(" | ⚠️ Expired: `%s` (%s ago)",
					expirationTime.Format("2006-01-02 15:04:05"),
					formatDuration(timeSinceExpiry))
				expiredUsers++
			}
		} else {
			userDetail += " | Runs until task completion"
			activeUsers++
		}

		// Add task information
		if err == nil && len(tasks) > 0 {
			taskDetails := []string{}
			for _, task := range tasks {
				scheduledTime := time.Unix(task.ScheduledTime, 0)
				timeUntilTask := time.Until(scheduledTime)
				scenarioLabel := task.Scenario
				if scenarioLabel == "" {
					scenarioLabel = "unspecified"
				}

				if timeUntilTask > 0 {
					taskDetails = append(taskDetails, fmt.Sprintf("`%s` [%s] (in %s)",
						string(task.FunctionName), scenarioLabel, formatDuration(timeUntilTask)))
				} else {
					taskDetails = append(taskDetails, fmt.Sprintf("`%s` [%s] (overdue by %s)",
						string(task.FunctionName), scenarioLabel, formatDuration(-timeUntilTask)))
				}
			}
			userDetail += fmt.Sprintf(" | Tasks: %s", fmt.Sprintf("%v", taskDetails))
		} else {
			userDetail += " | No tasks"
		}

		userDetails = append(userDetails, userDetail)
	}

	// Create final response
	response := "📊 **All Monitored Users**\n\n"
	response += fmt.Sprintf("**Total Users:** %d\n", len(trackedUsers))
	response += fmt.Sprintf("**Active:** %d\n", activeUsers)
	if expiredUsers > 0 {
		response += fmt.Sprintf("**Expired:** %d\n\n", expiredUsers)
	} else {
		response += "\n"
	}

	// Add detailed user information
	if len(userDetails) > 0 {
		response += "**👥 User Details:**\n"
		for _, detail := range userDetails {
			response += detail + "\n"
		}
	}

	FollowUpMessage(s, i, response, true)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// GetMonitoringStatusCommandDefinition returns the monitoring-status command definition
func GetMonitoringStatusCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "monitoring-status",
		Description: "Shows all users with their monitoring scenarios, expiration times, and matching events",
	}
}
