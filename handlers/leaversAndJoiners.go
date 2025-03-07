package handlers

import (
	"astralHRBot/handlers/middleware"

	"github.com/bwmarrin/discordgo"
)

var guildMemberAddMiddleware = []GuildMemberAddMiddleware{
	middleware.SendMessageOnMemberJoin,
}
var guildMemberRemoveMiddleware = []GuildMemberRemoveMiddleware{
	middleware.SendMessageOnMemberLeave,
}

func MemberLeaversAndJoiners(s *discordgo.Session, event any) {
	switch e := event.(type) {
	case *discordgo.GuildMemberAdd:
		memberJoiningServerHandlers(s, e)
	case *discordgo.GuildMemberRemove:
		memberLeavingSererHandlers(s, e)
	}
}

func memberJoiningServerHandlers(d *discordgo.Session, m *discordgo.GuildMemberAdd) {
	for _, middleware := range guildMemberAddMiddleware {
		if !middleware(d, m) {
			return
		}
	}

}

func memberLeavingSererHandlers(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	for _, middleware := range guildMemberRemoveMiddleware {
		if !middleware(s, m) {
			return
		}
	}

}
