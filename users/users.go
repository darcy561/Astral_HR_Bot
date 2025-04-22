package users

import (
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/models"
	"astralHRBot/workers/eventWorker"
	"context"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

func CreateOrUpdateUser(e eventWorker.Event) {
	p, t := e.Payload, e.TraceID

	if len(p) < 1 {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "invalid_args",
			"message":  "handle user creation: invalid arguments",
		})
		return
	}

	user, ok := p[0].(*discordgo.User)
	if !ok {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "type_assertion_failed",
			"message":  "handle user creation: type assertion failed",
		})
		return
	}

	discordID, err := strconv.Atoi(user.ID)
	if err != nil {
		logger.Error(logger.LogData{
			"trace_id": t,
			"action":   "user_creation",
			"message":  "Failed to convert Discord ID to integer",
			"error":    err.Error(),
			"user_id":  user.ID,
		})
		return
	}

	ctx := context.Background()
	existingUser, err := db.GetUserFromRedis(ctx, discordID)
	if err != nil {
		newUser := &models.User{
			DiscordID:          discordID,
			CurrentDisplayName: user.GlobalName,
			CurrentJoinDate:    time.Now(),
			Monitored:          false,
		}

		err = db.SaveUserToRedis(ctx, newUser)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": t,
				"action":   "create_user",
				"message":  "Failed to save new user to Redis",
				"error":    err.Error(),
				"user_id":  user.ID,
			})
			return
		}

		logger.Info(logger.LogData{
			"trace_id": t,
			"action":   "create_user",
			"message":  "Created new user in Redis",
			"user_id":  user.ID,
		})
	} else {
		existingUser.PreviousJoinDate = existingUser.CurrentJoinDate
		existingUser.CurrentJoinDate = time.Now()
		existingUser.CurrentDisplayName = user.GlobalName

		err = db.SaveUserToRedis(ctx, existingUser)
		if err != nil {
			logger.Error(logger.LogData{
				"trace_id": t,
				"action":   "update_user",
				"message":  "Failed to update user in Redis",
				"error":    err.Error(),
				"user_id":  user.ID,
			})
			return
		}

		logger.Info(logger.LogData{
			"trace_id": t,
			"action":   "update_user",
			"message":  "Updated user in Redis",
			"user_id":  user.ID,
			"details": map[string]interface{}{
				"previous_join_date": existingUser.PreviousJoinDate,
				"current_join_date":  existingUser.CurrentJoinDate,
			},
		})
	}
}

func UpdateRecruitmentDate(userID string) error {
	ctx := context.Background()

	discordID, err := strconv.Atoi(userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "user_update",
			"message": "Failed to convert Discord ID to integer",
			"error":   err.Error(),
			"user_id": userID,
		})
		return err
	}

	key := "User:" + strconv.Itoa(discordID)
	fields := map[string]any{
		"DateJoinedRecruitment": time.Now(),
	}

	err = db.UpdateHashFields(ctx, key, fields)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "user_update",
			"message": "Failed to update recruitment date in Redis",
			"error":   err.Error(),
			"user_id": userID,
		})
		return err
	}

	logger.Info(logger.LogData{
		"action":  "user_update",
		"message": "Updated user recruitment date",
		"user_id": userID,
		"details": map[string]any{
			"recruitment_date": time.Now(),
		},
	})

	return nil
}
