package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type tracker struct {
	trackedUsers map[string]*models.UserMonitoring
	eventChan    chan any
	mu           sync.RWMutex
}

var mon *tracker
var readyChan = make(chan struct{})

func Start() {
	mon = &tracker{
		trackedUsers: make(map[string]*models.UserMonitoring),
		eventChan:    make(chan any),
	}

	users, err := db.GetTrackedUsers(context.Background())
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "monitoring_startup",
			"message": "Failed to get tracked users",
			"error":   err.Error(),
		})
		return
	}

	// Clean up expired monitoring on startup
	for _, id := range users {
		monitoringData, err := db.GetUserMonitoring(context.Background(), id)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "monitoring_startup",
				"message": "Failed to get monitoring data for user",
				"error":   err.Error(),
				"user_id": id,
			})
			continue
		}

		if monitoringData == nil || monitoringData.IsExpired() {
			logger.Info(logger.LogData{
				"action":  "monitoring_startup",
				"message": "Removing expired monitoring",
				"user_id": id,
			})
			err := db.RemoveTrackedUser(context.Background(), id)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "monitoring_startup",
					"message": "Failed to remove expired monitoring",
					"error":   err.Error(),
					"user_id": id,
				})
			}
			continue
		}

		mon.trackedUsers[id] = monitoringData

		// Recreate tasks for this user's monitoring scenarios
		err = RecreateTasksForUser(id, monitoringData)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "monitoring_startup",
				"message": "Failed to recreate tasks for user",
				"error":   err.Error(),
				"user_id": id,
			})
		}
	}

	logger.Info(logger.LogData{
		"action":        "monitoring_startup",
		"message":       "Starting monitoring system",
		"tracked_users": len(mon.trackedUsers),
	})

	go mon.run()
	close(readyChan)
}

// WaitForReady blocks until the monitoring system is ready
func WaitForReady() {
	<-readyChan
}

func (t *tracker) run() {
	logger.Info(logger.LogData{
		"action":  "monitoring_worker",
		"message": "Monitoring worker started",
	})
	for raw := range t.eventChan {
		logger.Debug(logger.LogData{
			"action":  "monitoring_event",
			"message": "Received monitoring event",
			"type":    fmt.Sprintf("%T", raw),
		})
		switch evt := raw.(type) {
		case *discordgo.MessageCreate:
			t.handleMessageCreate(evt)
		case *discordgo.VoiceStateUpdate:
			t.handleVoiceState(evt)
		case *discordgo.InviteCreate:
			t.handleInviteCreate(evt)
		}
	}
}

func (t *tracker) isTracked(userID string, action models.MonitoringAction) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if user, exists := t.trackedUsers[userID]; exists {
		return user.ShouldTrackAction(action)
	}
	return false
}

// Helper function to update analytics for all active scenarios that track a specific action
func (t *tracker) updateAnalyticsForAction(userID string, action models.MonitoringAction, field string, amount int) {
	ctx := context.Background()

	// Get user's active scenarios
	userMonitoring, err := db.GetUserMonitoring(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "update_analytics_for_action",
			"message": "failed to get user monitoring data",
			"error":   err.Error(),
		})
		return
	}

	if userMonitoring == nil {
		return
	}

	// Update analytics for each active scenario that tracks this action
	for scenario := range userMonitoring.Scenarios {
		// Check if this scenario tracks the action
		actions, exists := models.ScenarioConfig[scenario]
		if !exists {
			continue
		}

		tracksAction := false
		for _, scenarioAction := range actions {
			if scenarioAction == action {
				tracksAction = true
				break
			}
		}

		if tracksAction {
			key := fmt.Sprintf("user:%s:analytics:%s", userID, scenario)
			err := db.IncreaseAttributeCount(ctx, key, field, amount)
			if err != nil {
				logger.Error(logger.LogData{
					"action":   "update_analytics_for_action",
					"message":  "failed to update analytics for scenario",
					"error":    err.Error(),
					"scenario": string(scenario),
					"field":    field,
				})
			}
		}
	}
}

// updateAnalyticsForActionInChannel updates analytics only for scenarios that
// both track the given action AND allow the provided channel via
// models.ScenarioChannelEnvFilter. If a scenario has no channel filter, it is allowed.
func (t *tracker) updateAnalyticsForActionInChannel(userID string, channelID string, action models.MonitoringAction, field string, amount int) {
	ctx := context.Background()

	userMonitoring, err := db.GetUserMonitoring(ctx, userID)
	if err != nil || userMonitoring == nil {
		return
	}

	for scenario := range userMonitoring.Scenarios {
		// Check action is tracked by scenario
		actions, exists := models.ScenarioConfig[scenario]
		if !exists {
			continue
		}
		tracksAction := false
		for _, scenarioAction := range actions {
			if scenarioAction == action {
				tracksAction = true
				break
			}
		}
		if !tracksAction {
			continue
		}

		// Channel allow-list check
		if !isChannelAllowedForScenario(scenario, channelID) {
			continue
		}

		key := fmt.Sprintf("user:%s:analytics:%s", userID, scenario)
		err := db.IncreaseAttributeCount(ctx, key, field, amount)
		if err != nil {
			logger.Error(logger.LogData{
				"action":   "update_analytics_for_action_in_channel",
				"message":  "failed to update analytics for scenario",
				"error":    err.Error(),
				"scenario": string(scenario),
				"field":    field,
			})
		}
	}
}

//handlers

func (t *tracker) handleMessageCreate(m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	ctx := context.Background()

	// Update channel activity for all active scenarios, respecting channel filters
	userMonitoring, err := db.GetUserMonitoring(ctx, m.Author.ID)
	if err == nil && userMonitoring != nil {
		for scenario := range userMonitoring.Scenarios {
			// Only count channel usage if scenario allows this channel (or has no filter)
			if !isChannelAllowedForScenario(scenario, m.ChannelID) {
				continue
			}
			err := db.IncreaseChannelCount(ctx, m.Author.ID, m.ChannelID, string(scenario))
			if err != nil {
				logger.Error(logger.LogData{
					"action":   "increase_channel_count",
					"message":  "failed to increase channel count for scenario",
					"error":    err.Error(),
					"scenario": string(scenario),
				})
			}
		}
	}

	// Only process analytics if user is being tracked for message creation
	if !t.isTracked(m.Author.ID, models.ActionMessageCreate) {
		return
	}

	logger.Debug(logger.LogData{
		"action":  "handle_message_create",
		"message": "Processing message for tracked user",
		"user_id": m.Author.ID,
	})

	// Update analytics only for scenarios that track message creation AND allow this channel
	t.updateAnalyticsForActionInChannel(m.Author.ID, m.ChannelID, models.ActionMessageCreate, "messages", 1)

	logger.Debug(logger.LogData{
		"action":     "handle_message_create",
		"message":    "Message processed successfully",
		"user_id":    m.Author.ID,
		"channel_id": m.ChannelID,
	})
}

func (t *tracker) handleMessageEdit(m *discordgo.MessageUpdate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	if !t.isTracked(m.Author.ID, models.ActionMessageEdit) {
		return
	}

	// Update analytics only for scenarios that track message edits AND allow this channel
	t.updateAnalyticsForActionInChannel(m.Author.ID, m.ChannelID, models.ActionMessageEdit, "message_edits", 1)
}

func (t *tracker) handleMessageDelete(m *discordgo.MessageDelete) {
	if !t.isTracked(m.Author.ID, models.ActionMessageDelete) {
		return
	}

	// Update analytics only for scenarios that track message deletes AND allow this channel
	t.updateAnalyticsForActionInChannel(m.Author.ID, m.ChannelID, models.ActionMessageDelete, "message_deletes", 1)
}

func (t *tracker) handleVoiceState(v *discordgo.VoiceStateUpdate) {
	// Handle voice join
	if v.BeforeUpdate == nil && v.ChannelID != "" {
		if !t.isTracked(v.UserID, models.ActionVoiceJoin) {
			return
		}

		logger.Debug(logger.LogData{
			"action":     "handle_voice_join",
			"message":    "User joined voice channel",
			"user_id":    v.UserID,
			"channel_id": v.ChannelID,
		})

		// Update analytics for all scenarios that track voice joins
		t.updateAnalyticsForAction(v.UserID, models.ActionVoiceJoin, "voice_joins", 1)
		return
	}

	// Handle voice leave
	if v.BeforeUpdate != nil && v.ChannelID == "" {
		if !t.isTracked(v.UserID, models.ActionVoiceLeave) {
			return
		}

		// Update analytics for all scenarios that track voice leaves
		t.updateAnalyticsForAction(v.UserID, models.ActionVoiceLeave, "voice_leaves", 1)
	}
}

func (t *tracker) handleInviteCreate(i *discordgo.InviteCreate) {
	if i.Inviter == nil {
		return
	}

	if !t.isTracked(i.Inviter.ID, models.ActionInviteCreate) {
		return
	}

	// Update analytics for all scenarios that track invite creation
	t.updateAnalyticsForAction(i.Inviter.ID, models.ActionInviteCreate, "invites", 1)

	logger.Debug(logger.LogData{
		"action":  "handle_invite_create",
		"message": "invite created",
		"user_id": i.Inviter.ID,
	})
}

func (t *tracker) handleReactionAdd(r *discordgo.MessageReactionAdd) {
	if !t.isTracked(r.UserID, models.ActionReactionAdd) {
		return
	}

	// Update analytics for all scenarios that track reaction adds
	t.updateAnalyticsForAction(r.UserID, models.ActionReactionAdd, "reactions_added", 1)
}

func (t *tracker) handleReactionRemove(r *discordgo.MessageReactionRemove) {
	if !t.isTracked(r.UserID, models.ActionReactionRemove) {
		return
	}

	// Update analytics for all scenarios that track reaction removes
	t.updateAnalyticsForAction(r.UserID, models.ActionReactionRemove, "reactions_removed", 1)
}

func AddUserTracking(userID string, scenario models.MonitoringScenario, trackingDuration time.Duration) {
	if mon == nil {
		logger.Error(logger.LogData{
			"action":  "add_user_tracking",
			"message": "Monitoring system not initialized",
			"user_id": userID,
		})
		return
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()

	userMonitoring, exists := mon.trackedUsers[userID]
	if !exists {
		userMonitoring = models.NewUserMonitoring(userID)
		mon.trackedUsers[userID] = userMonitoring
	}

	userMonitoring.AddScenario(scenario)
	userMonitoring.SetExpiration(trackingDuration)

	// Save to Redis
	err := db.SaveUserMonitoring(context.Background(), userMonitoring)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "add_user_tracking",
			"message": "failed to save user monitoring",
			"error":   err.Error(),
		})
		return
	}

	logger.Info(logger.LogData{
		"action":   "add_user_tracking",
		"message":  "Successfully added monitoring scenario for user",
		"user_id":  userID,
		"scenario": scenario,
		"duration": trackingDuration.String(),
	})
}

func GetTrackedUsers() []string {
	if mon == nil {
		return nil
	}
	mon.mu.RLock()
	defer mon.mu.RUnlock()
	ids := make([]string, 0, len(mon.trackedUsers))
	for id := range mon.trackedUsers {
		ids = append(ids, id)
	}
	return ids
}

func GetUserMonitoringScenarios(userID string) []models.MonitoringScenario {
	if mon == nil {
		return nil
	}
	mon.mu.RLock()
	defer mon.mu.RUnlock()
	if userMonitoring, exists := mon.trackedUsers[userID]; exists {
		return userMonitoring.GetScenarios()
	}
	return nil
}

// GetUserMonitoringStatus returns the current monitoring status for a user
func GetUserMonitoringStatus(userID string) (*models.UserMonitoring, error) {
	if mon == nil {
		return nil, fmt.Errorf("monitoring system not initialized")
	}

	mon.mu.RLock()
	defer mon.mu.RUnlock()

	if userMonitoring, exists := mon.trackedUsers[userID]; exists {
		return userMonitoring, nil
	}

	return nil, nil
}

// IsUserMonitored checks if a user is currently being monitored
func IsUserMonitored(userID string) bool {
	if mon == nil {
		return false
	}

	mon.mu.RLock()
	defer mon.mu.RUnlock()

	_, exists := mon.trackedUsers[userID]
	return exists
}

// GetActiveMonitoringScenarios returns all active monitoring scenarios for a user
func GetActiveMonitoringScenarios(userID string) ([]models.MonitoringScenario, error) {
	if mon == nil {
		return nil, fmt.Errorf("monitoring system not initialized")
	}

	mon.mu.RLock()
	defer mon.mu.RUnlock()

	if userMonitoring, exists := mon.trackedUsers[userID]; exists {
		return userMonitoring.GetScenarios(), nil
	}

	return []models.MonitoringScenario{}, nil
}

// AddScenario adds a monitoring scenario to a user
func AddScenario(userID string, scenario models.MonitoringScenario) {
	if mon == nil {
		return
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()

	userMonitoring, exists := mon.trackedUsers[userID]
	if !exists {
		userMonitoring = models.NewUserMonitoring(userID)
		mon.trackedUsers[userID] = userMonitoring
	}

	userMonitoring.AddScenario(scenario)
	db.SaveUserMonitoring(context.Background(), userMonitoring)

}

// RemoveScenario removes a specific monitoring scenario from a user
func RemoveScenario(userID string, scenario models.MonitoringScenario) error {
	if mon == nil {
		return fmt.Errorf("monitoring system not initialized")
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()

	userMonitoring, exists := mon.trackedUsers[userID]
	if !exists {
		return fmt.Errorf("user %s is not being monitored", userID)
	}

	// Check if the scenario exists for this user
	scenarios := userMonitoring.GetScenarios()
	scenarioExists := false
	for _, s := range scenarios {
		if s == scenario {
			scenarioExists = true
			break
		}
	}

	if !scenarioExists {
		return fmt.Errorf("scenario %s is not active for user %s", scenario, userID)
	}

	// Remove the scenario
	userMonitoring.RemoveScenario(scenario)

	// Remove associated tasks for this scenario
	err := RemoveTasksForScenario(userID, scenario)
	if err != nil {
		logger.Error(logger.LogData{
			"action":   "remove_scenario",
			"message":  "failed to remove tasks for scenario",
			"error":    err.Error(),
			"user_id":  userID,
			"scenario": scenario,
		})
		// Continue with scenario removal even if task removal fails
	}

	// If no more scenarios, remove user completely and all their tasks
	if len(userMonitoring.Scenarios) == 0 {
		// Remove all remaining tasks for this user
		err := RemoveAllTasksForUser(userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "remove_scenario",
				"message": "failed to remove all tasks for user",
				"error":   err.Error(),
				"user_id": userID,
			})
			// Continue with user removal even if task removal fails
		}

		delete(mon.trackedUsers, userID)
		err = db.RemoveTrackedUser(context.Background(), userID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "remove_scenario",
				"message": "failed to remove user tracking from database",
				"error":   err.Error(),
				"user_id": userID,
			})
			return fmt.Errorf("failed to remove user tracking from database: %w", err)
		}

		logger.Info(logger.LogData{
			"action":   "remove_scenario",
			"message":  "Removed last scenario, user tracking stopped",
			"user_id":  userID,
			"scenario": scenario,
		})
	} else {
		// Save updated monitoring scenarios
		err := db.SaveUserMonitoring(context.Background(), userMonitoring)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "remove_scenario",
				"message": "failed to save updated monitoring scenarios",
				"error":   err.Error(),
				"user_id": userID,
			})
			return fmt.Errorf("failed to save updated monitoring scenarios: %w", err)
		}

		logger.Info(logger.LogData{
			"action":   "remove_scenario",
			"message":  "Successfully removed monitoring scenario",
			"user_id":  userID,
			"scenario": scenario,
		})
	}

	return nil
}

// RemoveAllScenarios removes all active monitoring scenarios for a user.
// This will also remove all associated tasks for each scenario, and if the
// user has no scenarios left it will fully clean up their monitoring state.
func RemoveAllScenarios(userID string) error {
	if mon == nil {
		return fmt.Errorf("monitoring system not initialized")
	}

	// Take a snapshot of current scenarios under read lock
	mon.mu.RLock()
	var scenarios []models.MonitoringScenario
	if userMonitoring, exists := mon.trackedUsers[userID]; exists {
		scenarios = userMonitoring.GetScenarios()
	}
	mon.mu.RUnlock()

	if len(scenarios) == 0 {
		return nil
	}

	var firstErr error
	for _, sc := range scenarios {
		if err := RemoveScenario(userID, sc); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// RemoveTasksForScenario removes all tasks associated with a specific user and scenario
func RemoveTasksForScenario(userID string, scenario models.MonitoringScenario) error {
	ctx := context.Background()

	// Get all tasks from the queue (including future tasks)
	allTasks, err := db.FetchAllTasks(ctx)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "remove_tasks_for_scenario",
			"message": "failed to fetch tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	// Find and remove tasks for this user and scenario
	tasksRemoved := 0
	scenarioStr := string(scenario)

	logger.Debug(logger.LogData{
		"action":      "remove_tasks_for_scenario",
		"message":     "Starting task removal for scenario",
		"user_id":     userID,
		"scenario":    scenarioStr,
		"total_tasks": len(allTasks),
	})

	for _, task := range allTasks {
		// Check if this task is for the user and scenario using the new generic methods
		if !task.IsForUser(userID) || !task.IsForScenario(scenarioStr) {
			continue
		}

		logger.Debug(logger.LogData{
			"action":   "remove_tasks_for_scenario",
			"message":  "Found task to remove for scenario",
			"task_id":  task.TaskID,
			"user_id":  userID,
			"scenario": scenarioStr,
			"function": string(task.FunctionName),
		})

		// Remove the task
		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "remove_tasks_for_scenario",
				"message": "failed to delete task",
				"error":   err.Error(),
				"task_id": task.TaskID,
				"user_id": userID,
			})
			continue
		}

		tasksRemoved++
		logger.Info(logger.LogData{
			"action":   "remove_tasks_for_scenario",
			"message":  "Removed task for scenario",
			"task_id":  task.TaskID,
			"user_id":  userID,
			"scenario": scenario,
		})
	}

	logger.Info(logger.LogData{
		"action":        "remove_tasks_for_scenario",
		"message":       "Completed task removal for scenario",
		"user_id":       userID,
		"scenario":      scenario,
		"tasks_removed": tasksRemoved,
	})

	return nil
}

// RemoveAllTasksForUser removes all tasks for a specific user regardless of scenario
func RemoveAllTasksForUser(userID string) error {
	ctx := context.Background()

	// Get all tasks from the queue (including future tasks)
	allTasks, err := db.FetchAllTasks(ctx)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "remove_all_tasks_for_user",
			"message": "failed to fetch tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	// Find and remove all tasks for this user
	tasksRemoved := 0
	for _, task := range allTasks {
		// Check if this task is for the user
		if !task.IsForUser(userID) {
			continue
		}

		// Remove the task
		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "remove_all_tasks_for_user",
				"message": "failed to delete task",
				"error":   err.Error(),
				"task_id": task.TaskID,
				"user_id": userID,
			})
			continue
		}

		tasksRemoved++
		logger.Info(logger.LogData{
			"action":   "remove_all_tasks_for_user",
			"message":  "Removed task for user",
			"task_id":  task.TaskID,
			"user_id":  userID,
			"scenario": task.Scenario,
		})
	}

	logger.Info(logger.LogData{
		"action":        "remove_all_tasks_for_user",
		"message":       "Completed task removal for user",
		"user_id":       userID,
		"tasks_removed": tasksRemoved,
	})

	return nil
}

// RecreateTasksForUser recreates tasks for a user's monitoring scenarios
// This can be called during startup or manually via command
func RecreateTasksForUser(userID string, monitoringData *models.UserMonitoring) error {
	ctx := context.Background()

	// Check if user already has tasks
	existingTasks, err := db.GetTasksForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get existing tasks: %w", err)
	}

	// If user already has tasks, don't recreate them
	if len(existingTasks) > 0 {
		logger.Debug(logger.LogData{
			"action":     "recreate_tasks_for_user",
			"message":    "User already has tasks, skipping recreation",
			"user_id":    userID,
			"task_count": len(existingTasks),
		})
		return nil
	}

	// Recreate tasks for each scenario
	scenariosRemoved := 0
	for scenario := range monitoringData.Scenarios {
		logger.Debug(logger.LogData{
			"action":   "recreate_tasks_for_user",
			"message":  "Processing scenario for task recreation",
			"user_id":  userID,
			"scenario": scenario,
		})

		err := recreateTaskForScenario(userID, scenario, monitoringData)
		if err != nil {
			logger.Error(logger.LogData{
				"action":   "recreate_tasks_for_user",
				"message":  "Failed to recreate task for scenario",
				"error":    err.Error(),
				"user_id":  userID,
				"scenario": scenario,
			})
			// Continue with other scenarios even if one fails
		} else {
			// Check if scenario was removed (expired)
			updatedMonitoringData, err := db.GetUserMonitoring(ctx, userID)
			if err == nil && updatedMonitoringData != nil {
				if _, exists := updatedMonitoringData.Scenarios[scenario]; !exists {
					scenariosRemoved++
					logger.Info(logger.LogData{
						"action":   "recreate_tasks_for_user",
						"message":  "Scenario was removed during recreation (expired)",
						"user_id":  userID,
						"scenario": scenario,
					})
				}
			}
		}
	}

	if scenariosRemoved > 0 {
		logger.Info(logger.LogData{
			"action":            "recreate_tasks_for_user",
			"message":           "Completed task recreation with expired scenarios removed",
			"user_id":           userID,
			"scenarios_removed": scenariosRemoved,
		})
	}

	return nil
}

// recreateTaskForScenario recreates a task for a specific scenario
func recreateTaskForScenario(userID string, scenario models.MonitoringScenario, monitoringData *models.UserMonitoring) error {
	ctx := context.Background()

	// Get task functions for this scenario
	taskFunctions := models.GetTaskFunctionsForScenario(scenario)
	logger.Debug(logger.LogData{
		"action":         "recreate_task_for_scenario",
		"message":        "Retrieved task functions for scenario",
		"user_id":        userID,
		"scenario":       scenario,
		"task_functions": taskFunctions,
	})

	if len(taskFunctions) == 0 {
		logger.Debug(logger.LogData{
			"action":   "recreate_task_for_scenario",
			"message":  "No task functions for scenario",
			"user_id":  userID,
			"scenario": scenario,
		})
		return nil
	}

	// Calculate remaining time until monitoring expires
	var scheduledTime int64
	now := time.Now().Unix()

	if monitoringData.ExpiresAt > 0 {
		// Use the original expiration time, but ensure it's not in the past
		if monitoringData.ExpiresAt > now {
			scheduledTime = monitoringData.ExpiresAt
		} else {
			// Original expiration is in the past, scenario should be removed
			logger.Info(logger.LogData{
				"action":           "recreate_task_for_scenario",
				"message":          "Original expiration is in the past, removing expired scenario",
				"user_id":          userID,
				"scenario":         scenario,
				"original_expires": monitoringData.ExpiresAt,
				"current_time":     now,
			})

			// Remove the expired scenario
			err := RemoveScenario(userID, scenario)
			if err != nil {
				logger.Error(logger.LogData{
					"action":   "recreate_task_for_scenario",
					"message":  "Failed to remove expired scenario",
					"error":    err.Error(),
					"user_id":  userID,
					"scenario": scenario,
				})
				return fmt.Errorf("failed to remove expired scenario: %w", err)
			}

			// No task to create for expired scenario
			return nil
		}
	} else {
		// If no expiration, use global delay settings
		var defaultDelay int64
		switch scenario {
		case models.MonitoringScenarioRecruitmentProcess:
			defaultDelay = int64(globals.GetRecruitmentCleanupDelay()) * 24 * 60 * 60 // Convert days to seconds
		case models.MonitoringScenarioNewRecruit:
			defaultDelay = int64(globals.GetNewRecruitTrackingDays()) * 24 * 60 * 60 // Convert days to seconds
		default:
			// Fallback to 7 days for unknown scenarios
			defaultDelay = 7 * 24 * 60 * 60
		}
		scheduledTime = now + defaultDelay
	}

	// Create tasks for each function
	for _, functionName := range taskFunctions {
		var task *models.Task
		var err error

		switch functionName {
		case "ProcessRecruitmentCleanup":
			params := &models.RecruitmentCleanupParams{UserID: userID}
			task, err = models.NewTaskWithScenario(
				models.TaskRecruitmentCleanup,
				params,
				scheduledTime,
				string(scenario),
			)
		case "ProcessRecruitmentReminder":
			// Use helper function to create reminder at midpoint if it's in the future
			startTime := time.Unix(monitoringData.StartedAt, 0)
			if err := CreateRecruitmentReminderAtMidpoint(ctx, userID, startTime, scenario); err != nil {
				logger.Error(logger.LogData{
					"action":  "recreate_task_for_scenario",
					"message": "Failed to create recruitment reminder",
					"error":   err.Error(),
					"user_id": userID,
				})
			}
			// No task to return since it's handled by the helper
			return nil
		case "ProcessUserCheckin":
			params := &models.UserCheckinParams{UserID: userID}
			task, err = models.NewTaskWithScenario(
				models.TaskUserCheckin,
				params,
				scheduledTime,
				string(scenario),
			)
		default:
			logger.Warn(logger.LogData{
				"action":        "recreate_task_for_scenario",
				"message":       "Unknown task function",
				"user_id":       userID,
				"scenario":      scenario,
				"function_name": functionName,
			})
			continue
		}

		if err != nil {
			return fmt.Errorf("failed to create task for function %s: %w", functionName, err)
		}

		// Save the task to Redis
		err = db.SaveTaskToRedis(ctx, *task)
		if err != nil {
			return fmt.Errorf("failed to save task to Redis: %w", err)
		}

		logger.Info(logger.LogData{
			"action":         "recreate_task_for_scenario",
			"message":        "Recreated task for scenario",
			"user_id":        userID,
			"scenario":       scenario,
			"task_id":        task.TaskID,
			"function_name":  functionName,
			"scheduled_time": time.Unix(scheduledTime, 0).Format(time.RFC3339),
		})
	}

	return nil
}

func SubmitEvent(event any) {
	if mon == nil {
		logger.Error(logger.LogData{
			"action":  "submit_event",
			"message": "Monitoring system not initialized",
		})
		return
	}
	if mon.eventChan == nil {
		logger.Error(logger.LogData{
			"action":  "submit_event",
			"message": "Event channel is nil",
		})
		return
	}

	logger.Debug(logger.LogData{
		"action":  "submit_event",
		"message": "Submitting event to monitoring",
		"type":    fmt.Sprintf("%T", event),
	})

	select {
	case mon.eventChan <- event:
		logger.Debug(logger.LogData{
			"action":  "submit_event",
			"message": "Event submitted successfully",
			"type":    fmt.Sprintf("%T", event),
		})
	default:
		logger.Error(logger.LogData{
			"action":  "submit_event",
			"message": "Event channel full - dropping event",
			"type":    fmt.Sprintf("%T", event),
		})
	}
}

func GetUserAnalytics(userID string) (models.UserAnalytics, error) {
	if mon == nil {
		return models.UserAnalytics{}, nil
	}

	ctx := context.Background()
	return db.GetUserAnalytics(ctx, userID)
}
