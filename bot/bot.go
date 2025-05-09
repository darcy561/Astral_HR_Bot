package bot

import (
	"astralHRBot/handlers"
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	Discord *discordgo.Session
)

func Setup() {
	botToken, exists := os.LookupEnv("BOT_TOKEN")
	if !exists {
		logger.Error(logger.LogData{
			"action":  "server_startup",
			"message": "Missing Discord Token",
		})
		os.Exit(1)
	}

	var err error
	Discord, err = discordgo.New("Bot " + botToken)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "server_startup",
			"message": "Error creating Discord session",
			"error":   err.Error(),
		})
		os.Exit(1)
	}

	// Add all handlers before opening the connection
	Discord.AddHandler(handlers.MessageHandlers)
	Discord.AddHandler(handlers.MemberLeaversAndJoiners)
	Discord.AddHandler(handlers.GuildMemberUpdateHandlers)
	Discord.AddHandler(handlers.ManageGuildChanges)

	Discord.Identify.Intents = discordgo.IntentsAll
}

func Start() {
	logger.Info(logger.LogData{
		"action":  "server_startup",
		"message": "Attempting to open connection to Discord...",
	})

	err := Discord.Open()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "server_startup",
			"message": "Error opening connection to Discord",
			"error":   err.Error(),
		})
		os.Exit(1)
	}

	logger.Info(logger.LogData{
		"action":  "server_startup",
		"message": "Connection to Discord established successfully.",
	})

	logger.Info(logger.LogData{
		"action":  "server_startup",
		"message": "Astral HR Bot is running...",
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	defer func() {
		logger.Info(logger.LogData{
			"action":  "server_shutdown",
			"message": "Astral HR Bot is shutting down...",
		})
		eventWorker.Shutdown()
		discordAPIWorker.Stop()
		if err := Discord.Close(); err != nil {
			logger.Error(logger.LogData{
				"action":  "server_shutdown",
				"message": "Error closing Discord session",
				"error":   err.Error(),
			})
		}
		logger.Info(logger.LogData{
			"action":  "server_shutdown",
			"message": "Astral HR Bot has shut down gracefully.",
		})
	}()
}
