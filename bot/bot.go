package bot

import (
	"astralHRBot/channels"
	"astralHRBot/handlers"
	"astralHRBot/roles"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var Discord *discordgo.Session

func Setup() {
	botToken, exists := os.LookupEnv("BOT_TOKEN")
	if !exists {
		log.Fatalf("Missing Discord Token")
	}

	var err error
	Discord, err = discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	Discord.Identify.Intents = discordgo.IntentsAll

	Discord.AddHandler(handlers.MessageHandlers)
	Discord.AddHandler(handlers.MemberLeaversAndJoiners)
	Discord.AddHandler(handlers.GuildMemberUpdateHandlers)
	Discord.AddHandler(handlers.ManageGuildChanges)

}

func Start() {

	log.Println("Attempting to open connection to Discord...")

	err := Discord.Open()
	if err != nil {
		log.Fatalf("Error opening connection to Discord: %v", err)
	}

	log.Println("Connection to Discord established successfully.")
	roles.BuildGuildRoles(Discord)
	channels.BuildGuildChannels(Discord)

	log.Println("Astral HR Bot is running...")

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	defer func() {
		log.Println("Astral HR Bot is shutting down...")
		if err := Discord.Close(); err != nil {
			log.Printf("Error closing Discord session: %v", err)
		}
	}()

}
