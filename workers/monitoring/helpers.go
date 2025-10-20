package monitoring

import (
	"astralHRBot/db"
	"astralHRBot/globals"
	"astralHRBot/logger"
	"astralHRBot/models"
	"context"
	"fmt"
	"time"
)

// EnsureScenarioWindow attaches a scenario to a user's monitoring record and
// persists the provided start and end times.
func EnsureScenarioWindow(ctx context.Context, userID string, scenario models.MonitoringScenario, start time.Time, end time.Time) error {
	// Add the scenario (creates monitoring record if needed via AddScenario)
	AddScenario(userID, scenario)

	md, err := db.GetUserMonitoring(ctx, userID)
	if err != nil || md == nil {
		return err
	}

	md.SetStartTime(start)
	md.SetExpiry(end)
	if err := db.SaveUserMonitoring(ctx, md); err != nil {
		logger.Error(logger.LogData{
			"action":   "ensure_scenario_window",
			"message":  "Failed to persist monitoring window",
			"error":    err.Error(),
			"user_id":  userID,
			"scenario": scenario,
		})
		return err
	}

	return nil
}

// BackfillMonitoringFromTasks infers scenarios and time window from existing tasks
// when monitoring data is missing or has no scenarios. It returns the updated
// monitoring data and the list of scenarios that were added.
func BackfillMonitoringFromTasks(ctx context.Context, userID string, monitoringData *models.UserMonitoring, tasks []models.Task) (*models.UserMonitoring, []models.MonitoringScenario, error) {
	scenariosAdded := []models.MonitoringScenario{}

	if monitoringData == nil {
		monitoringData = models.NewUserMonitoring(userID)
	}

	// Map task types to scenarios
	scenarioMap := map[models.TaskType]models.MonitoringScenario{
		models.TaskRecruitmentCleanup: models.MonitoringScenarioRecruitmentProcess,
		models.TaskUserCheckin:        models.MonitoringScenarioNewRecruit,
	}

	for _, t := range tasks {
		if sc, ok := scenarioMap[t.FunctionName]; ok {
			if !monitoringData.HasScenario(sc) {
				monitoringData.AddScenario(sc)
				scenariosAdded = append(scenariosAdded, sc)
			}
		}
	}

	// If we can infer a window, set it when not already set
	if monitoringData.ExpiresAt == 0 {
		var earliest int64
		for _, t := range tasks {
			if earliest == 0 || (t.ScheduledTime > 0 && t.ScheduledTime < earliest) {
				earliest = t.ScheduledTime
			}
		}
		if earliest > 0 {
			monitoringData.ExpiresAt = earliest
			defaultDays := 7
			if monitoringData.HasScenario(models.MonitoringScenarioNewRecruit) {
				defaultDays = globals.GetNewRecruitTrackingDays()
			} else if monitoringData.HasScenario(models.MonitoringScenarioRecruitmentProcess) {
				defaultDays = globals.GetRecruitmentCleanupDelay()
			}
			monitoringData.StartedAt = time.Unix(earliest, 0).Add(-time.Duration(defaultDays) * 24 * time.Hour).Unix()
		}
	}

	if err := db.SaveUserMonitoring(ctx, monitoringData); err != nil {
		return monitoringData, scenariosAdded, err
	}

	return monitoringData, scenariosAdded, nil
}

// CreateRecruitmentReminderAtMidpoint creates a recruitment reminder task at the midpoint
// of the recruitment process duration, but only if the midpoint is in the future.
func CreateRecruitmentReminderAtMidpoint(ctx context.Context, userID string, startTime time.Time, scenario models.MonitoringScenario) error {
	// Calculate midpoint: start + (delay * 12 hours)
	midpoint := startTime.Add(time.Duration(globals.GetRecruitmentCleanupDelay()) * 12 * time.Hour).Unix()

	// Only create the reminder if it's in the future
	if midpoint <= time.Now().Unix() {
		return nil
	}

	reminderParams := &models.RecruitmentReminderParams{UserID: userID}
	reminderTask, err := models.NewTaskWithScenario(
		models.TaskRecruitmentReminder,
		reminderParams,
		midpoint,
		string(scenario),
	)
	if err != nil {
		return fmt.Errorf("failed to create recruitment reminder task: %w", err)
	}

	if err := db.SaveTaskToRedis(ctx, *reminderTask); err != nil {
		return fmt.Errorf("failed to save recruitment reminder task: %w", err)
	}

	return nil
}
