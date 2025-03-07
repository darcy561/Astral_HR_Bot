package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/roles"

	"github.com/bwmarrin/discordgo"
)

func ManageGuildChanges(s *discordgo.Session, event any) {
	switch e := event.(type) {
	case *discordgo.ChannelCreate:
		channels.AddChannelToMap(e.Channel)
	case *discordgo.ChannelDelete:
		channels.RemoveChannelFromMap(e.Channel)
	case *discordgo.ChannelUpdate:
		channels.UpdateChannel(e.Channel)
	case *discordgo.GuildRoleCreate:
		roles.AddRoleToMap(e.Role)
	case *discordgo.GuildRoleDelete:
		roles.RemoveRoleFromMap(e.RoleID)
	case *discordgo.GuildRoleUpdate:
		roles.UpdateRole(e.Role)
	}
}
