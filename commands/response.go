package commands

import (
	"astralHRBot/logger"

	"github.com/bwmarrin/discordgo"
)

// RespondToInteraction responds to an interaction with a message
func RespondToInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
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
