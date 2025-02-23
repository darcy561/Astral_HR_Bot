package main

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	discordAPIWorker "astralHRBot/workers/discordAPI"
)

func main() {
	db.InitRedis()
	bot.Setup()
	discordAPIWorker.NewWorker(bot.Discord)
	bot.Start()
}
