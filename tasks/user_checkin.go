package tasks

import (
	"astralHRBot/bot"
	"astralHRBot/channels"
	"astralHRBot/db"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// ProcessUserCheckin handles the user checkin task
func ProcessUserCheckin(task models.Task) {
	params, err := task.GetParams()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "process_user_checkin",
			"message": "Failed to get params",
			"error":   err.Error(),
		})
		return
	}

	parms := params.(*models.UserCheckinParams)
	fmt.Println("Processing user checkin for user", parms.UserID)

	eventWorker.Submit(parms.UserID, func(e eventWorker.Event) {
		ctx := context.Background()

		// Get user info from Discord
		member, err := bot.Discord.GuildMember(bot.Discord.State.Guilds[0].ID, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to get member from Discord",
				"error":    err.Error(),
			})
			return
		}

		displayName := member.Nick
		if displayName == "" {
			displayName = member.User.GlobalName
			if displayName == "" {
				displayName = member.User.Username
			}
		}

		// Get analytics for the new_recruit scenario
		analyticsKey := fmt.Sprintf("user:%s:analytics:new_recruit", e.UserID)
		fields, err := db.GetRedisClient().HGetAll(ctx, analyticsKey).Result()
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to get user analytics for new_recruit scenario",
				"error":    err.Error(),
			})
			return
		}

		// Parse analytics fields
		messages := 0
		voiceJoins := 0
		invites := 0
		topChannelID := ""

		if msgStr, exists := fields["messages"]; exists {
			if msg, err := strconv.Atoi(msgStr); err == nil {
				messages = msg
			}
		}
		if voiceStr, exists := fields["voice_joins"]; exists {
			if voice, err := strconv.Atoi(voiceStr); err == nil {
				voiceJoins = voice
			}
		}
		if inviteStr, exists := fields["invites"]; exists {
			if invite, err := strconv.Atoi(inviteStr); err == nil {
				invites = invite
			}
		}

		// Get the top channel from the new_recruit scenario sorted set
		channelsKey := fmt.Sprintf("user:%s:channels:new_recruit", e.UserID)
		topChan, err := db.GetRedisClient().ZRevRangeWithScores(ctx, channelsKey, 0, 0).Result()
		if err == nil && len(topChan) > 0 {
			topChannelID = topChan[0].Member.(string)
		}

		logger.Debug(logger.LogData{
			"trace_id": e.TraceID,
			"action":   "process_user_checkin",
			"message":  "Retrieved new_recruit scenario analytics",
			"analytics": map[string]interface{}{
				"messages":       messages,
				"voice_joins":    voiceJoins,
				"invites":        invites,
				"top_channel_id": topChannelID,
			},
		})

		embededMessage := discordgo.MessageEmbed{
			Title:       fmt.Sprintf("%s's First Week Analytics", displayName),
			Description: fmt.Sprintf("Here's how %s has been engaging with our community in their first week:", displayName),
			Color:       0x000000,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📝 Messages Sent",
					Value:  fmt.Sprintf("%d", messages),
					Inline: true,
				},
				{
					Name:   "🎙️ Voice Joins",
					Value:  fmt.Sprintf("%d", voiceJoins),
					Inline: true,
				},
				{
					Name:   "📨 Invites Created",
					Value:  fmt.Sprintf("%d", invites),
					Inline: true,
				},
				{
					Name: "📌 Most Active Channel",
					Value: func() string {
						if topChannelID != "" {
							return fmt.Sprintf("<#%s>", topChannelID)
						}
						return "No channel activity recorded"
					}(),
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "User activity tracker",
			},
		}

		// Send to recruitment hub
		helper.SendChannelEmbed(bot.Discord, channels.GetRecruitmentHub(), &embededMessage, e)

		// Find and handle the recruitment thread
		rtm := helper.NewRecruitmentThreadManager(bot.Discord, e, e.UserID)
		if !rtm.HasThread() {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to find recruitment thread",
				"user_id":  e.UserID,
			})
			return
		}

		// Reopen thread, send message, then re-archive
		rtm.ReopenThread()
		rtm.SendMessageEmbed(&embededMessage)
		rtm.CloseThread("")

		err = db.DeleteTaskFromRedis(ctx, task.TaskID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "process_user_checkin",
				"message":  "Failed to delete task from redis",
				"error":    err.Error(),
			})
			return
		}
		monitoring.RemoveScenario(e.UserID, models.MonitoringScenarioNewRecruit)
	})
}
