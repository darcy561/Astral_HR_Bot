package monitoring

import (
	"astralHRBot/db"
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

//handlers

func (t *tracker) handleMessageCreate(m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	ctx := context.Background()

	// Update channel activity for all active scenarios
	// This ensures most active channel is tracked per scenario
	userMonitoring, err := db.GetUserMonitoring(ctx, m.Author.ID)
	if err == nil && userMonitoring != nil {
		for scenario := range userMonitoring.Scenarios {
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

	// Update analytics for all scenarios that track message creation
	t.updateAnalyticsForAction(m.Author.ID, models.ActionMessageCreate, "messages", 1)

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

	// Update analytics for all scenarios that track message edits
	t.updateAnalyticsForAction(m.Author.ID, models.ActionMessageEdit, "message_edits", 1)
}

func (t *tracker) handleMessageDelete(m *discordgo.MessageDelete) {
	if !t.isTracked(m.Author.ID, models.ActionMessageDelete) {
		return
	}

	// Update analytics for all scenarios that track message deletes
	t.updateAnalyticsForAction(m.Author.ID, models.ActionMessageDelete, "message_deletes", 1)
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

	// Get all tasks from the queue
	allTasks, err := db.FetchLatestTasks(ctx)
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

	for _, task := range allTasks {
		// Check if this task is for the user and scenario using the new generic methods
		if !task.IsForUser(userID) || !task.IsForScenario(scenarioStr) {
			continue
		}

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

	// Get all tasks from the queue
	allTasks, err := db.FetchLatestTasks(ctx)
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
