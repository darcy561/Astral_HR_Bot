package channels

import (
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ChannelMap = make(map[string]string)
var emojiRegex = regexp.MustCompile(`[\p{So}\p{Sk}\p{Co}]`)

func BuildGuildChannels(s *discordgo.Session) error {
	if len(s.State.Guilds) == 0 {
		log.Fatalf("Bot is not connected to any guild.")
		return nil
	}

	guildID, exists := os.LookupEnv("GUILD_ID")
	if !exists {
		log.Fatalf("No Guild ID Provided")
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		log.Fatalf("Error fetching channels: %v", err)
		return err
	}

	for _, channel := range channels {
		processedChannelName := emojiRegex.ReplaceAllString(channel.Name, "")
		processedChannelName = strings.ToLower(processedChannelName)
		processedChannelName = strings.ReplaceAll(processedChannelName, "-", "_")

		ChannelMap[processedChannelName] = channel.ID
	}

	log.Println("Channels fetched and mapped successfully.")
	return nil
}

// Enter channel by name, all lowercase, use_instead of spaces
func GetChannelID(channelName string) string {
	return ChannelMap[channelName]
}
