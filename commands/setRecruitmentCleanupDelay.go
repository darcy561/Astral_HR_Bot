package commands

import (
	"astralHRBot/globals"
	"astralHRBot/logger"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// SetRecruitmentCleanupDelayCommand handles the /set-recruitment-cleanup-delay slash command
func SetRecruitmentCleanupDelayCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logger.Debug(logger.LogData{
		"action":  "set_recruitment_cleanup_delay_command",
		"message": "SetRecruitmentCleanupDelay command executed",
		"user_id": i.Member.User.ID,
	})

	// Get the days parameter from the interaction
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		RespondToInteraction(s, i, "Please provide the number of days for recruitment cleanup delay", true)
		return
	}

	daysStr := options[0].StringValue()
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		RespondToInteraction(s, i, "Please provide a valid number of days", true)
		return
	}

	if days < 1 {
		RespondToInteraction(s, i, "Recruitment cleanup delay must be at least 1 day", true)
		return
	}

	// Update the global setting safely
	globals.SetRecruitmentCleanupDelay(days)

	content := "Recruitment cleanup delay has been set to " + strconv.Itoa(days) + " days"
	RespondToInteraction(s, i, content, true)
}

// GetSetRecruitmentCleanupDelayCommandDefinition returns the command definition
func GetSetRecruitmentCleanupDelayCommandDefinition() *discordgo.ApplicationCommand {
	adminPerm := int64(discordgo.PermissionAdministrator)
	return &discordgo.ApplicationCommand{
		Name:                     "set-recruitment-cleanup-delay",
		Description:              "Set the recruitment cleanup delay in days (Administrator only)",
		DefaultMemberPermissions: &adminPerm,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "days",
				Description: "Number of days for recruitment cleanup delay",
				Required:    true,
				MinValue:    &[]float64{1}[0],
			},
		},
	}
}
