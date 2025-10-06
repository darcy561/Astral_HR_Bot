package models

import (
	"time"
)

type MonitoringAction string

const (
	// Message tracking
	ActionMessageCreate MonitoringAction = "message_create"
	ActionMessageEdit   MonitoringAction = "message_edit"
	ActionMessageDelete MonitoringAction = "message_delete"

	// Voice activity
	ActionVoiceJoin  MonitoringAction = "voice_join"
	ActionVoiceLeave MonitoringAction = "voice_leave"

	// Invites
	ActionInviteCreate MonitoringAction = "invite_create"

	// Reactions
	ActionReactionAdd    MonitoringAction = "reaction_add"
	ActionReactionRemove MonitoringAction = "reaction_remove"
)

type MonitoringScenario string

const (
	// NewRecruit tracks a user who has just joined the corporation
	MonitoringScenarioNewRecruit MonitoringScenario = "new_recruit"

	// RecruitmentProcess tracks a user going through the recruitment process
	MonitoringScenarioRecruitmentProcess MonitoringScenario = "recruitment_process"
)

// ScenarioConfig defines which actions are monitored for each scenario
var ScenarioConfig = map[MonitoringScenario][]MonitoringAction{
	MonitoringScenarioNewRecruit: {
		ActionMessageCreate,
		ActionVoiceJoin,
		ActionInviteCreate,
	},
	MonitoringScenarioRecruitmentProcess: {
		ActionMessageCreate,
		ActionInviteCreate,
	},
}

// ScenarioTaskConfig defines which task functions are associated with each scenario
var ScenarioTaskConfig = map[MonitoringScenario][]string{
	MonitoringScenarioNewRecruit: {
		"ProcessUserCheckin",
	},
	MonitoringScenarioRecruitmentProcess: {
		"ProcessRecruitmentCleanup",
	},
}

// GetTaskFunctionsForScenario returns the task function names for a given scenario
func GetTaskFunctionsForScenario(scenario MonitoringScenario) []string {
	if functions, exists := ScenarioTaskConfig[scenario]; exists {
		return functions
	}
	return []string{}
}

// TaskHandlerFunc represents a function that can handle a specific task type
type TaskHandlerFunc func(Task)

// TaskHandlers maps task types to their handler functions
var TaskHandlers = map[TaskType]TaskHandlerFunc{}

type UserAnalytics struct {
	UserID       string
	Messages     int64
	VoiceJoins   int64
	Invites      int64
	TopChannelID string
}

type UserMonitoring struct {
	UserID    string
	Scenarios map[MonitoringScenario]struct{}
	StartedAt int64 // Unix timestamp when monitoring started
	ExpiresAt int64 // Unix timestamp when monitoring should end (0 for indefinite)
}

func NewUserMonitoring(userID string) *UserMonitoring {
	return &UserMonitoring{
		UserID:    userID,
		Scenarios: make(map[MonitoringScenario]struct{}),
		StartedAt: time.Now().Unix(),
	}
}

func (um *UserMonitoring) AddScenario(scenario MonitoringScenario) {
	um.Scenarios[scenario] = struct{}{}
}

func (um *UserMonitoring) RemoveScenario(scenario MonitoringScenario) {
	delete(um.Scenarios, scenario)
}

func (um *UserMonitoring) HasScenario(scenario MonitoringScenario) bool {
	_, exists := um.Scenarios[scenario]
	return exists
}

func (um *UserMonitoring) GetScenarios() []MonitoringScenario {
	scenarios := make([]MonitoringScenario, 0, len(um.Scenarios))
	for s := range um.Scenarios {
		scenarios = append(scenarios, s)
	}
	return scenarios
}

func (um *UserMonitoring) SetExpiration(duration time.Duration) {
	if duration > 0 {
		um.ExpiresAt = time.Now().Add(duration).Unix()
	} else {
		um.ExpiresAt = 0
	}
}

func (um *UserMonitoring) IsExpired() bool {
	if um.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > um.ExpiresAt
}

// ShouldTrackAction checks if an action should be tracked based on the user's active scenarios
func (um *UserMonitoring) ShouldTrackAction(action MonitoringAction) bool {
	for scenario := range um.Scenarios {
		if actions, exists := ScenarioConfig[scenario]; exists {
			for _, a := range actions {
				if a == action {
					return true
				}
			}
		}
	}
	return false
}
