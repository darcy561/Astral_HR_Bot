package helper

import (
	"astralHRBot/channels"
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// RecruitmentThreadManager provides methods to manage recruitment threads
type RecruitmentThreadManager struct {
	session     *discordgo.Session
	event       eventWorker.Event
	channelID   string
	thread      *discordgo.Channel
	found       bool
	channelInfo *discordgo.Channel
}

// NewRecruitmentThreadManager creates a new manager for a specific user
func NewRecruitmentThreadManager(s *discordgo.Session, e eventWorker.Event, userID string) *RecruitmentThreadManager {
	recruitmentChannelID := channels.GetRecruitmentForum()

	// Debug logging to help diagnose thread finding issues
	logger.Debug(logger.LogData{
		"trace_id":   e.TraceID,
		"action":     "find_recruitment_thread",
		"message":    "Searching for recruitment thread",
		"user_id":    userID,
		"channel_id": recruitmentChannelID,
	})

	thread, found := FindForumThreadByTitle(s, recruitmentChannelID, userID)

	if found {
		logger.Debug(logger.LogData{
			"trace_id":    e.TraceID,
			"action":      "find_recruitment_thread",
			"message":     "Found recruitment thread",
			"user_id":     userID,
			"thread_id":   thread.ID,
			"thread_name": thread.Name,
		})
	} else {
		logger.Debug(logger.LogData{
			"trace_id": e.TraceID,
			"action":   "find_recruitment_thread",
			"message":  "No recruitment thread found",
			"user_id":  userID,
		})
	}

	// Get channel info for tags
	channelInfo, err := s.Channel(recruitmentChannelID)
	if err != nil {
		logger.Error(logger.LogData{
			"trace_id": e.TraceID,
			"action":   "get_recruitment_channel",
			"message":  "Failed to get recruitment channel info",
			"error":    err.Error(),
		})
	}

	return &RecruitmentThreadManager{
		session:     s,
		event:       e,
		channelID:   recruitmentChannelID,
		thread:      thread,
		found:       found,
		channelInfo: channelInfo,
	}
}

// HasThread returns true if a recruitment thread exists for the user
func (rtm *RecruitmentThreadManager) HasThread() bool {
	return rtm.found
}

// GetThread returns the thread if it exists
func (rtm *RecruitmentThreadManager) GetThread() (*discordgo.Channel, bool) {
	return rtm.thread, rtm.found
}

// IsThreadOpen checks if the recruitment thread is currently open (not archived)
func (rtm *RecruitmentThreadManager) IsThreadOpen() bool {
	if !rtm.found {
		return false
	}

	// Check if the thread has ThreadMetadata and if it's archived
	if rtm.thread.ThreadMetadata != nil {
		// If ArchiveTimestamp is set, the thread is archived
		return rtm.thread.ThreadMetadata.ArchiveTimestamp.IsZero()
	}

	// If no ThreadMetadata, assume it's open
	return true
}

// SendMessage sends a message to the recruitment thread
func (rtm *RecruitmentThreadManager) SendMessage(message string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "send_message",
			"message":  "No recruitment thread found - skipping message",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		_, err := rtm.session.ChannelMessageSend(rtm.thread.ID, message)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "send_message",
				"message":  "Failed to send message to recruitment thread",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "send_message",
				"message":  "Successfully sent message to recruitment thread",
			})
		}
		return err
	})

	return nil
}

// SendMessageEmbed sends an embed message to the recruitment thread
func (rtm *RecruitmentThreadManager) SendMessageEmbed(embed *discordgo.MessageEmbed) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "send_message_embed",
			"message":  "No recruitment thread found - skipping embed message",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		_, err := rtm.session.ChannelMessageSendEmbed(rtm.thread.ID, embed)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "send_message_embed",
				"message":  "Failed to send embed message to recruitment thread",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "send_message_embed",
				"message":  "Successfully sent embed message to recruitment thread",
			})
		}
		return err
	})

	return nil
}

// ApplyTag applies a tag to the recruitment thread
func (rtm *RecruitmentThreadManager) ApplyTag(tagName string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "apply_tag",
			"message":  "No recruitment thread found - skipping tag application",
		})
		return nil
	}

	if rtm.channelInfo == nil {
		logger.Error(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "apply_tag",
			"message":  "Channel info not available for tag lookup",
		})
		return fmt.Errorf("channel info not available")
	}

	// Find the tag by name
	tagID := ""
	for _, tag := range rtm.channelInfo.AvailableTags {
		if strings.EqualFold(tag.Name, tagName) {
			tagID = tag.ID
			break
		}
	}

	if tagID == "" {
		logger.Error(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "apply_tag",
			"message":  fmt.Sprintf("Tag '%s' not found in recruitment channel", tagName),
		})
		return fmt.Errorf("tag '%s' not found", tagName)
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		_, err := rtm.session.ChannelEditComplex(rtm.thread.ID, &discordgo.ChannelEdit{
			AppliedTags: &[]string{tagID},
		})
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "apply_tag",
				"message":  fmt.Sprintf("Failed to apply tag '%s' to recruitment thread", tagName),
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "apply_tag",
				"message":  fmt.Sprintf("Successfully applied tag '%s' to recruitment thread", tagName),
			})
		}
		return err
	})

	return nil
}

// CloseThread closes/archives the recruitment thread and optionally applies a tag
func (rtm *RecruitmentThreadManager) CloseThread(tagName string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "close_thread",
			"message":  "No recruitment thread found - skipping closure",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		// Find tag if specified
		var tagsToApply *[]string
		if tagName != "" && rtm.channelInfo != nil {
			tags := []string{}
			tagFound := false
			logger.Debug(logger.LogData{
				"trace_id":       rtm.event.TraceID,
				"action":         "close_thread",
				"message":        "Looking for tag in available tags",
				"tag_name":       tagName,
				"available_tags": len(rtm.channelInfo.AvailableTags),
			})

			for _, tag := range rtm.channelInfo.AvailableTags {
				logger.Debug(logger.LogData{
					"trace_id":           rtm.event.TraceID,
					"action":             "close_thread",
					"message":            "Checking available tag",
					"tag_name":           tagName,
					"available_tag_name": tag.Name,
					"available_tag_id":   tag.ID,
				})
				if strings.EqualFold(tag.Name, tagName) {
					tags = append(tags, tag.ID)
					tagFound = true
					logger.Debug(logger.LogData{
						"trace_id": rtm.event.TraceID,
						"action":   "close_thread",
						"message":  "Found matching tag",
						"tag_name": tagName,
						"tag_id":   tag.ID,
					})
					break
				}
			}

			if !tagFound {
				logger.Warn(logger.LogData{
					"trace_id":       rtm.event.TraceID,
					"action":         "close_thread",
					"message":        "Tag not found in available tags",
					"tag_name":       tagName,
					"available_tags": len(rtm.channelInfo.AvailableTags),
				})
				// Don't apply any tags if the specified tag wasn't found
				tagsToApply = nil
			} else {
				tagsToApply = &tags
			}
		}

		// Close the thread
		isArchived := true
		_, err := rtm.session.ChannelEditComplex(rtm.thread.ID, &discordgo.ChannelEdit{
			Archived:    &isArchived,
			AppliedTags: tagsToApply,
		})

		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "close_thread",
				"message":  "Failed to close recruitment thread",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "close_thread",
				"message":  "Successfully closed recruitment thread",
			})
		}
		return err
	})

	return nil
}

// RemoveTags removes tags from the recruitment thread
// If tagName is empty, removes all tags. If tagName is specified, removes only that tag.
func (rtm *RecruitmentThreadManager) RemoveTags(tagName string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "remove_tags",
			"message":  "No recruitment thread found - skipping tag removal",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		var tagsToApply *[]string

		if tagName == "" {
			// Remove all tags
			emptyTags := []string{}
			tagsToApply = &emptyTags
		} else {
			// Remove specific tag by getting current tags and filtering out the specified one
			if rtm.channelInfo == nil {
				logger.Error(logger.LogData{
					"trace_id": rtm.event.TraceID,
					"action":   "remove_tags",
					"message":  "Channel info not available for tag lookup",
				})
				return fmt.Errorf("channel info not available")
			}

			// Find the tag ID to remove
			tagIDToRemove := ""
			for _, tag := range rtm.channelInfo.AvailableTags {
				if strings.EqualFold(tag.Name, tagName) {
					tagIDToRemove = tag.ID
					break
				}
			}

			if tagIDToRemove == "" {
				logger.Error(logger.LogData{
					"trace_id": rtm.event.TraceID,
					"action":   "remove_tags",
					"message":  fmt.Sprintf("Tag '%s' not found in recruitment channel", tagName),
				})
				return fmt.Errorf("tag '%s' not found", tagName)
			}

			// Get current thread info to see existing tags
			threadInfo, err := rtm.session.Channel(rtm.thread.ID)
			if err != nil {
				logger.Error(logger.LogData{
					"trace_id": rtm.event.TraceID,
					"action":   "remove_tags",
					"message":  "Failed to get current thread info",
					"error":    err.Error(),
				})
				return err
			}

			// Filter out the tag to remove
			var filteredTags []string
			for _, existingTagID := range threadInfo.AppliedTags {
				if existingTagID != tagIDToRemove {
					filteredTags = append(filteredTags, existingTagID)
				}
			}
			tagsToApply = &filteredTags
		}

		_, err := rtm.session.ChannelEditComplex(rtm.thread.ID, &discordgo.ChannelEdit{
			AppliedTags: tagsToApply,
		})
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "remove_tags",
				"message":  fmt.Sprintf("Failed to remove tag '%s' from recruitment thread", tagName),
				"error":    err.Error(),
			})
		} else {
			if tagName == "" {
				logger.Debug(logger.LogData{
					"trace_id": rtm.event.TraceID,
					"action":   "remove_tags",
					"message":  "Successfully removed all tags from recruitment thread",
				})
			} else {
				logger.Debug(logger.LogData{
					"trace_id": rtm.event.TraceID,
					"action":   "remove_tags",
					"message":  fmt.Sprintf("Successfully removed tag '%s' from recruitment thread", tagName),
				})
			}
		}
		return err
	})

	return nil
}

// UpdateThreadTitle updates the title of the recruitment thread
func (rtm *RecruitmentThreadManager) UpdateThreadTitle(newTitle string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "update_thread_title",
			"message":  "No recruitment thread found - skipping title update",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		logger.Debug(logger.LogData{
			"trace_id":  rtm.event.TraceID,
			"action":    "update_thread_title",
			"message":   "Updating recruitment thread title",
			"thread_id": rtm.thread.ID,
			"new_title": newTitle,
		})

		_, err := rtm.session.ChannelEditComplex(rtm.thread.ID, &discordgo.ChannelEdit{
			Name: newTitle,
		})
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "update_thread_title",
				"message":  "Failed to update recruitment thread title",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "update_thread_title",
				"message":  "Successfully updated recruitment thread title",
			})
		}
		return err
	})

	return nil
}

// CreateThread creates a new recruitment thread for a user
func (rtm *RecruitmentThreadManager) CreateThread(userName, userID string) error {
	if rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "create_thread",
			"message":  "Recruitment thread already exists - skipping creation",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		newThreadTitle := fmt.Sprintf("%s - %s", userName, userID)
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "create_thread",
			"message":  "Creating new recruitment thread",
			"user_id":  userID,
			"title":    newThreadTitle,
		})

		_, err := rtm.session.ForumThreadStart(rtm.channelID, newThreadTitle, 10080, fmt.Sprintf("%s Joined Recruitment", userName))
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "create_thread",
				"message":  "Failed to create recruitment thread",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "create_thread",
				"message":  "Successfully created recruitment thread",
				"user_id":  userID,
			})
		}
		return err
	})

	return nil
}

// ReopenThread reopens an existing recruitment thread
func (rtm *RecruitmentThreadManager) ReopenThread() error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "reopen_thread",
			"message":  "No recruitment thread found - skipping reopen",
		})
		return nil
	}

	discordAPIWorker.NewRequest(rtm.event, func() error {
		logger.Debug(logger.LogData{
			"trace_id":  rtm.event.TraceID,
			"action":    "reopen_thread",
			"message":   "Reopening recruitment thread",
			"thread_id": rtm.thread.ID,
		})

		_, err := rtm.session.ChannelEditComplex(rtm.thread.ID, &discordgo.ChannelEdit{
			AutoArchiveDuration: 0,
		})
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "reopen_thread",
				"message":  "Failed to reopen recruitment thread",
				"error":    err.Error(),
			})
		} else {
			logger.Debug(logger.LogData{
				"trace_id": rtm.event.TraceID,
				"action":   "reopen_thread",
				"message":  "Successfully reopened recruitment thread",
			})
		}
		return err
	})

	return nil
}

// SendMessageAndClose sends a message and then closes the thread with an optional tag
func (rtm *RecruitmentThreadManager) SendMessageAndClose(message, tagName string) error {
	if !rtm.found {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "send_message_and_close",
			"message":  "No recruitment thread found - skipping operation",
		})
		return nil
	}

	logger.Debug(logger.LogData{
		"trace_id":     rtm.event.TraceID,
		"action":       "send_message_and_close",
		"message":      "Starting send message and close operation",
		"thread_id":    rtm.thread.ID,
		"user_message": message,
		"tag_name":     tagName,
	})

	// Send message first
	if message != "" {
		rtm.SendMessage(message)
	}

	// Then close with tag
	err := rtm.CloseThread(tagName)
	if err != nil {
		logger.Error(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "send_message_and_close",
			"message":  "Failed to close thread with tag",
			"error":    err.Error(),
			"tag_name": tagName,
		})
	} else {
		logger.Debug(logger.LogData{
			"trace_id": rtm.event.TraceID,
			"action":   "send_message_and_close",
			"message":  "Successfully completed send message and close operation",
			"tag_name": tagName,
		})
	}

	return err
}
