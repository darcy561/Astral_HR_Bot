package commands

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// UserStatusCommand handles the /userstatus slash command
func UserStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "user_status_command",
		"message": "UserStatus command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the user ID from the command options
	userID := i.ApplicationCommandData().Options[0].UserValue(s).ID

	ctx := context.Background()

	// Get user monitoring data
	monitoring, err := db.GetUserMonitoring(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "user_status_command",
			"message": "Failed to get user monitoring",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, "Error retrieving monitoring data", true)
		return
	}

	// Get user tasks
	tasks, err := db.GetTasksForUser(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "user_status_command",
			"message": "Failed to get user tasks",
			"error":   err.Error(),
			"user_id": userID,
		})
		RespondToInteraction(s, i, "Error retrieving task data", true)
		return
	}

	// Get user name from Discord
	user, err := s.User(userID)
	userName := userID // fallback to user ID if we can't get the user
	if err == nil && user != nil {
		userName = user.Username
	}

	// Create embed response
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Status for %s", userName),
		Color:       0x00ff00, // Green color
		Timestamp:   time.Now().Format(time.RFC3339),
		Description: "Current monitoring and task status",
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add monitoring information
	if monitoring != nil && !monitoring.IsExpired() {
		scenarios := monitoring.GetScenarios()
		scenarioNames := make([]string, len(scenarios))
		for i, scenario := range scenarios {
			scenarioNames[i] = string(scenario)
		}

		startedAt := time.Unix(monitoring.StartedAt, 0).Format("2006-01-02 15:04:05")
		expiresAt := "Never"
		if monitoring.ExpiresAt > 0 {
			expiresAt = time.Unix(monitoring.ExpiresAt, 0).Format("2006-01-02 15:04:05")
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📊 Monitoring Status",
			Value:  "✅ Active",
			Inline: true,
		})

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🎯 Active Scenarios",
			Value:  fmt.Sprintf("`%s`", fmt.Sprintf("%v", scenarioNames)),
			Inline: true,
		})

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "⏰ Started At",
			Value:  fmt.Sprintf("`%s`", startedAt),
			Inline: true,
		})

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "⏳ Expires At",
			Value:  fmt.Sprintf("`%s`", expiresAt),
			Inline: true,
		})
	} else {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📊 Monitoring Status",
			Value:  "❌ Not Active",
			Inline: true,
		})
	}

	// Add task information
	if len(tasks) > 0 {
		taskInfo := ""
		for _, task := range tasks {
			scheduledTime := time.Unix(task.ScheduledTime, 0).Format("2006-01-02 15:04:05")
			status := task.Status
			scenario := task.Scenario
			if scenario == "" {
				scenario = "N/A"
			}

			taskInfo += fmt.Sprintf("**%s**\n", string(task.FunctionName))
			taskInfo += fmt.Sprintf("• Status: `%s`\n", status)
			taskInfo += fmt.Sprintf("• Scheduled: `%s`\n", scheduledTime)
			taskInfo += fmt.Sprintf("• Scenario: `%s`\n", scenario)
			taskInfo += fmt.Sprintf("• Retries: `%d`\n\n", task.Retries)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("📋 Tasks (%d)", len(tasks)),
			Value:  taskInfo,
			Inline: false,
		})
	} else {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "📋 Tasks",
			Value:  "No active tasks",
			Inline: false,
		})
	}

	// Add analytics if monitoring is active
	if monitoring != nil && !monitoring.IsExpired() {
		analytics, err := db.GetUserAnalytics(ctx, userID)
		if err == nil {
			analyticsInfo := fmt.Sprintf("• Messages: `%d`\n", analytics.Messages)
			analyticsInfo += fmt.Sprintf("• Voice Joins: `%d`\n", analytics.VoiceJoins)
			analyticsInfo += fmt.Sprintf("• Invites: `%d`\n", analytics.Invites)
			if analytics.TopChannelID != "" {
				analyticsInfo += fmt.Sprintf("• Top Channel: <#%s>\n", analytics.TopChannelID)
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "📈 Analytics",
				Value:  analyticsInfo,
				Inline: false,
			})
		}
	}

	RespondToInteractionWithEmbed(s, i, embed, true)
}

// GetUserStatusCommandDefinition returns the userstatus command definition
func GetUserStatusCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "userstatus",
		Description: "Get current monitoring and task status for a user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The user to check status for",
				Required:    true,
			},
		},
	}
}
