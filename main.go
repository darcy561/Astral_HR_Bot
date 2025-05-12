package main

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	"astralHRBot/logger"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/taskworker"
)

func main() {
	logger.StartLogger()
	logger.System(logger.LogData{
		"action":  "startup",
		"message": "Starting AstralHRBot...",
	})

	db.InitRedis()
	bot.Setup()

	discordAPIWorker.NewWorker(bot.Discord)
	eventWorker.NewWorkerPool()
	taskworker.StartTaskProcessor()
	bot.Start()
}
