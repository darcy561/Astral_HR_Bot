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

//handlers

func (t *tracker) handleMessageCreate(m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	if !t.isTracked(m.Author.ID, models.ActionMessageCreate) {
		return
	}

	key := "user:" + m.Author.ID + ":monitoring"
	ctx := context.Background()

	logger.Debug(logger.LogData{
		"action":  "handle_message_create",
		"message": "Processing message for tracked user",
		"user_id": m.Author.ID,
		"key":     key,
	})

	err := db.IncreaseAttributeCount(ctx, key, "messages", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_message_count",
			"message": "failed to increase message count",
			"error":   err.Error(),
		})
		return
	}

	err = db.IncreaseChannelCount(ctx, m.Author.ID, m.ChannelID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_channel_count",
			"message": "failed to increase channel count",
			"error":   err.Error(),
		})
	}

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

	key := "user:" + m.Author.ID + ":monitoring"
	ctx := context.Background()

	err := db.IncreaseAttributeCount(ctx, key, "message_edits", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_message_edits",
			"message": "failed to increase message edits count",
			"error":   err.Error(),
		})
	}
}

func (t *tracker) handleMessageDelete(m *discordgo.MessageDelete) {
	if !t.isTracked(m.Author.ID, models.ActionMessageDelete) {
		return
	}

	key := "user:" + m.Author.ID + ":monitoring"
	ctx := context.Background()

	err := db.IncreaseAttributeCount(ctx, key, "message_deletes", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_message_deletes",
			"message": "failed to increase message deletes count",
			"error":   err.Error(),
		})
	}
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

		ctx := context.Background()
		err := db.IncreaseAttributeCount(ctx, "user:"+v.UserID+":monitoring", "voice_joins", 1)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "increase_voice_joins",
				"message": "failed to increase voice joins count",
				"error":   err.Error(),
			})
		}
		return
	}

	// Handle voice leave
	if v.BeforeUpdate != nil && v.ChannelID == "" {
		if !t.isTracked(v.UserID, models.ActionVoiceLeave) {
			return
		}

		ctx := context.Background()
		err := db.IncreaseAttributeCount(ctx, "user:"+v.UserID+":monitoring", "voice_leaves", 1)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "increase_voice_leaves",
				"message": "failed to increase voice leaves count",
				"error":   err.Error(),
			})
		}
	}
}

func (t *tracker) handleInviteCreate(i *discordgo.InviteCreate) {
	if i.Inviter == nil {
		return
	}

	if !t.isTracked(i.Inviter.ID, models.ActionInviteCreate) {
		return
	}

	key := "user:" + i.Inviter.ID + ":monitoring"
	ctx := context.Background()
	err := db.IncreaseAttributeCount(ctx, key, "invites", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_invite_count",
			"message": "failed to increase invite count",
			"error":   err.Error(),
		})
	}
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

	key := "user:" + r.UserID + ":monitoring"
	ctx := context.Background()
	err := db.IncreaseAttributeCount(ctx, key, "reactions_added", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_reactions_added",
			"message": "failed to increase reactions added count",
			"error":   err.Error(),
		})
	}
}

func (t *tracker) handleReactionRemove(r *discordgo.MessageReactionRemove) {
	if !t.isTracked(r.UserID, models.ActionReactionRemove) {
		return
	}

	key := "user:" + r.UserID + ":monitoring"
	ctx := context.Background()
	err := db.IncreaseAttributeCount(ctx, key, "reactions_removed", 1)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_reactions_removed",
			"message": "failed to increase reactions removed count",
			"error":   err.Error(),
		})
	}
}

func AddUserTracking(userID string, scenario models.MonitoringScenario, duration time.Duration) {
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
	userMonitoring.SetExpiration(duration)

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
		"duration": duration.String(),
	})
}

func RemoveUserTracking(userID string, scenario models.MonitoringScenario) {
	if mon == nil {
		return
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()

	if userMonitoring, exists := mon.trackedUsers[userID]; exists {
		userMonitoring.RemoveScenario(scenario)

		// If no more scenarios, remove user completely
		if len(userMonitoring.Scenarios) == 0 {
			delete(mon.trackedUsers, userID)
			err := db.RemoveTrackedUser(context.Background(), userID)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "remove_user_tracking",
					"message": "failed to remove user tracking",
					"error":   err.Error(),
				})
			}
		} else {
			// Save updated monitoring scenarios
			err := db.SaveUserMonitoring(context.Background(), userMonitoring)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "remove_user_tracking",
					"message": "failed to save updated monitoring scenarios",
					"error":   err.Error(),
				})
			}
		}
	}
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
