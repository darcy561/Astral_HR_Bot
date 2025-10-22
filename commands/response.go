package commands

import (
	"astralHRBot/logger"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// RespondToInteraction responds to an interaction with a message
func RespondToInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	// Check if content exceeds Discord's 2000 character limit
	if len(content) > 2000 {
		// For initial responses, we can't split them, so we'll send a truncated version
		// and then send the full content as a follow-up
		truncatedContent := content[:1900] + "\n\n*[Message truncated - see follow-up for full content]*"

		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: truncatedContent,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}

		if !ephemeral {
			response.Data.Flags = 0
		}

		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "slash_command_error",
				"message": "Failed to respond to interaction",
				"error":   err.Error(),
			})
			return
		}

		// Send the full content as follow-up messages
		FollowUpMessage(s, i, content, ephemeral)
	} else {
		// Send single message if under limit
		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}

		if !ephemeral {
			response.Data.Flags = 0
		}

		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "slash_command_error",
				"message": "Failed to respond to interaction",
				"error":   err.Error(),
			})
		}
	}
}

// RespondToInteractionWithEmbed responds to an interaction with an embed
func RespondToInteractionWithEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, ephemeral bool) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}

	if !ephemeral {
		response.Data.Flags = 0
	}

	err := s.InteractionRespond(i.Interaction, response)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "slash_command_error",
			"message": "Failed to respond to interaction with embed",
			"error":   err.Error(),
		})
	}
}

// FollowUpMessage sends a follow-up message
func FollowUpMessage(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	// Check if content exceeds Discord's 2000 character limit
	if len(content) > 2000 {
		// Split content into chunks
		chunks := splitMessage(content, 1900) // Leave some buffer for formatting

		for chunkIndex, chunk := range chunks {
			// Add chunk indicator for multi-part messages
			if len(chunks) > 1 {
				chunk = fmt.Sprintf("**Part %d/%d**\n%s", chunkIndex+1, len(chunks), chunk)
			}

			_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: chunk,
				Flags:   flags,
			})
			if err != nil {
				logger.Error(logger.LogData{
					"action":  "slash_command_error",
					"message": "Failed to send follow-up message chunk",
					"chunk":   chunkIndex + 1,
					"total":   len(chunks),
					"error":   err.Error(),
				})
				break // Stop sending chunks if one fails
			}
		}
	} else {
		// Send single message if under limit
		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Flags:   flags,
		})
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "slash_command_error",
				"message": "Failed to send follow-up message",
				"error":   err.Error(),
			})
		}
	}
}

// splitMessage splits a long message into chunks that fit within Discord's character limit
func splitMessage(content string, maxLength int) []string {
	if len(content) <= maxLength {
		return []string{content}
	}

	var chunks []string
	lines := strings.Split(content, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		// If adding this line would exceed the limit, start a new chunk
		if currentChunk.Len()+len(line)+1 > maxLength && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}

		// If a single line is too long, split it
		if len(line) > maxLength {
			// If we have content in current chunk, save it first
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
			}

			// Split the long line
			lineChunks := splitLongLine(line, maxLength)
			chunks = append(chunks, lineChunks...)
			continue
		}

		// Add line to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
	}

	// Add the last chunk if it has content
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// splitLongLine splits a single line that's too long into multiple chunks
func splitLongLine(line string, maxLength int) []string {
	var chunks []string

	for len(line) > maxLength {
		// Find a good break point (space or comma)
		breakPoint := maxLength
		for i := maxLength - 1; i >= maxLength-50; i-- {
			if i < len(line) && (line[i] == ' ' || line[i] == ',' || line[i] == '\n') {
				breakPoint = i + 1
				break
			}
		}

		chunks = append(chunks, line[:breakPoint])
		line = line[breakPoint:]
	}

	if len(line) > 0 {
		chunks = append(chunks, line)
	}

	return chunks
}
