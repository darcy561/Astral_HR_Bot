package commands

import (
	"astralHRBot/globals"
	"astralHRBot/logger"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// SetNewRecruitTrackingDaysCommand handles the /set-new-recruit-tracking-days slash command
func SetNewRecruitTrackingDaysCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "set_new_recruit_tracking_days_command",
		"message": "SetNewRecruitTrackingDays command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the days parameter from the interaction
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		RespondToInteraction(s, i, "Please provide the number of days for new recruit tracking", true)
		return
	}

	daysStr := options[0].StringValue()
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		RespondToInteraction(s, i, "Please provide a valid number of days", true)
		return
	}

	if days < 1 {
		RespondToInteraction(s, i, "New recruit tracking days must be at least 1 day", true)
		return
	}

	// Update the global setting safely
	globals.SetNewRecruitTrackingDays(days)

	content := "New recruit tracking days has been set to " + strconv.Itoa(days) + " days"
	RespondToInteraction(s, i, content, true)
}

// GetSetNewRecruitTrackingDaysCommandDefinition returns the command definition
func GetSetNewRecruitTrackingDaysCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "set-new-member-tracking-days",
		Description: "Set the number of days to track new member activity",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "days",
				Description: "Number of days to track new member activity",
				Required:    true,
				MinValue:    &[]float64{1}[0],
			},
		},
	}
}
