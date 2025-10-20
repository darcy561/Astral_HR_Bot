package monitoring

import (
	"astralHRBot/channels"
	"astralHRBot/models"
)

// getAllowedChannelIDsForScenarios returns a set of channel IDs allowed across any of the provided scenarios.
// If no scenarios specify channel filters, the returned set will be empty, indicating no restriction.
func getAllowedChannelIDsForScenarios(scenarios []models.MonitoringScenario) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, sc := range scenarios {
		if envVars, ok := models.ScenarioChannelEnvFilter[sc]; ok {
			for _, envVar := range envVars {
				if id := channels.GetChannelID(envVar); id != "" {
					allowed[id] = struct{}{}
				}
			}
		}
	}
	return allowed
}

// isChannelAllowedForScenario returns true if the scenario either has no channel restrictions
// or if the provided channelID is included in its allow-list resolved from environment variables.
func isChannelAllowedForScenario(scenario models.MonitoringScenario, channelID string) bool {
	if envVars, ok := models.ScenarioChannelEnvFilter[scenario]; ok && len(envVars) > 0 {
		for _, envVar := range envVars {
			if id := channels.GetChannelID(envVar); id != "" && id == channelID {
				return true
			}
		}
		return false
	}
	return true
}
