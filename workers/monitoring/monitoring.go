package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type tracker struct {
	trackedUsers map[string]struct{}
	eventChan    chan interface{}
	mu           sync.RWMutex
}

var mon *tracker

func Start() {

	mon = &tracker{
		trackedUsers: make(map[string]struct{}),
		eventChan:    make(chan any),
	}

	users, err := db.GetTrackedUsers(context.Background())

	if err != nil {
		return
	}

	for _, id := range users {
		mon.trackedUsers[id] = struct{}{}
	}

	go mon.run()
}

func (t *tracker) run() {
	for raw := range t.eventChan {
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
	if m.Author == nil && m.Author.Bot && !t.isTracked(m.Author.ID) {
		return
	}

	key := "user:" + m.Author.ID + ":monitoring"

	ctx := context.Background()

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
		"message":    "message created",
		"user_id":    m.Author.ID,
		"channel_id": m.ChannelID,
		"content":    m.Content,
	})

}

func (t *tracker) handleVoiceJoin(v *discordgo.VoiceStateUpdate) {

	if v.BeforeUpdate == nil && v.ChannelID != "" && t.isTracked(v.UserID) {
		ctx := context.Background()
		err := db.IncreaseAttributeCount(ctx, "user:"+v.UserID+":monitoring", "voice_time", 1)

		if err != nil {
			logger.Error(logger.LogData{
				"action":  "increase_voice_time",
				"message": "failed to increase voice time",
				"error":   err.Error(),
			})
		}
		logger.Debug(logger.LogData{
			"action":     "handle_voice_join",
			"message":    "voice join",
			"user_id":    v.UserID,
			"channel_id": v.ChannelID,
		})
	}
}

func (t *tracker) handleInviteCreate(i *discordgo.InviteCreate) {
	if i.Inviter != nil && t.isTracked(i.Inviter.ID) {
		key := "user:" + i.Inviter.ID + ":analytics"
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
		return
	}

	mon.mu.Lock()
	defer mon.mu.Unlock()
	mon.trackedUsers[userID] = struct{}{}
	err := db.AddTrackedUser(context.Background(), userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "add_user_tracking",
			"message": "failed to add user tracking",
			"error":   err.Error(),
		})
	}
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
		return
	}
	mon.eventChan <- event
}

func GetUserAnalytics(userID string) (models.UserAnalytics, error) {
	if mon == nil {
		return models.UserAnalytics{}, nil
	}

	ctx := context.Background()
	return db.GetUserAnalytics(ctx, userID)
}
