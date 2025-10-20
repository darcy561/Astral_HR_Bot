package commands

import (
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RebuildRecruitmentProcessScenariosCommand handles the /rebuildrecruitmentprocessscenarios slash command
func RebuildRecruitmentProcessScenariosCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "rebuild_recruitment_process_scenarios_command",
		"message": "RebuildRecruitmentProcessScenarios command executed",
		"user_id": i.Member.User.ID,
	})

	RespondToInteraction(s, i, "🔄 **Scanning forum posts for recruitment process scenarios...**", true)

	ctx := context.Background()

	// Get all threads in the guild
	threads, err := s.GuildThreadsActive(i.GuildID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "rebuild_recruitment_process_scenarios_command",
			"message": "Failed to get forum threads",
			"error":   err.Error(),
		})
		FollowUpMessage(s, i, fmt.Sprintf("Error getting forum threads: %s", err.Error()), true)
		return
	}

	// Patterns to match recruitment messages
	joinedPattern := regexp.MustCompile(`.+ Joined Recruitment`)
	rejoinedPattern := regexp.MustCompile(`.+ Rejoined Recruitment`)

	processedThreads := 0
	recreatedScenarios := 0
	skippedTagged := 0
	errors := 0
	userDetails := []string{}

	defaultDelay := globals.GetRecruitmentCleanupDelay()

	logger.Info(logger.LogData{
		"action":        "rebuild_recruitment_process_scenarios_command",
		"message":       "Starting forum scan",
		"total_threads": len(threads.Threads),
		"default_delay": defaultDelay,
	})

	for _, thread := range threads.Threads {
		processedThreads++

		logger.Debug(logger.LogData{
			"action":       "rebuild_recruitment_process_scenarios_command",
			"message":      "Processing thread",
			"thread_id":    thread.ID,
			"thread_name":  thread.Name,
			"applied_tags": thread.AppliedTags,
		})

		// Skip threads that have any tags (closed/marked threads)
		if len(thread.AppliedTags) > 0 {
			logger.Debug(logger.LogData{
				"action":       "rebuild_recruitment_process_scenarios_command",
				"message":      "Skipping thread with tags (closed/marked)",
				"thread_id":    thread.ID,
				"thread_name":  thread.Name,
				"applied_tags": thread.AppliedTags,
			})
			skippedTagged++
			continue
		}

		// Get messages in the thread
		messages, err := s.ChannelMessages(thread.ID, 100, "", "", "") // Fetch up to 100 messages
		if err != nil {
			logger.Error(logger.LogData{
				"action":    "rebuild_recruitment_process_scenarios_command",
				"message":   "Failed to get thread messages",
				"error":     err.Error(),
				"thread_id": thread.ID,
			})
			errors++
			continue
		}

		// Look for the most recent recruitment message
		var recruitmentMessage *discordgo.Message
		var messageType string

		for _, msg := range messages {
			if joinedPattern.MatchString(msg.Content) {
				recruitmentMessage = msg
				messageType = "Joined"
				break // Take the first (most recent) match
			} else if rejoinedPattern.MatchString(msg.Content) {
				recruitmentMessage = msg
				messageType = "Rejoined"
				break // Take the first (most recent) match
			}
		}

		// Skip if we don't have a recruitment message
		if recruitmentMessage == nil {
			continue
		}

		// Extract user ID from thread title
		userID := extractUserIDFromThreadTitle(thread.Name)
		if userID == "" {
			logger.Warn(logger.LogData{
				"action":      "rebuild_recruitment_process_scenarios_command",
				"message":     "Could not extract user ID from thread title",
				"thread_id":   thread.ID,
				"thread_name": thread.Name,
			})
			continue
		}

		// Get user name for display
		user, err := s.User(userID)
		userName := userID
		if err == nil && user != nil {
			userName = user.Username
		}

		// Calculate expiration time based on when the message was sent
		messageTime := recruitmentMessage.Timestamp
		now := time.Now()

		// Calculate natural expiration (message time + default delay)
		naturalExpiration := messageTime.Add(time.Duration(defaultDelay) * 24 * time.Hour)

		// If natural expiration is in the past, schedule for 1 hour, otherwise use natural expiration
		var expirationTime time.Time
		if naturalExpiration.Before(now) {
			// Natural expiration is in the past, schedule for 1 hour
			expirationTime = now.Add(1 * time.Hour)
		} else {
			// Natural expiration is in the future, use that time
			expirationTime = naturalExpiration
		}

		// Check if user already has recruitment_process scenario
		userMonitoring, err := db.GetUserMonitoring(ctx, userID)
		if err == nil && userMonitoring != nil && userMonitoring.HasScenario(models.MonitoringScenarioRecruitmentProcess) {
			logger.Debug(logger.LogData{
				"action":    "rebuild_recruitment_process_scenarios_command",
				"message":   "User already has recruitment_process scenario, skipping recreation",
				"user_id":   userID,
				"thread_id": thread.ID,
			})
			userDetails = append(userDetails, fmt.Sprintf("**%s** - Already active, skipped", userName))
			continue
		}

		// Create the monitoring scenario
		// Note: This will be handled by the monitoring system when the user gets the role

		// Create the recruitment cleanup task
		params := &models.RecruitmentCleanupParams{UserID: userID}
		scheduledTime := expirationTime.Unix()
		newTask, err := models.NewTaskWithScenario(
			models.TaskRecruitmentCleanup,
			params,
			scheduledTime,
			string(models.MonitoringScenarioRecruitmentProcess),
		)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_recruitment_process_scenarios_command",
				"message": "Failed to create recruitment cleanup task",
				"error":   err.Error(),
				"user_id": userID,
			})
			errors++
			userDetails = append(userDetails, fmt.Sprintf("**%s** - Error creating task: %s", userName, err.Error()))
			continue
		}

		err = db.SaveTaskToRedis(ctx, *newTask)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_recruitment_process_scenarios_command",
				"message": "Failed to save recruitment cleanup task to Redis",
				"error":   err.Error(),
				"user_id": userID,
			})
			errors++
			userDetails = append(userDetails, fmt.Sprintf("**%s** - Error saving task: %s", userName, err.Error()))
			continue
		}

		recreatedScenarios++
		userDetails = append(userDetails, fmt.Sprintf("**%s** - ✅ Recreated scenario (%s, expires: %s)",
			userName, messageType, expirationTime.Format("2006-01-02 15:04:05")))

		logger.Info(logger.LogData{
			"action":          "rebuild_recruitment_process_scenarios_command",
			"message":         "Successfully recreated recruitment process scenario",
			"user_id":         userID,
			"thread_id":       thread.ID,
			"message_type":    messageType,
			"expiration_time": expirationTime.Format(time.RFC3339),
		})

		// Ensure the scenario is attached to the user's monitoring data with correct window
		_ = monitoring.EnsureScenarioWindow(ctx, userID, models.MonitoringScenarioRecruitmentProcess, messageTime, expirationTime)

		// Trigger analytics rebuild for this user using scenario-based monitoring data
		eventWorker.Submit(userID, func(e eventWorker.Event) {
			logger.Info(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_recruitment_process",
				"message":  "Starting analytics rebuild for recruitment process scenario",
				"user_id":  e.UserID,
			})

			// Build a minimal monitoring record representing just this scenario and its time bounds
			um := &models.UserMonitoring{UserID: e.UserID, Scenarios: map[models.MonitoringScenario]struct{}{models.MonitoringScenarioRecruitmentProcess: {}}}
			um.SetStartTime(messageTime)
			um.SetExpiry(expirationTime)

			result, err := monitoring.RebuildUserAnalytics(e.UserID, um, s, e.TraceID)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "rebuild_analytics_for_recruitment_process",
					"message":  "Failed to rebuild analytics",
					"error":    err.Error(),
					"user_id":  e.UserID,
				})
			} else {
				logger.Info(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "rebuild_analytics_for_recruitment_process",
					"message":  "Successfully rebuilt analytics",
					"user_id":  e.UserID,
				})

				// Send results back to the invoking user
				FollowUpMessage(s, i, fmt.Sprintf(
					"📊 Analytics rebuilt for %s (recruitment process)\n- Messages: %d\n- Voice joins: %d\n- Invites: %d\n- Top channel: %s\n- Window: %s → %s",
					userName,
					result.Messages,
					result.VoiceJoins,
					result.Invites,
					result.TopChannelID,
					result.StartTime.Format("2006-01-02 15:04:05"),
					result.EndTime.Format("2006-01-02 15:04:05"),
				), true)
			}
		}, nil)
	}

	// Create final response
	response := "✅ **Recruitment Process Scenarios Rebuild Complete**\n\n"
	response += fmt.Sprintf("**Threads processed:** %d\n", processedThreads)
	response += fmt.Sprintf("**Scenarios recreated:** %d\n", recreatedScenarios)
	response += fmt.Sprintf("**Skipped (tagged/closed):** %d\n", skippedTagged)
	response += fmt.Sprintf("**Errors:** %d\n\n", errors)

	if len(userDetails) > 0 {
		response += "**📋 User Details:**\n"
		for _, detail := range userDetails {
			response += detail + "\n"
		}
	}

	FollowUpMessage(s, i, response, true)
}

// GetRebuildRecruitmentProcessScenariosCommandDefinition returns the rebuildrecruitmentprocessscenarios command definition
func GetRebuildRecruitmentProcessScenariosCommandDefinition() *discordgo.ApplicationCommand {
	adminPerm := int64(discordgo.PermissionAdministrator)
	return &discordgo.ApplicationCommand{
		Name:                     "rebuild-recruitment-scenarios",
		Description:              "Rebuilds missing recruitment process monitoring scenarios from forum posts",
		DefaultMemberPermissions: &adminPerm,
	}
}
