package db

import (
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisDB *redis.Client

func InitRedis() {

	redisHost, exists := os.LookupEnv("REDIS_HOST")

	if !exists {
		logger.Error(logger.LogData{
			"action":  "redis_startup",
			"message": "Missing Redis Host",
		})
		os.Exit(1)
	}

	RedisDB = redis.NewClient(&redis.Options{
		Addr: redisHost,
	})

	_, err := RedisDB.Ping(context.Background()).Result()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "redis_startup",
			"message": "Failed to connect to Redis",
			"error":   err.Error(),
		})
		os.Exit(1)
	}

	logger.Info(logger.LogData{
		"action":  "redis_startup",
		"message": "Connection to Redis established.",
	})
}

func GetRedisClient() *redis.Client {
	if RedisDB == nil {
		logger.Error(logger.LogData{
			"action":  "redis_startup",
			"message": "Redis client is not initialised. Call InitRedis() first.",
		})
		os.Exit(1)
	}

	return RedisDB
}

func GetUserFromRedis(ctx context.Context, userID string) (*models.User, error) {

	key := "User:" + userID

	data, err := RedisDB.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user from redis %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no data found for key %s", key)
	}

	user := &models.User{}

	if DiscordID, exists := data["DiscordID"]; exists {
		user.DiscordID = DiscordID
	}

	if PreviousJoinDate, err := time.Parse(time.RFC3339, data["PreviousJoinDate"]); err == nil {
		user.PreviousJoinDate = PreviousJoinDate
	}
	if PreviousLeaveDate, err := time.Parse(time.RFC3339, data["PreviousLeaveDate"]); err == nil {
		user.PreviousLeaveDate = PreviousLeaveDate
	}
	if CurrentJoinDate, err := time.Parse(time.RFC3339, data["CurrentJoinDate"]); err == nil {
		user.CurrentJoinDate = CurrentJoinDate
	}
	if DateJoinedRecruitment, err := time.Parse(time.RFC3339, data["DateJoinedRecruitment"]); err == nil {
		user.DateJoinedRecruitment = DateJoinedRecruitment
	}

	return user, nil
}

func SaveUserToRedis(ctx context.Context, user *models.User) error {
	key := "User:" + user.DiscordID

	userMap, err := structToMap(user)
	if err != nil {
		return fmt.Errorf("error converting user struct: %w", err)
	}

	err = RedisDB.HSet(ctx, key, userMap).Err()
	if err != nil {
		return fmt.Errorf("error saving user struct to redis: %w", err)
	}

	return nil
}

func UpdateHashFields(ctx context.Context, key string, fields map[string]interface{}) error {

	convertedFields := convertMapToStrings(fields)

	err := RedisDB.HSet(ctx, key, convertedFields).Err()
	if err != nil {
		return fmt.Errorf("failed to update fields in Redis hash: %w", err)
	}

	return nil
}

func convertMapToStrings(fields map[string]interface{}) map[string]interface{} {

	converted := make(map[string]interface{})

	for key, value := range fields {
		switch v := value.(type) {
		case int:
			converted[key] = strconv.Itoa(v) // Convert int to string
		case float64:
			converted[key] = strconv.FormatFloat(v, 'f', 2, 64) // Convert float64 to string with 2 decimal places
		case bool:
			converted[key] = strconv.FormatBool(v) // Convert bool to "true"/"false" string
		case string:
			converted[key] = v // Keep string as is
		case time.Time:
			if v.IsZero() { // Check if time is blank (zero value)
				converted[key] = "" // Store blank time as an empty string
			} else {
				converted[key] = v.Format(time.RFC3339)
			}
		default:
			converted[key] = fmt.Sprintf("%v", v) // Generic fallback for unsupported types
		}
	}

	return converted

}

func structToMap(input interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	v := reflect.ValueOf(input)

	// Ensure we're working with a struct
	if v.Kind() == reflect.Ptr {
		v = v.Elem() // Dereference pointer
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input is not a struct")
	}

	// Loop through struct fields
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i).Interface()
		fieldName := field.Name // Use struct field name as the key

		// Convert values to strings for Redis compatibility
		switch v := value.(type) {
		case int:
			result[fieldName] = strconv.Itoa(v)
		case int64:
			result[fieldName] = strconv.FormatInt(v, 10)
		case float64:
			result[fieldName] = strconv.FormatFloat(v, 'f', 2, 64)
		case bool:
			result[fieldName] = strconv.FormatBool(v)
		case string:
			result[fieldName] = v
		case time.Time:
			if v.IsZero() { // Check if time is blank (zero value)
				result[fieldName] = "" // Store blank time as an empty string
			} else {
				result[fieldName] = v.Format(time.RFC3339)
			}
		default:
			result[fieldName] = fmt.Sprintf("%v", v) // Generic fallback
		}
	}

	return result, nil
}

func FetchLatestTasks(ctx context.Context) ([]models.Task, error) {
	taskQueue := "taskQueue"
	now := time.Now().Unix()

	taskIDs, err := RedisDB.ZRangeByScore(ctx, taskQueue, &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", now),
		Offset: 0,
		Count:  100,
	}).Result()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "fetch_latest_tasks",
			"message": "Failed to fetch latest tasks",
			"error":   err.Error(),
		})
		return nil, err
	}

	tasks := make([]models.Task, len(taskIDs))

	for i, taskID := range taskIDs {
		task, err := getTaskByID(ctx, taskID)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "fetch_latest_tasks",
				"message": "Failed to get task by ID",
				"error":   err.Error(),
			})
			return nil, err
		}
		tasks[i] = task
	}

	return tasks, nil
}

func getTaskByID(ctx context.Context, taskID string) (models.Task, error) {
	data, err := RedisDB.Get(ctx, "task:"+taskID).Result()
	if err != nil {
		return models.Task{}, err
	}
	var task models.Task
	err = json.Unmarshal([]byte(data), &task)
	return task, err
}

func SaveTaskToRedis(ctx context.Context, task models.Task) error {
	key := "task:" + task.TaskID

	// Marshal task to JSON before saving
	taskJSON, err := json.Marshal(task)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_task_to_redis",
			"message": "Failed to marshal task to JSON",
			"error":   err.Error(),
		})
		return err
	}

	err = RedisDB.Set(ctx, key, taskJSON, 0).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_task_to_redis",
			"message": "Failed to save task to redis",
			"error":   err.Error(),
		})
		return err
	}

	err = RedisDB.ZAdd(ctx, "taskQueue", redis.Z{
		Score:  float64(task.ScheduledTime),
		Member: task.TaskID,
	}).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_task_to_redis",
			"message": "Failed to add task to queue",
			"error":   err.Error(),
		})
		return err
	}

	logger.Debug(logger.LogData{
		"action":  "save_task_to_redis",
		"message": "Task saved to redis",
		"task_id": task.TaskID,
	})

	return nil
}

func DeleteTaskFromRedis(ctx context.Context, taskID string) error {
	err := RedisDB.Del(ctx, "task:"+taskID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "delete_task_from_redis",
			"message": "Failed to delete task from redis",
			"error":   err.Error(),
		})
		return err
	}

	err = RedisDB.ZRem(ctx, "taskQueue", taskID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "delete_task_from_redis",
			"message": "Failed to remove task from queue",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func GetTrackedUsers(ctx context.Context) ([]string, error) {

	users, err := RedisDB.SMembers(ctx, "trackedUsers").Result()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "get tracked users from redis",
			"message": "failed to retrieve tracked users",
			"error":   err.Error(),
		})
		return nil, err
	}

	return users, nil
}

func AddTrackedUser(ctx context.Context, userID string) error {
	err := RedisDB.SAdd(ctx, "trackedUsers", userID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "add tracked user to redis",
			"message": "failed to add tracked user to redis",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func RemoveTrackedUser(ctx context.Context, userID string) error {
	// Remove from tracked users set
	err := RedisDB.SRem(ctx, "trackedUsers", userID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "remove tracked user from redis",
			"message": "failed to remove tracked user from redis",
			"error":   err.Error(),
		})
		return err
	}

	// Clean up all user-related data
	err = CleanupUserData(ctx, userID)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "cleanup_user_data",
			"message": "failed to cleanup user data",
			"error":   err.Error(),
			"user_id": userID,
		})
		return err
	}

	return nil
}

func CleanupUserData(ctx context.Context, userID string) error {
	// Clean up monitoring sessions
	userSessionsKey := fmt.Sprintf("user:%s:monitoring_sessions", userID)
	sessionKeys, err := RedisDB.SMembers(ctx, userSessionsKey).Result()
	if err == nil {
		// Delete all monitoring sessions
		for _, sessionKey := range sessionKeys {
			RedisDB.Del(ctx, sessionKey)
		}
		// Remove the sessions set
		RedisDB.Del(ctx, userSessionsKey)
	}

	// Clean up analytics and channel activity for all scenarios
	scenarios := []string{"new_recruit", "recruitment_process"}
	for _, scenario := range scenarios {
		// Clean up analytics hash
		analyticsKey := fmt.Sprintf("user:%s:analytics:%s", userID, scenario)
		RedisDB.Del(ctx, analyticsKey)

		// Clean up channel activity sorted set
		channelsKey := fmt.Sprintf("user:%s:channels:%s", userID, scenario)
		RedisDB.Del(ctx, channelsKey)
	}

	logger.Debug(logger.LogData{
		"action":  "cleanup_user_data",
		"message": "cleaned up all user data",
		"user_id": userID,
	})

	return nil
}

func IncreaseAttributeCount(ctx context.Context, key string, attribute string, amount int) error {

	err := RedisDB.HIncrBy(ctx, key, attribute, int64(amount)).Err()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_attribute_count",
			"message": "failed to increase attribute count",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func DecreaseAttributeCount(ctx context.Context, key string, attribute string, amount int) error {

	err := RedisDB.HIncrBy(ctx, key, attribute, int64(-amount)).Err()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "decrease_attribute_count",
			"message": "failed to decrease attribute count",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func IncreaseChannelCount(ctx context.Context, userID string, channelID string, scenario string) error {
	channelsKey := fmt.Sprintf("user:%s:channels:%s", userID, scenario)
	err := RedisDB.ZIncrBy(ctx, channelsKey, 1, channelID).Err()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "increase_channel_count",
			"message": "failed to increase channel count",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func DecreaseChannelCount(ctx context.Context, userID string, channelID string, scenario string) error {
	channelsKey := fmt.Sprintf("user:%s:channels:%s", userID, scenario)
	err := RedisDB.ZIncrBy(ctx, channelsKey, -1, channelID).Err()

	if err != nil {
		logger.Error(logger.LogData{
			"action":  "decrease_channel_count",
			"message": "failed to decrease channel count",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func GetUserAnalytics(ctx context.Context, userID string) (models.UserAnalytics, error) {
	// Get all monitoring sessions for this user to find active scenarios
	userSessionsKey := fmt.Sprintf("user:%s:monitoring_sessions", userID)
	sessionKeys, err := RedisDB.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return models.UserAnalytics{}, err
	}

	userAnalytics := models.UserAnalytics{
		UserID: userID,
	}

	// Aggregate analytics from all active scenarios
	scenarioAnalytics := make(map[string]int64) // field -> total count

	for _, sessionKey := range sessionKeys {
		// Get session data to find scenarios
		data, err := RedisDB.Get(ctx, sessionKey).Result()
		if err != nil {
			if err == redis.Nil {
				continue // Session expired
			}
			continue
		}

		var session models.UserMonitoring
		err = json.Unmarshal([]byte(data), &session)
		if err != nil {
			continue
		}

		// Check if session is expired
		if session.IsExpired() {
			continue
		}

		// Get analytics for each scenario in this session
		for scenario := range session.Scenarios {
			scenarioKey := fmt.Sprintf("user:%s:analytics:%s", userID, scenario)
			fields, err := RedisDB.HGetAll(ctx, scenarioKey).Result()
			if err != nil {
				continue
			}

			// Aggregate the fields
			for field, value := range fields {
				if val, err := strconv.ParseInt(value, 10, 64); err == nil {
					scenarioAnalytics[field] += val
				}
			}
		}
	}

	// Map aggregated fields to UserAnalytics struct
	if messages, ok := scenarioAnalytics["messages"]; ok {
		userAnalytics.Messages = messages
	}
	if voiceJoins, ok := scenarioAnalytics["voice_joins"]; ok {
		userAnalytics.VoiceJoins = voiceJoins
	}
	if invites, ok := scenarioAnalytics["invites"]; ok {
		userAnalytics.Invites = invites
	}

	// Get the top channel from the most active scenario
	// For now, we'll use the first scenario found, but this could be enhanced
	// to aggregate or choose the most relevant scenario
	for _, sessionKey := range sessionKeys {
		data, err := RedisDB.Get(ctx, sessionKey).Result()
		if err != nil {
			continue
		}

		var session models.UserMonitoring
		err = json.Unmarshal([]byte(data), &session)
		if err != nil {
			continue
		}

		if session.IsExpired() {
			continue
		}

		// Get the top channel for the first active scenario
		for scenario := range session.Scenarios {
			channelsKey := fmt.Sprintf("user:%s:channels:%s", userID, scenario)
			topChan, err := RedisDB.ZRevRangeWithScores(ctx, channelsKey, 0, 0).Result()
			if err == nil && len(topChan) > 0 {
				userAnalytics.TopChannelID = topChan[0].Member.(string)
				break // Use the first scenario's top channel
			}
		}
		break // Only check the first valid session
	}

	logger.Debug(logger.LogData{
		"action":    "get_user_analytics",
		"message":   "aggregated user analytics from scenarios",
		"user_id":   userID,
		"analytics": scenarioAnalytics,
	})

	return userAnalytics, nil
}

func InitializeScenarioAnalytics(ctx context.Context, userID string, scenario models.MonitoringScenario) error {
	key := fmt.Sprintf("user:%s:analytics:%s", userID, scenario)

	// Get the actions for this scenario from the config
	actions, exists := models.ScenarioConfig[scenario]
	if !exists {
		return fmt.Errorf("unknown scenario: %s", scenario)
	}

	// Initialize analytics hash with fields based on scenario actions
	initialFields := make(map[string]interface{})

	// Map actions to their corresponding counter fields
	actionToField := map[models.MonitoringAction]string{
		models.ActionMessageCreate:  "messages",
		models.ActionMessageEdit:    "message_edits",
		models.ActionMessageDelete:  "message_deletes",
		models.ActionVoiceJoin:      "voice_joins",
		models.ActionVoiceLeave:     "voice_leaves",
		models.ActionInviteCreate:   "invites",
		models.ActionReactionAdd:    "reactions_added",
		models.ActionReactionRemove: "reactions_removed",
	}

	// Only initialize fields for actions that this scenario tracks
	for _, action := range actions {
		if field, exists := actionToField[action]; exists {
			initialFields[field] = "0"
		}
	}

	err := RedisDB.HMSet(ctx, key, initialFields).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":   "initialize_scenario_analytics",
			"message":  "failed to initialize scenario analytics hash",
			"error":    err.Error(),
			"scenario": string(scenario),
		})
		return err
	}

	logger.Debug(logger.LogData{
		"action":   "initialize_scenario_analytics",
		"message":  "initialized analytics hash for scenario",
		"user_id":  userID,
		"scenario": string(scenario),
		"fields":   initialFields,
	})

	return nil
}

func SaveUserMonitoring(ctx context.Context, monitoring *models.UserMonitoring) error {
	// Store monitoring session as JSON with unique key
	sessionKey := fmt.Sprintf("user:%s:monitoring:%d", monitoring.UserID, monitoring.StartedAt)

	data, err := json.Marshal(monitoring)
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_user_monitoring",
			"message": "failed to marshal monitoring data",
			"error":   err.Error(),
		})
		return err
	}

	err = RedisDB.Set(ctx, sessionKey, string(data), 0).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_user_monitoring",
			"message": "failed to save monitoring session to redis",
			"error":   err.Error(),
		})
		return err
	}

	// Add session to user's monitoring sessions set
	userSessionsKey := fmt.Sprintf("user:%s:monitoring_sessions", monitoring.UserID)
	err = RedisDB.SAdd(ctx, userSessionsKey, sessionKey).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_user_monitoring",
			"message": "failed to add session to user sessions set",
			"error":   err.Error(),
		})
		return err
	}

	// Initialize analytics for each scenario
	for scenario := range monitoring.Scenarios {
		err = InitializeScenarioAnalytics(ctx, monitoring.UserID, scenario)
		if err != nil {
			logger.Error(logger.LogData{
				"action":   "save_user_monitoring",
				"message":  "failed to initialize scenario analytics",
				"error":    err.Error(),
				"scenario": string(scenario),
			})
			return err
		}
	}

	// Add to tracked users set if not already present
	err = RedisDB.SAdd(ctx, "trackedUsers", monitoring.UserID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "save_user_monitoring",
			"message": "failed to add user to tracked users set",
			"error":   err.Error(),
		})
		return err
	}

	return nil
}

func GetUserMonitoring(ctx context.Context, userID string) (*models.UserMonitoring, error) {
	// Get all monitoring sessions for this user
	userSessionsKey := fmt.Sprintf("user:%s:monitoring_sessions", userID)
	sessionKeys, err := RedisDB.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "get_user_monitoring",
			"message": "failed to get user monitoring sessions",
			"error":   err.Error(),
		})
		return nil, err
	}

	if len(sessionKeys) == 0 {
		return nil, nil
	}

	// For now, return the most recent session (highest timestamp)
	// In the future, we might want to return all active sessions
	var latestSession *models.UserMonitoring
	var latestTimestamp int64 = 0

	for _, sessionKey := range sessionKeys {
		data, err := RedisDB.Get(ctx, sessionKey).Result()
		if err != nil {
			if err == redis.Nil {
				// Session expired, remove from set
				RedisDB.SRem(ctx, userSessionsKey, sessionKey)
				continue
			}
			logger.Error(logger.LogData{
				"action":  "get_user_monitoring",
				"message": "failed to get monitoring session data",
				"error":   err.Error(),
			})
			continue
		}

		var session models.UserMonitoring
		err = json.Unmarshal([]byte(data), &session)
		if err != nil {
			logger.Error(logger.LogData{
				"action":  "get_user_monitoring",
				"message": "failed to unmarshal monitoring session",
				"error":   err.Error(),
			})
			continue
		}

		// Check if session is expired
		if session.IsExpired() {
			// Remove expired session
			RedisDB.Del(ctx, sessionKey)
			RedisDB.SRem(ctx, userSessionsKey, sessionKey)
			continue
		}

		// Keep track of the latest session
		if session.StartedAt > latestTimestamp {
			latestTimestamp = session.StartedAt
			latestSession = &session
		}
	}

	return latestSession, nil
}

func SetUserPresence(ctx context.Context, key string, data string) error {
	err := RedisDB.Set(ctx, key, data, 24*time.Hour).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "set_user_presence",
			"message": "failed to set user presence in redis",
			"error":   err.Error(),
		})
		return err
	}
	return nil
}
