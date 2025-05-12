package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/logger"
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
		case *discordgo.InviteCreate:
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

	err := db.IncreaseAttributeCount(context.Background(), key, "messages", 1)

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_message_count",
			"message": "failed to increase message count",
			"error":   err.Error(),
		})
		return
	}

}
