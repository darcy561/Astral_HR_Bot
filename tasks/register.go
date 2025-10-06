package tasks

import (
	"astralHRBot/logger"
	"astralHRBot/models"
)

// RegisterHandlers registers all task handlers with the models package
func RegisterHandlers() {
	models.TaskHandlers[models.TaskRecruitmentCleanup] = ProcessRecruitmentCleanup
	models.TaskHandlers[models.TaskUserCheckin] = ProcessUserCheckin

	logger.Info(logger.LogData{
		"action":  "register_handlers",
		"message": "Task handlers registered",
		"count":   len(models.TaskHandlers),
	})
}
