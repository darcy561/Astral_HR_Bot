package middleware

import (
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

// MessageCreateMiddleware handles message creation events
type MessageCreateMiddleware func(s *discordgo.Session, m *discordgo.MessageCreate, e eventWorker.Event) bool

// GuildMemberAddMiddleware handles member join events
type GuildMemberAddMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberAdd, e eventWorker.Event) bool

// GuildMemberUpdateMiddleware handles member update events
type GuildMemberUpdateMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberUpdate, e eventWorker.Event) bool

// GuildMemberRemoveMiddleware handles member leave events
type GuildMemberRemoveMiddleware func(s *discordgo.Session, r *discordgo.GuildMemberRemove, e eventWorker.Event) bool
