package commands

import (
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RebuildAllUserEventsCommand handles the /rebuildalluserevents slash command
func RebuildAllUserEventsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "rebuild_all_user_events_command",
		"message": "RebuildAllUserEvents command executed",
		"user_id": i.Member.User.ID,
	})

	ctx := context.Background()

	// Get all tracked users
	trackedUsers, err := db.GetTrackedUsers(ctx)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_all_user_events_command",
			"message": "Failed to get tracked users",
			"error":   err.Error(),
		})
		RespondToInteraction(s, i, "Error retrieving tracked users", true)
		return
	}

	if len(trackedUsers) == 0 {
		RespondToInteraction(s, i, "No users are currently being monitored", true)
		return
	}

	// Process each user
	successCount := 0
	errorCount := 0
	totalTasksRemoved := 0
	totalTasksCreated := 0
	userDetails := []string{}

	response := fmt.Sprintf("🔄 **Rebuilding events for %d monitored users...**\n\n", len(trackedUsers))

	for _, userID := range trackedUsers {
		// Get user monitoring data
		monitoringData, err := db.GetUserMonitoring(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_all_user_events_command",
				"message": "Failed to get monitoring data for user",
				"error":   err.Error(),
				"user_id": userID,
			})
			errorCount++
			continue
		}

		if monitoringData == nil {
			logger.Warn(logger.LogData{
				"action":  "rebuild_all_user_events_command",
				"message": "User has no monitoring data",
				"user_id": userID,
			})
			errorCount++
			continue
		}

		// Check existing tasks
		existingTasks, err := db.GetTasksForUser(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_all_user_events_command",
				"message": "Failed to get existing tasks for user",
				"error":   err.Error(),
				"user_id": userID,
			})
			errorCount++
			continue
		}

		// If no scenarios are active, infer scenarios from existing tasks and backfill monitoring
		if len(monitoringData.Scenarios) == 0 && len(existingTasks) > 0 {
			// Map task types to scenarios
			scenarioMap := map[models.TaskType]models.MonitoringScenario{
				models.TaskRecruitmentCleanup: models.MonitoringScenarioRecruitmentProcess,
				models.TaskUserCheckin:        models.MonitoringScenarioNewRecruit,
			}

			for _, t := range existingTasks {
				if sc, ok := scenarioMap[t.FunctionName]; ok {
					if !monitoringData.HasScenario(sc) {
						monitoringData.AddScenario(sc)
					}
				}
			}

			// Set expiry/start if not set, based on upcoming task schedule
			if monitoringData.ExpiresAt == 0 {
				var earliest int64
				for _, t := range existingTasks {
					if earliest == 0 || (t.ScheduledTime > 0 && t.ScheduledTime < earliest) {
						earliest = t.ScheduledTime
					}
				}
				if earliest > 0 {
					monitoringData.ExpiresAt = earliest
					defaultDays := 7
					if monitoringData.HasScenario(models.MonitoringScenarioNewRecruit) {
						defaultDays = globals.GetNewRecruitTrackingDays()
					} else if monitoringData.HasScenario(models.MonitoringScenarioRecruitmentProcess) {
						defaultDays = globals.GetRecruitmentCleanupDelay()
					}
					monitoringData.StartedAt = time.Unix(earliest, 0).Add(-time.Duration(defaultDays) * 24 * time.Hour).Unix()
				}
			}

			if err := db.SaveUserMonitoring(ctx, monitoringData); err != nil {
				logger.Error(logger.LogData{
					"action":  "rebuild_all_user_events_command",
					"message": "Failed to save backfilled monitoring data",
					"error":   err.Error(),
					"user_id": userID,
				})
			}
		}

		// Remove existing tasks
		tasksRemoved := 0
		for _, task := range existingTasks {
			err = db.DeleteTaskFromRedis(ctx, task.TaskID)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "rebuild_all_user_events_command",
					"message": "Failed to delete existing task",
					"error":   err.Error(),
					"task_id": task.TaskID,
					"user_id": userID,
				})
			} else {
				tasksRemoved++
			}
		}

		// Recreate tasks
		logger.Debug(logger.LogData{
			"action":    "rebuild_all_user_events_command",
			"message":   "Attempting to recreate tasks for user",
			"user_id":   userID,
			"scenarios": monitoringData.GetScenarios(),
		})

		err = monitoring.RecreateTasksForUser(userID, monitoringData)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_all_user_events_command",
				"message": "Failed to recreate tasks for user",
				"error":   err.Error(),
				"user_id": userID,
			})
			errorCount++
			continue
		}

		// Get new task count
		newTasks, err := db.GetTasksForUser(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_all_user_events_command",
				"message": "Failed to get new tasks for user",
				"error":   err.Error(),
				"user_id": userID,
			})
			errorCount++
			continue
		}

		successCount++
		totalTasksRemoved += tasksRemoved
		totalTasksCreated += len(newTasks)

		// Get updated monitoring data to check for removed scenarios
		updatedMonitoringData, err := db.GetUserMonitoring(ctx, userID)
		var activeScenarios []string
		if err == nil && updatedMonitoringData != nil {
			scenarios := updatedMonitoringData.GetScenarios()
			scenarioNames := make([]string, len(scenarios))
			for i, scenario := range scenarios {
				scenarioNames[i] = string(scenario)
			}
			activeScenarios = scenarioNames
		}

		// Collect detailed information about this user's events
		userDetail := fmt.Sprintf("**<@%s>** - Removed: %d, Created: %d", userID, tasksRemoved, len(newTasks))

		// Add scenario info
		if len(activeScenarios) > 0 {
			userDetail += fmt.Sprintf(" | Scenarios: `%s`", fmt.Sprintf("%v", activeScenarios))
		} else {
			userDetail += " | ⚠️ **All scenarios expired**"
		}

		// Add task details if any were created
		if len(newTasks) > 0 {
			taskDetails := []string{}
			for _, task := range newTasks {
				scheduledTime := time.Unix(task.ScheduledTime, 0).Format("2006-01-02 15:04:05")
				taskDetails = append(taskDetails, fmt.Sprintf("`%s` (%s)", string(task.FunctionName), scheduledTime))
			}
			userDetail += fmt.Sprintf("\n  • %s", fmt.Sprintf("%v", taskDetails))
		}

		userDetails = append(userDetails, userDetail)

		logger.Info(logger.LogData{
			"action":        "rebuild_all_user_events_command",
			"message":       "Successfully rebuilt events for user",
			"user_id":       userID,
			"tasks_removed": tasksRemoved,
			"tasks_created": len(newTasks),
		})
	}

	// Create final response
	response += "✅ **Completed rebuilding events**\n\n"
	response += fmt.Sprintf("**Users processed:** %d\n", len(trackedUsers))
	response += fmt.Sprintf("**Successful:** %d\n", successCount)
	response += fmt.Sprintf("**Errors:** %d\n", errorCount)
	response += fmt.Sprintf("**Total tasks removed:** %d\n", totalTasksRemoved)
	response += fmt.Sprintf("**Total tasks created:** %d\n\n", totalTasksCreated)

	// Add detailed user information
	if len(userDetails) > 0 {
		response += "**📋 User Details:**\n"
		for _, detail := range userDetails {
			response += detail + "\n"
		}
	}

	if errorCount > 0 {
		response += fmt.Sprintf("\n⚠️ **%d users had errors during processing**", errorCount)
	}

	RespondToInteraction(s, i, response, true)
}

// GetRebuildAllUserEventsCommandDefinition returns the rebuildalluserevents command definition
func GetRebuildAllUserEventsCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "rebuildalluserevents",
		Description: "Rebuild events/tasks for all monitored users",
	}
}
