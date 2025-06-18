package handlers

import (
	"astralHRBot/handlers/middleware"
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

func ManageGuildChanges(s *discordgo.Session, event any) {
	switch evt := event.(type) {
	case *discordgo.VoiceStateUpdate:
		handleVoiceStateUpdate(s, evt)
	case *discordgo.InviteCreate:
		handleInviteCreate(s, evt)
	}
}

func handleVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	logger.Debug(logger.LogData{
		"action":     "voice_state_update",
		"message":    "Received voice state update",
		"user_id":    v.UserID,
		"channel_id": v.ChannelID,
	})

	eventWorker.Submit(v.UserID, func(e eventWorker.Event) {
		middleware.MonitorVoiceStateUpdate(s, v, e)
	}, s, v)
}

func handleInviteCreate(s *discordgo.Session, i *discordgo.InviteCreate) {
	if i.Inviter == nil {
		return
	}

	logger.Debug(logger.LogData{
		"action":     "invite_create",
		"message":    "Received invite create",
		"user_id":    i.Inviter.ID,
		"channel_id": i.ChannelID,
	})

	eventWorker.Submit(i.Inviter.ID, func(e eventWorker.Event) {
		middleware.MonitorInviteCreate(s, i, e)
	}, s, i)
}
