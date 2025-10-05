package main

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/tasks"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
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

	tasks.RegisterHandlers()

	discordAPIWorker.NewWorker(bot.Discord)
	eventWorker.NewWorkerPool()
	taskworker.StartTaskProcessor()
	monitoring.Start()
	monitoring.WaitForReady()

	logger.Info(logger.LogData{
		"action":  "startup",
		"message": "All systems initialized, starting bot...",
	})

	bot.Start()
}
