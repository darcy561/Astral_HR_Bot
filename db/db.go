package db

import (
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
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

func GetUserFromRedis(ctx context.Context, userID int) (*models.User, error) {

	key := "User:" + strconv.Itoa(userID)

	data, err := RedisDB.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user from redis %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no data found for key %s", key)
	}

	user := &models.User{}

	if DiscordID, err := strconv.ParseInt(data["DiscordID"], 10, 64); err == nil {
		user.DiscordID = int(DiscordID)
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
	key := "User:" + strconv.Itoa(user.DiscordID)

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
