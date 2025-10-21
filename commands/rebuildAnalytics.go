package commands

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RebuildAnalyticsCommand handles the /rebuildanalytics slash command
func RebuildAnalyticsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "rebuild_analytics_command",
		"message": "RebuildAnalytics command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the user ID from the command options
	userID := i.ApplicationCommandData().Options[0].UserValue(s).ID

	// Get user name for display
	user, err := s.User(userID)
	userName := userID
	if err == nil && user != nil {
		userName = user.Username
	}

	RespondToInteraction(s, i, fmt.Sprintf("🔄 **Rebuilding analytics for %s...**", userName), true)

	// Trigger analytics rebuild via event worker
	eventWorker.Submit(userID, func(e eventWorker.Event) {
		ctx := context.Background()

		logger.Info(logger.LogData{
			"trace_id": e.TraceID,
			"action":   "rebuild_analytics_for_user",
			"message":  "Starting analytics rebuild",
			"user_id":  e.UserID,
		})

		// Get user monitoring data to determine the time period
		monitoringData, err := db.GetUserMonitoring(ctx, e.UserID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_user",
				"message":  "Failed to get user monitoring data",
				"error":    err.Error(),
				"user_id":  e.UserID,
			})
			return
		}

		if monitoringData == nil {
			logger.Warn(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_user",
				"message":  "No monitoring data found for user",
				"user_id":  e.UserID,
			})
			return
		}

		// Rebuild analytics for the user
		result, err := monitoring.RebuildUserAnalytics(e.UserID, monitoringData, s, e.TraceID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_user",
				"message":  "Failed to rebuild analytics",
				"error":    err.Error(),
				"user_id":  e.UserID,
			})
		} else {
			logger.Info(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_user",
				"message":  "Successfully rebuilt analytics",
				"user_id":  e.UserID,
			})

			// Send a follow-up message to the invoker with the results
			// Note: i is captured from outer scope; safe because this runs shortly after invocation
			userDisplay := userName
			FollowUpMessage(s, i, fmt.Sprintf(
				"✅ Analytics rebuilt for %s\n- Messages: %d\n- Voice joins: %d\n- Invites: %d\n- Top channel: %s\n- Window: %s → %s",
				userDisplay,
				result.Messages,
				result.VoiceJoins,
				result.Invites,
				result.TopChannelID,
				result.StartTime.Format(time.RFC3339),
				result.EndTime.Format(time.RFC3339),
			), true)
		}
	}, nil)

	FollowUpMessage(s, i, fmt.Sprintf("✅ **Analytics rebuild triggered for %s**\n\nThis will run in the background and may take a few minutes.", userName), true)
}

// GetRebuildAnalyticsCommandDefinition returns the rebuildanalytics command definition
func GetRebuildAnalyticsCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "rebuild-analytics",
		Description: "Rebuilds analytics data for a specific user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The user to rebuild analytics for",
				Required:    true,
			},
		},
	}
}
