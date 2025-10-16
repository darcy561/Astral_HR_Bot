package handlers

import (
	"astralHRBot/handlers/middleware"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"
	"astralHRBot/workers/monitoring"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var guildMemberAddMiddleware = []GuildMemberAddMiddleware{
	middleware.CreateOrUpdateUserMiddleware,
	middleware.SendMessageOnMemberJoin,
}
var guildMemberRemoveMiddleware = []GuildMemberRemoveMiddleware{
	middleware.SendMessageOnMemberLeave,
}

func MemberLeaversAndJoiners(s *discordgo.Session, d any) {
	switch t := d.(type) {
	case *discordgo.GuildMemberAdd:
		eventWorker.Submit(t.User.ID, memberJoiningServerHandlers, s, t)
	case *discordgo.GuildMemberRemove:
		eventWorker.Submit(t.User.ID, memberLeavingSererHandlers, s, t)
	}
}

func memberJoiningServerHandlers(e eventWorker.Event) {
	p, t := e.Payload, e.TraceID

	if len(p) < 2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "invalid_args",
			"message":  "handle role changes: invalid arguments",
		})
		return
	}

	s, ok1 := p[0].(*discordgo.Session)
	m, ok2 := p[1].(*discordgo.GuildMemberAdd)

	if !ok1 || !ok2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "type_assertion_failed",
			"message":  "handle role changes: type assertion failed",
		})
		return
	}

	for _, middleware := range guildMemberAddMiddleware {
		if !middleware(s, m, e) {
			return
		}
	}
}

func memberLeavingSererHandlers(e eventWorker.Event) {
	p, t := e.Payload, e.TraceID

	if len(p) < 2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "invalid_args",
			"message":  "handle role changes: invalid arguments",
		})
		return
	}

	s, ok1 := p[0].(*discordgo.Session)
	m, ok2 := p[1].(*discordgo.GuildMemberRemove)

	if !ok1 || !ok2 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "type_assertion_failed",
			"message":  "handle role changes: type assertion failed",
		})
		return
	}
	for _, middleware := range guildMemberRemoveMiddleware {
		if !middleware(s, m, e) {
			return
		}
	}

	//close a recruitment thread if its open and assign the "Left Server" tag
	rtm := helper.NewRecruitmentThreadManager(s, e, m.User.ID)
	rtm.SendMessageAndClose(fmt.Sprintf("%s left the server.", m.User.GlobalName), "Left Server")

	//clear any monitoring or events for the user
	monitoring.RemoveAllScenarios(m.User.ID)

}
