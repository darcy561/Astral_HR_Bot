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
		AddChannelToMap(channel)
	}

	log.Println("Channels fetched and mapped successfully.")
	return nil
}

// Enter channel by name, lowercase and spaces as _, followed by a - and then the last 4 digits of channel ID (e.g., general_1a2b)
func GetChannelID(channelName string) string {
	return ChannelMap[channelName]
}

func generateChannelKey(name, id string) string {
	processedName := emojiRegex.ReplaceAllString(name, "")
	processedName = strings.ToLower(processedName)
	processedName = strings.ReplaceAll(processedName, "-", "_")
	return processedName + "_" + id[len(id)-4:]
}

func AddChannelToMap(channel *discordgo.Channel) {
	key := generateChannelKey(channel.Name, channel.ID)
	ChannelMap[key] = channel.ID
}
func RemoveChannelFromMap(channel *discordgo.Channel) {
	key := generateChannelKey(channel.Name, channel.ID)
	delete(ChannelMap, key)
}

func UpdateChannel(channel *discordgo.Channel) {
	RemoveChannelFromMap(channel)
	AddChannelToMap(channel)
}
