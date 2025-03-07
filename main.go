package main

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
)

func main() {
	db.InitRedis()
	bot.Setup()

	discordAPIWorker.NewWorker(bot.Discord)
	eventWorker.NewWorker()
	bot.Start()
}
