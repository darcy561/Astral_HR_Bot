package globals

import "sync"

var (
	// DebugMode is a boolean that determines if the debug mode is enabled
	DebugMode = true
	// RecruitmentCleanupDelay is the delay in days before the recruitment cleanup task is run
	RecruitmentCleanupDelay = 7
	// RecruitmentWelcomeMessage is the message sent to new recruits when they join the recruitment channel
	RecruitmentWelcomeMessage = "Welcome <@%s>! \n\n" +
		"A member of the recruitment team will be with you shortly. In the meantime, please follow these steps:\n\n" +
		"[Alliance Auth](https://auth.astralinc.space/)\n\n" +
		"* Follow the above link and register your character(s).\n" +
		"* In the **Char Link** tab, authorize each of your characters.\n" +
		"* In the **Member Audit** tab, register each of your characters.\n" +
		"* In the **Services** tab, click the checkbox to link your Discord account.\n\n" +
		"Once you've completed this, a green tick should appear next to your character name on Discord."

	// MemberJoinWelcomeMessage is the message sent to new members when they recieve the member role
	MemberJoinWelcomeMessage = "Welcome to Astral, %s <@%s> o/ \n\n" +
		"Please take a look at <#1229904357697261569> for guides, and specifically the newbro doc for info on our region.\n\n" +
		"If you need a hand moving your stuff around, feel free to head over to <#1082494747937087581> to speak with them directly.\n\n" +
		"Most importantly, head over to <#1161264045584822322> to opt out of the content pings that do not interest you.\n\n" +
		"Clear skies,\n" +
		"And KTF!"
	// NewRecruitTrackingDays is the number of days to track new recruits
	NewRecruitTrackingDays = 7
)

var (
	debugModeMutex               sync.RWMutex
	recruitmentCleanupDelayMutex sync.RWMutex
	newRecruitTrackingDaysMutex  sync.RWMutex
)

// SetDebugMode safely sets the debug mode with proper synchronization
func SetDebugMode(enabled bool) {
	debugModeMutex.Lock()
	DebugMode = enabled
	debugModeMutex.Unlock()
}

// GetDebugMode safely gets the current debug mode
func GetDebugMode() bool {
	debugModeMutex.RLock()
	defer debugModeMutex.RUnlock()
	return DebugMode
}

// SetRecruitmentCleanupDelay safely sets the recruitment cleanup delay with proper synchronization
func SetRecruitmentCleanupDelay(days int) {
	recruitmentCleanupDelayMutex.Lock()
	RecruitmentCleanupDelay = days
	recruitmentCleanupDelayMutex.Unlock()
}

// GetRecruitmentCleanupDelay safely gets the current recruitment cleanup delay
func GetRecruitmentCleanupDelay() int {
	recruitmentCleanupDelayMutex.RLock()
	defer recruitmentCleanupDelayMutex.RUnlock()
	return RecruitmentCleanupDelay
}

// SetNewRecruitTrackingDays safely sets the new recruit tracking days with proper synchronization
func SetNewRecruitTrackingDays(days int) {
	newRecruitTrackingDaysMutex.Lock()
	NewRecruitTrackingDays = days
	newRecruitTrackingDaysMutex.Unlock()
}

// GetNewRecruitTrackingDays safely gets the current new recruit tracking days
func GetNewRecruitTrackingDays() int {
	newRecruitTrackingDaysMutex.RLock()
	defer newRecruitTrackingDaysMutex.RUnlock()
	return NewRecruitTrackingDays
}
