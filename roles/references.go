package roles

import (
	"log"
	"os"
)

// Content notification role environment variable names
const (
	MiningRole   = "MINING_ROLE_ID"
	IndustryRole = "INDUSTRY_ROLE_ID"
	PveRole      = "PVE_ROLE_ID"
	PvpRole      = "PVP_ROLE_ID"
	FwRole       = "FW_ROLE_ID"

	MemberRole          = "MEMBER_ROLE_ID"
	RecruitRole         = "RECRUIT_ROLE_ID"
	GuestRole           = "GUEST_ROLE_ID"
	AbsenteeRole        = "ABSENTEE_ROLE_ID"
	ServerClown         = "SERVER_CLOWN_ROLE_ID"
	BlueRole            = "BLUE_ROLE_ID"
	NewcomerRole        = "NEWCOMER_ROLE_ID"
	AuthenticatedGuest  = "AUTHENTICATED_GUEST_ROLE_ID"
	AuthenticatedMember = "AUTHENTICATED_MEMBER_ROLE_ID"
)

// ContentNotificationRoles contains all content notification role env var names
var ContentNotificationRoles = []string{
	GetMiningRoleID(),
	GetIndustryRoleID(),
	GetPveRoleID(),
	GetPvpRoleID(),
	GetFwRoleID(),
}

// GetRoleIDFromEnv returns the role ID from environment variables
func GetRoleIDFromEnv(roleEnvVar string) string {
	id, exists := os.LookupEnv(roleEnvVar)
	if !exists {
		log.Printf("Warning: Role ID not found for environment variable: %s", roleEnvVar)
		return ""
	}
	return id
}

// GetContentNotificationRoleIDs returns a map of role names to their IDs
func GetContentNotificationRoleIDs() map[string]string {
	roleIDs := make(map[string]string)
	for _, envVar := range ContentNotificationRoles {
		roleIDs[envVar] = GetRoleIDFromEnv(envVar)
	}
	return roleIDs
}

func GetMemberRoleID() string {
	return GetRoleIDFromEnv(MemberRole)
}

func GetRecruitRoleID() string {
	return GetRoleIDFromEnv(RecruitRole)
}

func GetGuestRoleID() string {
	return GetRoleIDFromEnv(GuestRole)
}

func GetAbsenteeRoleID() string {
	return GetRoleIDFromEnv(AbsenteeRole)
}

func GetServerClownRoleID() string {
	return GetRoleIDFromEnv(ServerClown)
}

func GetBlueRoleID() string {
	return GetRoleIDFromEnv(BlueRole)
}

func GetNewcomerRoleID() string {
	return GetRoleIDFromEnv(NewcomerRole)
}

func GetAuthenticatedGuestRoleID() string {
	return GetRoleIDFromEnv(AuthenticatedGuest)
}

func GetAuthenticatedMemberRoleID() string {
	return GetRoleIDFromEnv(AuthenticatedMember)
}

func GetMiningRoleID() string {
	return GetRoleIDFromEnv(MiningRole)
}

func GetIndustryRoleID() string {
	return GetRoleIDFromEnv(IndustryRole)
}

func GetPveRoleID() string {
	return GetRoleIDFromEnv(PveRole)
}

func GetPvpRoleID() string {
	return GetRoleIDFromEnv(PvpRole)
}

func GetFwRoleID() string {
	return GetRoleIDFromEnv(FwRole)
}
