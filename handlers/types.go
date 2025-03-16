package handlers

import (
	"astralHRBot/workers/eventWorker"

	"github.com/bwmarrin/discordgo"
)

type MessageCreateMiddleware func(s *discordgo.Session, m *discordgo.MessageCreate, e eventWorker.Event) bool
type GuildMemberAddMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberAdd, e eventWorker.Event) bool
type GuildMemberUpdateMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberUpdate, e eventWorker.Event) bool
type GuildMemberRemoveMiddleware func(s *discordgo.Session, r *discordgo.GuildMemberRemove, e eventWorker.Event) bool
