package models

import (
	"encoding/json"
	"fmt"
)

// TaskType represents the type of task
type TaskType string

const (
	TaskRecruitmentCleanup TaskType = "recruitmentCleanup"
	TaskUserCheckin        TaskType = "userCheckin"
)

// TaskTypeMap maps task types to their parameter types
var TaskTypeMap = map[TaskType]func() TaskParams{
	TaskRecruitmentCleanup: func() TaskParams { return &RecruitmentCleanupParams{} },
	TaskUserCheckin:        func() TaskParams { return &UserCheckinParams{} },
}

// TaskParams is an interface that all function-specific parameter structs must implement
type TaskParams interface {
	Validate() error
}

// Task represents a scheduled task in the system
type Task struct {
	TaskID        string          `json:"task_id"`
	FunctionName  TaskType        `json:"function_name"`
	Params        json.RawMessage `json:"params"` // Store raw JSON to be unmarshaled into specific param types
	ScheduledTime int64           `json:"scheduled_time"`
	Status        string          `json:"status"`
	Retries       int             `json:"retries"`
	CreatedBy     string          `json:"created_by"`
}

// GetParams unmarshals the raw params into the appropriate struct type
func (t *Task) GetParams() (TaskParams, error) {
	paramCreator, exists := TaskTypeMap[t.FunctionName]
	if !exists {
		return nil, fmt.Errorf("unknown function type: %s", t.FunctionName)
	}

	params := paramCreator()
	if err := json.Unmarshal(t.Params, params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}

	return params, nil
}

type RecruitmentCleanupParams struct {
	UserID string `json:"user_id"`
}

func (p *RecruitmentCleanupParams) Validate() error {
	if p.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	return nil
}

type UserCheckinParams struct {
	UserID string `json:"user_id"`
}

func (p *UserCheckinParams) Validate() error {
	if p.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	return nil
}
