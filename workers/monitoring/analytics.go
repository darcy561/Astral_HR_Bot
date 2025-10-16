package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

// AnalyticsResult contains the computed analytics summary for a user
type AnalyticsResult struct {
	Messages        int64
	VoiceJoins      int64
	Invites         int64
	TopChannelID    string
	ChannelsScanned int
	StartTime       time.Time
	EndTime         time.Time
}

// RebuildUserAnalytics rebuilds analytics data for a specific user
// This can be called from other places as needed
func RebuildUserAnalytics(userID string, monitoringData *models.UserMonitoring, s *discordgo.Session, traceID string) (*AnalyticsResult, error) {
	ctx := context.Background()

	logger.Debug(logger.LogData{
		"trace_id":       traceID,
		"action":         "rebuild_user_analytics_debug",
		"message":        "Received monitoring data for analytics",
		"user_id":        userID,
		"started_at":     time.Unix(monitoringData.StartedAt, 0).Format(time.RFC3339),
		"expires_at":     time.Unix(monitoringData.ExpiresAt, 0).Format(time.RFC3339),
		"expires_at_raw": monitoringData.ExpiresAt,
		"scenarios":      monitoringData.GetScenarios(),
	})

	// Determine the time period for analytics
	startTime := time.Unix(monitoringData.StartedAt, 0)
	var endTime time.Time

	if monitoringData.ExpiresAt > 0 {
		// If there is an expiry date, use this timeframe
		endTime = time.Unix(monitoringData.ExpiresAt, 0)
	} else {
		// If there isn't an expiry, use the start date + the default time for the scenario
		trackingWindow := time.Duration(globals.GetNewRecruitTrackingDays()) * 24 * time.Hour
		endTime = startTime.Add(trackingWindow)
	}

	logger.Debug(logger.LogData{
		"trace_id":                       traceID,
		"action":                         "rebuild_user_analytics",
		"message":                        "Rebuilding analytics for time period",
		"user_id":                        userID,
		"start_time":                     startTime.Format(time.RFC3339),
		"end_time":                       endTime.Format(time.RFC3339),
		"monitoring_data_started_at":     time.Unix(monitoringData.StartedAt, 0).Format(time.RFC3339),
		"monitoring_data_expires_at":     time.Unix(monitoringData.ExpiresAt, 0).Format(time.RFC3339),
		"monitoring_data_expires_at_raw": monitoringData.ExpiresAt,
	})

	// Initialize analytics counters
	messages := int64(0)
	voiceJoins := int64(0)
	invites := int64(0)
	topChannelID := ""
	channelMessageCounts := make(map[string]int64)

	// Get all channels in the guild
	guild, err := s.State.Guild(s.State.Guilds[0].ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	// Scan each channel for user messages
	for _, channel := range guild.Channels {
		// Skip non-text channels
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// Get messages from the channel during the monitoring period
		channelMessages, err := getChannelMessagesForUser(s, channel.ID, userID, startTime, endTime)
		if err != nil {
			logger.Warn(logger.LogData{
				"trace_id":   traceID,
				"action":     "rebuild_user_analytics",
				"message":    "Failed to get messages from channel",
				"error":      err.Error(),
				"user_id":    userID,
				"channel_id": channel.ID,
			})
			continue
		}

		// Count messages for this channel
		channelCount := int64(len(channelMessages))
		channelMessageCounts[channel.ID] = channelCount
		messages += channelCount

		// Update top channel
		if channelCount > 0 && (topChannelID == "" || channelCount > channelMessageCounts[topChannelID]) {
			topChannelID = channel.ID
		}
	}

	// Get voice joins from audit log
	voiceJoins, err = getVoiceJoinsFromAuditLog(s, guild.ID, userID, startTime, endTime, traceID)
	if err != nil {
		logger.Warn(logger.LogData{
			"trace_id": traceID,
			"action":   "rebuild_user_analytics",
			"message":  "Failed to get voice joins from audit log",
			"error":    err.Error(),
			"user_id":  userID,
		})
		voiceJoins = 0
	}

	// Get invites from audit log
	invites, err = getInvitesFromAuditLog(s, guild.ID, userID, startTime, endTime, traceID)
	if err != nil {
		logger.Warn(logger.LogData{
			"trace_id": traceID,
			"action":   "rebuild_user_analytics",
			"message":  "Failed to get invites from audit log",
			"error":    err.Error(),
			"user_id":  userID,
		})
		invites = 0
	}

	logger.Info(logger.LogData{
		"trace_id":         traceID,
		"action":           "rebuild_user_analytics",
		"message":          "Analytics rebuild completed",
		"user_id":          userID,
		"messages":         messages,
		"voice_joins":      voiceJoins,
		"invites":          invites,
		"top_channel_id":   topChannelID,
		"channels_scanned": len(guild.Channels),
	})

	// Update analytics in Redis for each scenario
	scenarios := monitoringData.GetScenarios()
	for _, scenario := range scenarios {
		err = db.UpdateUserAnalytics(ctx, userID, string(scenario), messages, voiceJoins, invites, topChannelID)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": traceID,
				"action":   "rebuild_user_analytics",
				"message":  "Failed to update analytics in Redis",
				"error":    err.Error(),
				"user_id":  userID,
				"scenario": scenario,
			})
			return nil, fmt.Errorf("failed to update analytics: %w", err)
		}
	}

	return &AnalyticsResult{
		Messages:        messages,
		VoiceJoins:      voiceJoins,
		Invites:         invites,
		TopChannelID:    topChannelID,
		ChannelsScanned: len(guild.Channels),
		StartTime:       startTime,
		EndTime:         endTime,
	}, nil
}

// getChannelMessagesForUser gets messages from a channel for a specific user within a time period
func getChannelMessagesForUser(s *discordgo.Session, channelID, userID string, startTime, endTime time.Time) ([]*discordgo.Message, error) {
	var allMessages []*discordgo.Message
	before := ""

	// Get messages in batches
	for {
		messages, err := s.ChannelMessages(channelID, 100, before, "", "")
		if err != nil {
			return nil, err
		}

		if len(messages) == 0 {
			break
		}

		// Filter messages by user and time period
		for _, msg := range messages {
			// Check if message is from our user
			if msg.Author.ID != userID {
				continue
			}

			// Check if message is within our time period
			msgTime := msg.Timestamp
			if msgTime.Before(startTime) {
				// If we've gone past the start time, we can stop
				return allMessages, nil
			}
			if msgTime.After(endTime) {
				// Skip messages after end time
				continue
			}

			allMessages = append(allMessages, msg)
		}

		// Set up for next batch
		before = messages[len(messages)-1].ID

		// Check if the oldest message is before our start time
		oldestMsg := messages[len(messages)-1]
		if oldestMsg.Timestamp.Before(startTime) {
			break
		}
	}

	return allMessages, nil
}

// getVoiceJoinsFromAuditLog gets voice join events from audit log
func getVoiceJoinsFromAuditLog(s *discordgo.Session, guildID, userID string, startTime, endTime time.Time, traceID string) (int64, error) {
	var totalJoins int64

	// Get audit logs in batches
	before := ""
	for {
		auditLogs, err := s.GuildAuditLog(guildID, before, "", 100, 27) // Action type 27 = MEMBER_VOICE_JOIN
		if err != nil {
			return 0, fmt.Errorf("failed to get audit logs: %w", err)
		}

		if len(auditLogs.AuditLogEntries) == 0 {
			break
		}

		// Count voice joins for this user in the time period
		for _, entry := range auditLogs.AuditLogEntries {
			// Check if this is a voice join event for our user
			if entry.ActionType != nil && *entry.ActionType == 27 && entry.TargetID == userID {
				// Parse the snowflake ID to get timestamp
				if id, err := strconv.ParseInt(entry.ID, 10, 64); err == nil {
					entryTime := time.Unix((id>>22)+1420070400000, 0)
					if entryTime.After(startTime) && entryTime.Before(endTime) {
						totalJoins++
					}
				}
			}
		}

		// Set up for next batch
		before = auditLogs.AuditLogEntries[len(auditLogs.AuditLogEntries)-1].ID

		// Check if we've gone past our start time
		oldestEntry := auditLogs.AuditLogEntries[len(auditLogs.AuditLogEntries)-1]
		if func() time.Time {
			if id, err := strconv.ParseInt(oldestEntry.ID, 10, 64); err == nil {
				return time.Unix((id>>22)+1420070400000, 0)
			}
			return time.Time{}
		}().Before(startTime) {
			break
		}
	}

	logger.Debug(logger.LogData{
		"trace_id": traceID,
		"action":   "get_voice_joins_from_audit_log",
		"message":  "Retrieved voice joins from audit log",
		"user_id":  userID,
		"joins":    totalJoins,
	})

	return totalJoins, nil
}

// getInvitesFromAuditLog gets invite creation events from audit log
func getInvitesFromAuditLog(s *discordgo.Session, guildID, userID string, startTime, endTime time.Time, traceID string) (int64, error) {
	var totalInvites int64

	// Get audit logs in batches
	before := ""
	for {
		auditLogs, err := s.GuildAuditLog(guildID, before, "", 100, 40) // Action type 40 = INVITE_CREATE
		if err != nil {
			return 0, fmt.Errorf("failed to get audit logs: %w", err)
		}

		if len(auditLogs.AuditLogEntries) == 0 {
			break
		}

		// Count invite creations for this user in the time period
		for _, entry := range auditLogs.AuditLogEntries {
			// Check if this is an invite creation event for our user
			if entry.ActionType != nil && *entry.ActionType == 40 && entry.UserID == userID {
				// Parse the snowflake ID to get timestamp
				if id, err := strconv.ParseInt(entry.ID, 10, 64); err == nil {
					entryTime := time.Unix((id>>22)+1420070400000, 0)
					if entryTime.After(startTime) && entryTime.Before(endTime) {
						totalInvites++
					}
				}
			}
		}

		// Set up for next batch
		before = auditLogs.AuditLogEntries[len(auditLogs.AuditLogEntries)-1].ID

		// Check if we've gone past our start time
		oldestEntry := auditLogs.AuditLogEntries[len(auditLogs.AuditLogEntries)-1]
		if func() time.Time {
			if id, err := strconv.ParseInt(oldestEntry.ID, 10, 64); err == nil {
				return time.Unix((id>>22)+1420070400000, 0)
			}
			return time.Time{}
		}().Before(startTime) {
			break
		}
	}

	logger.Debug(logger.LogData{
		"trace_id": traceID,
		"action":   "get_invites_from_audit_log",
		"message":  "Retrieved invites from audit log",
		"user_id":  userID,
		"invites":  totalInvites,
	})

	return totalInvites, nil
}
