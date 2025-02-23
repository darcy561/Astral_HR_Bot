package handlers

import "github.com/bwmarrin/discordgo"

var guildMemberAddMiddleware = []GuildMemberAddMiddleware{}

func MemberJoiningServerHandlers(discord *discordgo.Session, member *discordgo.GuildMemberAdd) {
	for _, middleware := range guildMemberAddMiddleware {
		if middleware(discord, member) {
			return
		}
	}

}
