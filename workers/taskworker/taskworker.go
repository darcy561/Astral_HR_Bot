package taskworker

import (
	"astralHRBot/bot"
	"astralHRBot/db"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"time"
)

// StartTaskProcessor starts a background goroutine that processes tasks every 5 seconds
func StartTaskProcessor() {
	go func() {
		// Wait for Discord to be ready
		<-bot.ReadyChan
		logger.Info(logger.LogData{
			"action":  "task_processor",
			"message": "Discord connection established, starting task processing",
		})

		for {
			taskList, err := db.FetchLatestTasks(context.Background())

			if err != nil {
				logger.Error(logger.LogData{
					"action":  "start_task_processor",
					"message": "Failed to get tasks",
					"error":   err.Error(),
				})
				time.Sleep(5 * time.Second)
				continue
			}

			for _, task := range taskList {
				// Get handler for this task type from models
				handler, exists := models.TaskHandlers[task.FunctionName]
				if !exists {
					logger.Error(logger.LogData{
						"action":    "process_task",
						"task_id":   task.TaskID,
						"task_type": string(task.FunctionName),
						"message":   "No handler found for task type",
					})
					continue
				}

				// Execute handler in goroutine
				go handler(task)
			}

			time.Sleep(5 * time.Second)
		}
	}()
}
