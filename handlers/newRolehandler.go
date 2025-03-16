package handlers

import (
	"astralHRBot/channels"
	"astralHRBot/helper"
	"astralHRBot/logger"
	"astralHRBot/roles"
	discordAPIWorker "astralHRBot/workers/discordAPI"
	"astralHRBot/workers/eventWorker"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandleRoleGained(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) {

	if welcomeNewRecruit(s, m, a, e) {
		return
	}
	if recruitAuthenticated(s, m, a, e) {
		return
	}
	if newMemberOnboarding(s, m, a, e) {
		return
	}
}

func welcomeNewRecruit(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {
	if hasRole(a, roles.GetRoleID("newbro-7102")) && !hasRole(m.Roles, roles.GetRoleID("server_clown-3309")) {
		logger.Debug(e.TraceID,
			"welcome new recruit process started: "+m.User.ID)

		channelID := channels.GetChannelID("recruitment-8356")
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

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(e.TraceID,
				"welcome message sent: "+m.User.ID)

			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		if hasRole(m.Roles, roles.GetRoleID("newcomer-9439")) {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"remove newcomer role: "+m.User.ID)

				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID("newcomer-9439"))
				return err
			})
		}

		recruitmentChannelID := channels.GetChannelID("recruitment_forum-1311")
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)

		if !found {
			discordAPIWorker.NewRequest(e, func() error {
				newThreadTitle := fmt.Sprintf("%s - %s", m.User.GlobalName, m.User.ID)
				logger.Debug(e.TraceID,
					"create recruitment forum thread: "+m.User.ID,
				)

				_, err := s.ForumThreadStart(recruitmentChannelID, newThreadTitle, 10080, fmt.Sprintf("%s Joined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})

		} else {

			discordAPIWorker.NewRequest(e, func() error {

				logger.Debug(e.TraceID,
					"reopen recruitment forum thread:  "+m.User.ID,
				)

				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					AutoArchiveDuration: 0,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"add message to recruitment thread: "+m.User.ID,
				)
				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Rejoined Recruitment", m.User.GlobalName))
				if err != nil {
					return err
				}
				return nil
			})

		}
		logger.Debug(e.TraceID,
			"welcome new recruit process ended: "+m.User.ID,
		)
		return true
	}
	return false
}

func recruitAuthenticated(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {

	if hasRole(m.Roles, roles.GetRoleID("newbro-7102")) && hasRole(a, roles.GetRoleID("authenticated_guest-1333")) {
		logger.Debug(e.TraceID,
			"recruit authenticated process started: "+m.User.ID,
		)

		discordAPIWorker.NewRequest(e, func() error {
			logger.Debug(e.TraceID,
				"add message to recruitment thread: "+m.User.ID,
			)

			_, err := s.ChannelMessageSend(channels.GetChannelID("recruitment_hub-3185"), fmt.Sprintf("%s has completed the authentication steps.", m.Member.DisplayName()))
			if err != nil {
				return err
			}
			return nil
		})

		recruitmentChannelID := channels.GetChannelID("recruitment_forum-1311")
		recruitmentThread, found := helper.FindForumThreadByTitle(s, recruitmentChannelID, m.User.ID)
		if found {
			updatedThreadTitle := fmt.Sprintf("%s - %s", m.Member.DisplayName(), m.User.ID)

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"recruitment thead updated: "+m.User.ID,
				)

				_, err := s.ChannelEditComplex(recruitmentThread.ID, &discordgo.ChannelEdit{
					Name: updatedThreadTitle,
				})
				if err != nil {
					return err
				}
				return nil
			})

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"channel message sent: "+m.User.ID,
				)

				_, err := s.ChannelMessageSend(recruitmentThread.ID, fmt.Sprintf("%s Authentication Steps Complete.", m.Member.DisplayName()))
				if err != nil {
					return err
				}
				return nil
			})

		} else {
			logger.Info(e.TraceID,
				"no existing recruitment thread found for: "+m.User.ID,
			)
		}
		logger.Debug(e.TraceID,
			"recruit authenticated process complete: "+m.User.ID,
		)
		return true
	}

	return false

}

func newMemberOnboarding(s *discordgo.Session, m *discordgo.GuildMemberUpdate, a []string, e eventWorker.Event) bool {

	if (hasRole(m.Roles, roles.GetRoleID("newbro-7102")) || hasRole(m.Roles, roles.GetRoleID("authenticated_guest-1333"))) && hasRole(a, roles.GetRoleID("authenticated_member-6454")) {

		logger.Debug(e.TraceID,
			"new member onboarding process started: "+m.User.ID,
		)

		rolesToRemove := []string{
			"newcomer-9439", "newbro-7102", "guest-4128", "legacy_guest-9234",
		}

		for _, role := range rolesToRemove {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"remove role: "+role,
				)
				err := s.GuildMemberRoleRemove(m.GuildID, m.User.ID, roles.GetRoleID(role))
				return err
			})
		}

		for _, role := range roles.ContentNotificationRoles {
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"add role: "+role,
				)
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

		channelID := channels.GetChannelID("general-5953")
		discordAPIWorker.NewRequest(e, func() error {

			logger.Debug(e.TraceID,
				"welcome message sent: "+m.User.ID,
			)
			_, err := s.ChannelMessageSend(channelID, message)
			return err
		})

		recruitmentChannelID := channels.GetChannelID("recruitment_forum-1311")
		recruitmentChannel, err := s.Channel(recruitmentChannelID)
		if err != nil {
			logger.Warn(e.TraceID,
				"Failed to fetch recruitment channel: "+err.Error(),
			)
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

			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"add message to recruitment thread: "+m.User.ID,
				)

				_, err := s.ChannelMessageSend(recruitmentThread.ID, "Character Joined Corp")
				return err
			})

			isArchived := true
			discordAPIWorker.NewRequest(e, func() error {
				logger.Debug(e.TraceID,
					"modify recruitment thread: "+m.User.ID,
				)
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
		logger.Debug(e.TraceID,
			"new member onboarding process complete: "+m.User.ID,
		)
		return true

	}

	return false
}
