package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type tracker struct {
	trackedUsers map[string]struct{}
	eventChan    chan interface{}
	mu           sync.RWMutex
}

var mon *tracker
var readyChan = make(chan struct{})

func Start() {

	mon = &tracker{
		trackedUsers: make(map[string]struct{}),
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

	for _, id := range users {
		mon.trackedUsers[id] = struct{}{}
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
			t.handleVoiceJoin(evt)
		case *discordgo.InviteCreate:
			t.handleInviteCreate(evt)
		}
	}
}

func (t *tracker) isTracked(userID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.trackedUsers[userID]
	return ok
}

//handlers

func (t *tracker) handleMessageCreate(m *discordgo.MessageCreate) {
	if m.Author == nil {
		logger.Debug(logger.LogData{
			"action":  "handle_message_create",
			"message": "Skipping message - nil author",
		})
		return
	}

	if m.Author.Bot {
		logger.Debug(logger.LogData{
			"action":  "handle_message_create",
			"message": "Skipping message - bot author",
			"user_id": m.Author.ID,
		})
		return
	}

	if !t.isTracked(m.Author.ID) {
		logger.Debug(logger.LogData{
			"action":  "handle_message_create",
			"message": "Skipping message - user not tracked",
			"user_id": m.Author.ID,
		})
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

func (t *tracker) handleVoiceJoin(v *discordgo.VoiceStateUpdate) {
	if !t.isTracked(v.UserID) {
		logger.Debug(logger.LogData{
			"action":  "handle_voice_join",
			"message": "Skipping voice join - user not tracked",
			"user_id": v.UserID,
		})
		return
	}

	// User joined a voice channel
	if v.BeforeUpdate == nil && v.ChannelID != "" {
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
	}
}

func (t *tracker) handleInviteCreate(i *discordgo.InviteCreate) {
	if i.Inviter != nil && t.isTracked(i.Inviter.ID) {
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
}

func AddUserTracking(userID string) {
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

	if _, exists := mon.trackedUsers[userID]; exists {
		logger.Debug(logger.LogData{
			"action":  "add_user_tracking",
			"message": "User already being tracked",
			"user_id": userID,
		})
		return
	}

	mon.trackedUsers[userID] = struct{}{}
	err := db.AddTrackedUser(context.Background(), userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "add_user_tracking",
			"message": "failed to add user tracking",
			"error":   err.Error(),
		})
		return
	}

	logger.Info(logger.LogData{
		"action":  "add_user_tracking",
		"message": "Successfully added user to tracking",
		"user_id": userID,
	})
}

func RemoveUserTracking(userID string) {
	if mon == nil {
		return
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()
	delete(mon.trackedUsers, userID)
	err := db.RemoveTrackedUser(context.Background(), userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "remove_user_tracking",
			"message": "failed to remove user tracking",
			"error":   err.Error(),
		})
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
