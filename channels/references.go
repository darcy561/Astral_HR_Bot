package channels

import (
	"log"
	"os"
)

// Channel references with environment variable names
const (
	// Announcement Channels
	GeneralChannel = "GENERAL_CHANNEL_ID"
	LandingChannel = "LANDING_CHANNEL_ID"
	LeaversChannel = "LEAVERS_CHANNEL_ID"

	// Recruitment Channels
	RecruitmentChannel = "RECRUITMENT_CHANNEL_ID"
	RecruitmentForum   = "RECRUITMENT_FORUM_ID"
	RecruitmentHub     = "RECRUITMENT_HUB_ID"

	HRChannel = "HR_CHANNEL_ID"
)

// GetChannelID returns the channel ID from environment variables
func GetChannelID(channelEnvVar string) string {
	id, exists := os.LookupEnv(channelEnvVar)
	if !exists {
		log.Printf("Warning: Channel ID not found for environment variable: %s", channelEnvVar)
		return ""
	}
	return id
}

// Helper functions for each channel
func GetGeneralChannel() string {
	return GetChannelID(GeneralChannel)
}

func GetLandingChannel() string {
	return GetChannelID(LandingChannel)
}

func GetLeaversChannel() string {
	return GetChannelID(LeaversChannel)
}

func GetRecruitmentChannel() string {
	return GetChannelID(RecruitmentChannel)
}

func GetRecruitmentForum() string {
	return GetChannelID(RecruitmentForum)
}

func GetRecruitmentHub() string {
	return GetChannelID(RecruitmentHub)
}

func GetHRChannel() string {
	return GetChannelID(HRChannel)
}
