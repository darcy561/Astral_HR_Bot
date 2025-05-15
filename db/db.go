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
	"strings"
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
	err := RedisDB.SRem(ctx, "trackedUsers", userID).Err()
	if err != nil {
		logger.Error(logger.LogData{
			"action":  "remove tracked user from redis",
			"message": "failed to remove tracked user from redis",
			"error":   err.Error(),
		})
		return err
	}

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

func IncreaseChannelCount(ctx context.Context, userID string, channelID string) error {

	err := RedisDB.ZIncrBy(ctx, "user:"+userID+":channels", 1, channelID).Err()

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

func DecreaseChannelCount(ctx context.Context, userID string, channelID string) error {

	err := RedisDB.ZIncrBy(ctx, "user:"+userID+":channels", -1, channelID).Err()

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

func mapToStruct[T any](fields map[string]string, target *T) error {
	v := reflect.ValueOf(target).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name
		value := fields[strings.ToLower(fieldName)]

		if value == "" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.String:
			v.Field(i).SetString(value)
		case reflect.Int64:
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				v.Field(i).SetInt(val)
			}
		case reflect.Int:
			if val, err := strconv.Atoi(value); err == nil {
				v.Field(i).SetInt(int64(val))
			}
		case reflect.Bool:
			if val, err := strconv.ParseBool(value); err == nil {
				v.Field(i).SetBool(val)
			}
		}
	}
	return nil
}

func GetUserAnalytics(ctx context.Context, userID string) (models.UserAnalytics, error) {
	key := "user:" + userID + ":monitoring"
	fields, err := RedisDB.HGetAll(ctx, key).Result()

	logger.Debug(logger.LogData{
		"action":  "get_user_analytics",
		"message": "getting user analytics",
		"user_id": userID,
		"fields":  fields,
	})

	if err != nil {
		return models.UserAnalytics{}, err
	}

	userAnalytics := models.UserAnalytics{
		UserID: userID,
	}

	// Convert string values to appropriate types
	if messages, ok := fields["messages"]; ok {
		if val, err := strconv.ParseInt(messages, 10, 64); err == nil {
			userAnalytics.Messages = val
		}
	}
	if voiceJoins, ok := fields["voice_joins"]; ok {
		if val, err := strconv.ParseInt(voiceJoins, 10, 64); err == nil {
			userAnalytics.VoiceJoins = val
		}
	}
	if invites, ok := fields["invites"]; ok {
		if val, err := strconv.ParseInt(invites, 10, 64); err == nil {
			userAnalytics.Invites = val
		}
	}

	// Get the top channel
	topChan, err := RedisDB.ZRevRangeWithScores(ctx, "user:"+userID+":channels", 0, 0).Result()
	if err == nil && len(topChan) > 0 {
		userAnalytics.TopChannelID = topChan[0].Member.(string)
	}

	return userAnalytics, nil
}
