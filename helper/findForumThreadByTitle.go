package helper

import (
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func FindForumThreadByTitle(s *discordgo.Session, channelID string, phrase string) (*discordgo.Channel, bool) {
	guildID, _ := os.LookupEnv("GUILD_ID")

	activeThreadList, err := s.GuildThreadsActive(guildID)

	if err != nil {
		return nil, false
	}

	for _, thread := range activeThreadList.Threads {

		if strings.Contains(strings.ToLower(thread.Name), strings.ToLower(phrase)) {
			return thread, true
		}
	}

	var before *time.Time

	for {
		archivedThreads, err := s.ThreadsArchived(channelID, before, 100)
		if err != nil {
			return nil, false
		}
		for _, thread := range archivedThreads.Threads {
			if strings.Contains(strings.ToLower(thread.Name), strings.ToLower(phrase)) {
				return thread, true
			}
		}

		if len(archivedThreads.Threads) == 0 {
			break
		}

		oldest := archivedThreads.Threads[len(archivedThreads.Threads)-1].ThreadMetadata.ArchiveTimestamp
		before = &oldest
	}

	return nil, false

}
