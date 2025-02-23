package handlers

import "github.com/bwmarrin/discordgo"

type MessageCreateMiddleware func(s *discordgo.Session, m *discordgo.MessageCreate) bool
type GuildMemberAddMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberAdd) bool
type GuildMemberUpdateMiddleware func(s *discordgo.Session, a *discordgo.GuildMemberUpdate) bool
