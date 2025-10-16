package commands

import (
	"astralHRBot/channels"
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RebuildNewRecruitScenariosCommand handles the /rebuild-new-recruit-scenarios slash command
func RebuildNewRecruitScenariosCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "rebuild_new_recruit_scenarios_command",
		"message": "RebuildNewRecruitScenarios command executed",
		"user_id": i.Member.User.ID,
	})

	RespondToInteraction(s, i, "🔄 **Scanning archived forum posts for new recruit scenarios...**", true)

	ctx := context.Background()

	// Get the recruitment forum channel ID from environment variables
	forumChannelID := channels.GetRecruitmentForum()
	if forumChannelID == "" {
		logger.Error(logger.LogData{
			"action":  "rebuild_new_recruit_scenarios_command",
			"message": "Recruitment forum channel ID not found in environment variables",
		})
		FollowUpMessage(s, i, "Recruitment forum channel not configured", true)
		return
	}

	// Get the default tracking period
	defaultTrackingDays := globals.GetNewRecruitTrackingDays()
	cutoffTime := time.Now().Add(-time.Duration(defaultTrackingDays) * 24 * time.Hour)

	// Pattern to match the "Character Joined Corporation" message
	pattern := regexp.MustCompile(`Character Joined Corporation\.`)

	processedThreads := 0
	recreatedScenarios := 0
	skippedOld := 0
	errors := 0
	userDetails := []string{}

	// Function to process a single thread with cutoff check
	processThread := func(thread *discordgo.Channel, cutoffTime time.Time) bool {
		processedThreads++

		logger.Debug(logger.LogData{
			"action":       "rebuild_new_recruit_scenarios_command",
			"message":      "Processing thread",
			"thread_id":    thread.ID,
			"thread_name":  thread.Name,
			"applied_tags": thread.AppliedTags,
		})

		// Get thread messages
		messages, err := s.ChannelMessages(thread.ID, 100, "", "", "")
		if err != nil {
			logger.Error(logger.LogData{
				"action":    "rebuild_new_recruit_scenarios_command",
				"message":   "Failed to get thread messages",
				"error":     err.Error(),
				"thread_id": thread.ID,
			})
			errors++
			return true // Continue processing other threads
		}

		// Look for the "Character Joined Corporation" message
		var corporationMessage *discordgo.Message

		for _, msg := range messages {
			if pattern.MatchString(msg.Content) {
				corporationMessage = msg
				logger.Debug(logger.LogData{
					"action":      "rebuild_new_recruit_scenarios_command",
					"message":     "Found 'Character Joined Corporation' message",
					"thread_id":   thread.ID,
					"thread_name": thread.Name,
					"msg_time":    msg.Timestamp.Format(time.RFC3339),
				})
				break
			}
		}

		// Skip if we don't have the corporation message
		if corporationMessage == nil {
			logger.Debug(logger.LogData{
				"action":      "rebuild_new_recruit_scenarios_command",
				"message":     "No 'Character Joined Corporation' message found in thread",
				"thread_id":   thread.ID,
				"thread_name": thread.Name,
			})
			return true // Continue processing other threads
		}

		// Check if the message timestamp is before the cutoff time
		messageTime := corporationMessage.Timestamp
		if messageTime.Before(cutoffTime) {
			logger.Info(logger.LogData{
				"action":       "rebuild_new_recruit_scenarios_command",
				"message":      "Message timestamp is before cutoff time, stopping processing",
				"thread_id":    thread.ID,
				"thread_name":  thread.Name,
				"message_time": messageTime.Format(time.RFC3339),
				"cutoff_time":  cutoffTime.Format(time.RFC3339),
			})
			return false // Stop processing - we've reached the cutoff
		}

		// Check if the thread is tagged as "Accepted"
		isAccepted := false
		logger.Debug(logger.LogData{
			"action":       "rebuild_new_recruit_scenarios_command",
			"message":      "Checking for 'Accepted' tag",
			"thread_id":    thread.ID,
			"thread_name":  thread.Name,
			"applied_tags": thread.AppliedTags,
		})

		for _, tagID := range thread.AppliedTags {
			// Fetch the channel to get tag names
			channel, err := s.Channel(thread.ParentID)
			if err != nil {
				logger.Error(logger.LogData{
					"action":    "rebuild_new_recruit_scenarios_command",
					"message":   "Failed to get parent channel for thread",
					"error":     err.Error(),
					"thread_id": thread.ID,
				})
				return true // Continue processing other threads
			}
			for _, tag := range channel.AvailableTags {
				logger.Debug(logger.LogData{
					"action":      "rebuild_new_recruit_scenarios_command",
					"message":     "Checking tag",
					"thread_id":   thread.ID,
					"tag_id":      tagID,
					"tag_name":    tag.Name,
					"is_accepted": strings.EqualFold(tag.Name, "accepted"),
				})
				if tag.ID == tagID && strings.EqualFold(tag.Name, "accepted") {
					isAccepted = true
					logger.Debug(logger.LogData{
						"action":    "rebuild_new_recruit_scenarios_command",
						"message":   "Found 'Accepted' tag",
						"thread_id": thread.ID,
						"tag_name":  tag.Name,
					})
					break
				}
			}
			if isAccepted {
				break
			}
		}

		if !isAccepted {
			logger.Debug(logger.LogData{
				"action":       "rebuild_new_recruit_scenarios_command",
				"message":      "Thread is not tagged as 'Accepted'",
				"thread_id":    thread.ID,
				"thread_name":  thread.Name,
				"applied_tags": thread.AppliedTags,
			})
			return true // Continue processing other threads
		}

		// Extract user ID from thread title
		userID := extractUserIDFromThreadTitle(thread.Name)
		if userID == "" {
			logger.Warn(logger.LogData{
				"action":      "rebuild_new_recruit_scenarios_command",
				"message":     "Could not extract user ID from thread title",
				"thread_id":   thread.ID,
				"thread_name": thread.Name,
			})
			return true // Continue processing other threads
		}

		// Get user name for display
		user, err := s.User(userID)
		userName := userID
		if err == nil && user != nil {
			userName = user.Username
		}

		// Calculate expiration time based on when the message was sent
		expirationTime := messageTime.Add(time.Duration(defaultTrackingDays) * 24 * time.Hour)

		// Check if this would result in a task in the past
		if expirationTime.Before(time.Now()) {
			logger.Debug(logger.LogData{
				"action":          "rebuild_new_recruit_scenarios_command",
				"message":         "Thread is too old, skipping",
				"thread_id":       thread.ID,
				"thread_name":     thread.Name,
				"message_time":    messageTime.Format(time.RFC3339),
				"expiration_time": expirationTime.Format(time.RFC3339),
			})
			skippedOld++
			return true // Continue processing other threads
		}

		// Check if user already has new_recruit scenario and remove it if so
		userMonitoring, err := db.GetUserMonitoring(ctx, userID)
		if err == nil && userMonitoring != nil && userMonitoring.HasScenario(models.MonitoringScenarioNewRecruit) {
			logger.Debug(logger.LogData{
				"action":    "rebuild_new_recruit_scenarios_command",
				"message":   "User already has new_recruit scenario, removing existing scenario",
				"user_id":   userID,
				"thread_id": thread.ID,
			})

			// Remove the existing scenario to recreate it with correct timing
			monitoring.RemoveScenario(userID, models.MonitoringScenarioNewRecruit)
		}

		// Create the monitoring scenario
		monitoring.AddUserTracking(userID, models.MonitoringScenarioNewRecruit, time.Duration(defaultTrackingDays)*24*time.Hour)

		// Get the monitoring data and set the correct start time
		userMonitoring, err = db.GetUserMonitoring(ctx, userID)
		if err == nil && userMonitoring != nil {
			logger.Debug(logger.LogData{
				"action":         "rebuild_new_recruit_scenarios_command",
				"message":        "Setting correct start time",
				"user_id":        userID,
				"old_start_time": time.Unix(userMonitoring.StartedAt, 0).Format(time.RFC3339),
				"new_start_time": messageTime.Format(time.RFC3339),
			})
			userMonitoring.SetStartTime(messageTime)
			userMonitoring.SetExpiry(expirationTime)

			// Save the corrected monitoring data to the database
			logger.Debug(logger.LogData{
				"action":     "rebuild_new_recruit_scenarios_command",
				"message":    "Saving corrected monitoring data to database",
				"user_id":    userID,
				"started_at": time.Unix(userMonitoring.StartedAt, 0).Format(time.RFC3339),
				"expires_at": time.Unix(userMonitoring.ExpiresAt, 0).Format(time.RFC3339),
				"scenarios":  userMonitoring.GetScenarios(),
			})

			err = db.SaveUserMonitoring(ctx, userMonitoring)
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "rebuild_new_recruit_scenarios_command",
					"message": "Failed to save corrected monitoring data",
					"error":   err.Error(),
					"user_id": userID,
				})
			} else {
				logger.Debug(logger.LogData{
					"action":            "rebuild_new_recruit_scenarios_command",
					"message":           "Successfully saved corrected monitoring data to database",
					"user_id":           userID,
					"saved_start_time":  time.Unix(userMonitoring.StartedAt, 0).Format(time.RFC3339),
					"saved_expiry_time": time.Unix(userMonitoring.ExpiresAt, 0).Format(time.RFC3339),
				})
			}
		}

		// Create the user checkin task
		params := &models.UserCheckinParams{UserID: userID}
		scheduledTime := userMonitoring.ExpiresAt
		newTask, err := models.NewTaskWithScenario(
			models.TaskUserCheckin,
			params,
			scheduledTime,
			string(models.MonitoringScenarioNewRecruit),
		)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_new_recruit_scenarios_command",
				"message": "Failed to create user checkin task",
				"error":   err.Error(),
				"user_id": userID,
			})
			errors++
			userDetails = append(userDetails, fmt.Sprintf("**%s** - Error creating task: %s", userName, err.Error()))
			return true // Continue processing other threads
		}

		err = db.SaveTaskToRedis(ctx, *newTask)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_new_recruit_scenarios_command",
				"message": "Failed to save user checkin task to Redis",
				"error":   err.Error(),
				"user_id": userID,
			})
			errors++
			userDetails = append(userDetails, fmt.Sprintf("**%s** - Error saving task: %s", userName, err.Error()))
			return true // Continue processing other threads
		}

		recreatedScenarios++
		userDetails = append(userDetails, fmt.Sprintf("**%s** - ✅ Recreated scenario (expires: %s)",
			userName, expirationTime.Format("2006-01-02 15:04:05")))

		// Trigger analytics rebuild for this user with the correct start time
		eventWorker.Submit(userID, func(e eventWorker.Event) {
			logger.Info(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "rebuild_analytics_for_new_recruit",
				"message":  "Starting analytics rebuild for new recruit scenario",
				"user_id":  e.UserID,
			})

			// Use the monitoring data with the correct start time
			result, err := monitoring.RebuildUserAnalytics(e.UserID, userMonitoring, s, e.TraceID)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "rebuild_analytics_for_new_recruit",
					"message":  "Failed to rebuild analytics",
					"error":    err.Error(),
					"user_id":  e.UserID,
				})
			} else {
				logger.Info(logger.LogData{
					"trace_id": e.TraceID,
					"action":   "rebuild_analytics_for_new_recruit",
					"message":  "Successfully rebuilt analytics",
					"user_id":  e.UserID,
				})

				// Send results back to the invoking user
				FollowUpMessage(s, i, fmt.Sprintf(
					"📊 Analytics rebuilt for %s\n- Messages: %d\n- Voice joins: %d\n- Invites: %d\n- Top channel: %s\n- Window: %s → %s",
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

		logger.Info(logger.LogData{
			"action":          "rebuild_new_recruit_scenarios_command",
			"message":         "Successfully recreated new recruit scenario",
			"user_id":         userID,
			"thread_id":       thread.ID,
			"expiration_time": expirationTime.Format(time.RFC3339),
		})

		return true // Continue processing other threads
	}

	// Process archived threads until we reach the cutoff point
	logger.Info(logger.LogData{
		"action":      "rebuild_new_recruit_scenarios_command",
		"message":     "Processing archived recruitment threads",
		"cutoff_time": cutoffTime.Format(time.RFC3339),
	})

	var before *time.Time
	shouldContinue := true

	for shouldContinue {
		archivedThreads, err := s.ThreadsArchived(forumChannelID, before, 100)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "rebuild_new_recruit_scenarios_command",
				"message": "Failed to get archived forum threads",
				"error":   err.Error(),
			})
			break
		}

		if len(archivedThreads.Threads) == 0 {
			break
		}

		// Get the oldest thread timestamp for pagination
		oldest := archivedThreads.Threads[len(archivedThreads.Threads)-1].ThreadMetadata.ArchiveTimestamp

		// Process each archived thread individually and check for cutoff
		for _, thread := range archivedThreads.Threads {
			// Only process threads from the recruitment forum
			if thread.ParentID == forumChannelID {
				// Check if we should continue based on the message timestamp
				shouldContinue = processThread(thread, cutoffTime)
				if !shouldContinue {
					logger.Info(logger.LogData{
						"action":      "rebuild_new_recruit_scenarios_command",
						"message":     "Reached cutoff time based on message timestamp, stopping archived thread processing",
						"cutoff_time": cutoffTime.Format(time.RFC3339),
					})
					break
				}
			}
		}

		before = &oldest
	}

	// Create final response
	response := "✅ **New Recruit Scenarios Rebuild Complete**\n\n"
	response += fmt.Sprintf("**Threads processed:** %d\n", processedThreads)
	response += fmt.Sprintf("**Scenarios recreated:** %d\n", recreatedScenarios)
	response += fmt.Sprintf("**Skipped (too old):** %d\n", skippedOld)
	response += fmt.Sprintf("**Errors:** %d\n\n", errors)

	if len(userDetails) > 0 {
		response += "**👥 User Details:**\n"
		for _, detail := range userDetails {
			response += detail + "\n"
		}
	}

	FollowUpMessage(s, i, response, true)
}

// extractUserIDFromThreadTitle extracts user ID from thread title
// Thread format: "name - id" where id is a raw Discord user ID
func extractUserIDFromThreadTitle(threadName string) string {
	// Look for pattern "name - id" where id is a Discord user ID (17-19 digits)
	userIDPattern := regexp.MustCompile(`.* - (\d{17,19})$`)
	matches := userIDPattern.FindStringSubmatch(threadName)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// GetRebuildNewRecruitScenariosCommandDefinition returns the rebuild-new-recruit-scenarios command definition
func GetRebuildNewRecruitScenariosCommandDefinition() *discordgo.ApplicationCommand {
	adminPerm := int64(discordgo.PermissionAdministrator)
	return &discordgo.ApplicationCommand{
		Name:                     "rebuild-new-recruit-scenarios",
		Description:              "Rebuilds missing new recruit monitoring scenarios from archived forum posts",
		DefaultMemberPermissions: &adminPerm,
	}
}
