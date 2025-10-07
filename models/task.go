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

// UserTaskParams is an interface for task parameters that include a user ID
type UserTaskParams interface {
	TaskParams
	GetUserID() string
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
	Scenario      string          `json:"scenario,omitempty"` // Optional scenario that created this task
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

// IsForUser checks if this task is for a specific user
func (t *Task) IsForUser(userID string) bool {
	params, err := t.GetParams()
	if err != nil {
		return false
	}

	if userParams, ok := params.(UserTaskParams); ok {
		return userParams.GetUserID() == userID
	}

	return false
}

// IsForScenario checks if this task was created by a specific scenario
func (t *Task) IsForScenario(scenario string) bool {
	return t.Scenario == scenario
}

// NewTaskWithScenario creates a new task with scenario information
func NewTaskWithScenario(functionName TaskType, params TaskParams, scheduledTime int64, scenario string) (*Task, error) {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Marshal parameters to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	// Generate task ID (you might want to use a UUID library for this)
	taskID := fmt.Sprintf("%s_%d", functionName, scheduledTime)

	return &Task{
		TaskID:        taskID,
		FunctionName:  functionName,
		Params:        paramsJSON,
		ScheduledTime: scheduledTime,
		Status:        "pending",
		Retries:       0,
		CreatedBy:     "system",
		Scenario:      scenario,
	}, nil
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

func (p *RecruitmentCleanupParams) GetUserID() string {
	return p.UserID
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

func (p *UserCheckinParams) GetUserID() string {
	return p.UserID
}
