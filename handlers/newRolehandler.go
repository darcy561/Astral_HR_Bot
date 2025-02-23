package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/helper"
	"astralHRBot/roles"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleGained(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string) {

	if welcomeNewRecruit(s, m, a) {
		return
	}
	if recruitAuthenticated(s, m, a) {
		return
	}
	if newMemberOnboarding(s, m, a) {
		return
	}
}

func welcomeNewRecruit(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string) bool {
	if hasRole(a, roles.GetRoleID("newbro")) && !hasRole(m.Roles, roles.GetRoleID("server_clown")) {
		channelID := channels.GetChannelID("recruitment")
		message := fmt.Sprintf(
			"Welcome <@%s>! \n\n"+
				"A member of the recruitment team will be with you shortly. In the meantime, please follow these steps:\n\n"+
				"[Alliance Auth](https://auth.astralinc.space/)\n\n"+
				"* Follow the above link and register your character(s).\n"+
				"* In the **Char Link** tab, authorize each of your characters.\n"+
				"* In the **Member Audit** tab, register each of your characters.\n"+
				"* In the **Services** tab, click the checkbox to link your Discord account.\n\n"+
				"Once you've completed this, a green tick should appear next to your character name on Discord.",
			m.User.ID,
		)

		discordAPIWorker.NewRequest(func() error {
			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		if hasRole(m.Roles, roles.GetRoleID("newcomer")) {
			discordAPIWorker.NewRequest(func() error {
				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID("newcomer"))
				return err
			})
		}

		recruitmentChannelID := channels.GetChannelID("recruitment_forum")
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		if !found {
			discordAPIWorker.NewRequest(func() error {
				newThreadTitle := fmt.Sprintf("%s - %s", m.User.GlobalName, m.User.ID)

				_, err := s.ForumThreadStart(recruitmentChannelID, newThreadTitle, 10080, fmt.Sprintf("%s Joined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})

		} else {

			discordAPIWorker.NewRequest(func() error {
				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					AutoArchiveDuration: 0,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(func() error {
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Rejoined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})

		}

		return true
	}
	return false
}

func recruitAuthenticated(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string) bool {

	if hasRole(m.Roles, roles.GetRoleID("newbro")) && hasRole(a, roles.GetRoleID("authenticated_guest")) {

		discordAPIWorker.NewRequest(func() error {
			_, err := s.ChannelMessageSend(channels.GetChannelID("recruitment_hub"), fmt.Sprintf("%s has completed the authentication steps.", m.Member.DisplayName()))
			if err != nil {
				return err
			}
			return nil
		})

		recruitmentChannelID := channels.GetChannelID("recruitment_forum")
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)
		if found {
			updatedThreadTitle := fmt.Sprintf("%s - %s", m.Member.DisplayName(), m.User.ID)

			discordAPIWorker.NewRequest(func() error {
				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					Name: updatedThreadTitle,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(func() error {
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Authentication Steps Complete.", m.Member.DisplayName()))
				if err != nil {
					return err
				}
				return nil
			})

		}

		return true
	}

	return false

}

func newMemberOnboarding(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string) bool {

	if (hasRole(m.Roles, roles.GetRoleID("newbro")) || hasRole(m.Roles, roles.GetRoleID("authenticated_guest"))) && hasRole(a, roles.GetRoleID("authenticated_member")) {

		rolesToRemove := []string{
			"newcomer", "newbro", "guest", "legacy_guest",
		}

		for _, role := range rolesToRemove {
			discordAPIWorker.NewRequest(func() error {
				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID(role))
				return err
			})
		}

		for _, role := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(func() error {
				err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roles.GetRoleID(role))
				return err
			})
		}

		message := fmt.Sprintf(
			"Welcome to Astral, %s <@%s> o/ \n\n"+
				"Please take a look at <#1229904357697261569> for guides, and specifically the newbro doc for info on our region.\n\n"+
				"If you need a hand moving your stuff around, feel free to head over to <#1082494747937087581> to speak with them directly.\n\n"+
				"Most importantly, head over to <#1161264045584822322> to opt out of the content pings that do not interest you.\n\n"+
				"Clear skies,\n"+
				"And KTF!",
			m.Member.DisplayName(), m.User.ID,
		)

		channelID := channels.GetChannelID("general")
		discordAPIWorker.NewRequest(func() error {
			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		recruitmentChannelID := channels.GetChannelID("recruitment_forum")
		recruitmentChannel, err := s.Channel(recruitmentChannelID)
		if err != nil {
			log.Printf("Failed to fetch recruitment channel: %v", err)
		}

		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		tagsToApply := []string{}
		if recruitmentChannel != nil {
			for _, tag := range recruitmentChannel.AvailableTags {
				if tag.Name == "Accepted" {
					tagsToApply = append(tagsToApply, tag.ID)
					break
				}
			}
		}

		if found {
			isArchived := true
			discordAPIWorker.NewRequest(func() error {
				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					Archived:    &isArchived,
					AppliedTags: &tagsToApply,
				})
				if err != nil {
					return err
				}
				return nil
			})
		}

		return true

	}

	return false
}
