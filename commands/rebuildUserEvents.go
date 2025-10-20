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
		// Try to infer scenarios from existing tasks for this user
		existingTasks, err := db.GetTasksForUser(ctx, userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_user_events_command",
				"message": "Failed to get existing tasks while user has no monitoring",
				"error":   err.Error(),
				"user_id": userID,
			})
			RespondToInteraction(s, i, "User is not currently being monitored", true)
			return
		}

		if len(existingTasks) == 0 {
			RespondToInteraction(s, i, "User is not currently being monitored", true)
			return
		}

		// Backfill new monitoring record
		monitoringData = models.NewUserMonitoring(userID)

		scenarioMap := map[models.TaskType]models.MonitoringScenario{
			models.TaskRecruitmentCleanup: models.MonitoringScenarioRecruitmentProcess,
			models.TaskUserCheckin:        models.MonitoringScenarioNewRecruit,
		}
		for _, t := range existingTasks {
			if sc, ok := scenarioMap[t.FunctionName]; ok {
				monitoringData.AddScenario(sc)
			}
		}

		// Compute window
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

		if err := db.SaveUserMonitoring(ctx, monitoringData); err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_user_events_command",
				"message": "Failed to save inferred monitoring data",
				"error":   err.Error(),
				"user_id": userID,
			})
		}
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

	// If no scenarios are active, infer scenarios from existing tasks and backfill monitoring
	scenariosAdded := []string{}
	if len(monitoringData.Scenarios) == 0 && len(existingTasks) > 0 {
		// Map task types to scenarios
		scenarioMap := map[models.TaskType]models.MonitoringScenario{
			models.TaskRecruitmentCleanup: models.MonitoringScenarioRecruitmentProcess,
			models.TaskUserCheckin:        models.MonitoringScenarioNewRecruit,
		}

		// Determine any scenarios present based on tasks
		for _, t := range existingTasks {
			if sc, ok := scenarioMap[t.FunctionName]; ok {
				if !monitoringData.HasScenario(sc) {
					monitoringData.AddScenario(sc)
					scenariosAdded = append(scenariosAdded, string(sc))
				}
			}
		}

		// Set expiry/start if not set, based on upcoming task schedule
		if monitoringData.ExpiresAt == 0 {
			var earliest int64 = 0
			for _, t := range existingTasks {
				if earliest == 0 || (t.ScheduledTime > 0 && t.ScheduledTime < earliest) {
					earliest = t.ScheduledTime
				}
			}
			if earliest > 0 {
				monitoringData.ExpiresAt = earliest
				// Derive a reasonable start time using scenario defaults (prefer new_recruit if present)
				defaultDays := 7
				if monitoringData.HasScenario(models.MonitoringScenarioNewRecruit) {
					defaultDays = globals.GetNewRecruitTrackingDays()
				} else if monitoringData.HasScenario(models.MonitoringScenarioRecruitmentProcess) {
					defaultDays = globals.GetRecruitmentCleanupDelay()
				}
				monitoringData.StartedAt = time.Unix(earliest, 0).Add(-time.Duration(defaultDays) * 24 * time.Hour).Unix()
			}
		}

		// Persist backfilled monitoring data
		if err := db.SaveUserMonitoring(ctx, monitoringData); err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_user_events_command",
				"message": "Failed to save backfilled monitoring data",
				"error":   err.Error(),
				"user_id": userID,
			})
		}
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

	if len(scenariosAdded) > 0 {
		response += fmt.Sprintf("**Backfilled Scenarios:** `%v`\n\n", scenariosAdded)
	}

	if len(newTasks) > 0 {
		response += "**📋 New Tasks:**\n"
		for _, task := range newTasks {
			scheduledTime := time.Unix(task.ScheduledTime, 0).Format("2006-01-02 15:04:05")
			scenarioLabel := task.Scenario
			if scenarioLabel == "" {
				scenarioLabel = "unspecified"
			}
			response += fmt.Sprintf("• **%s** [%s] - Scheduled: `%s`\n", string(task.FunctionName), scenarioLabel, scheduledTime)
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
