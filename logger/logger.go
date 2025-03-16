package logger

import (
	"astralHRBot/settings"
	"fmt"
	"time"
)

type logLevel string

const (
	info   logLevel = "INFO"
	warn   logLevel = "WARN"
	error  logLevel = "ERROR"
	debug  logLevel = "DEBUG"
	system logLevel = "SYSTEM"
)

type logMessage struct {
	Level   logLevel
	Message string
	Time    time.Time
	TraceID string
}

var logChannel = make(chan logMessage, 200)

func StartLogger() {
	go logWorker()
}

func logWorker() {
	for logMsg := range logChannel {
		output := fmt.Sprintf("[%s] [%s] [TraceID: %s]: %s\n",
			logMsg.Time.Format(time.RFC3339),
			logMsg.Level,
			logMsg.TraceID,
			logMsg.Message,
		)
		fmt.Println(output)
	}
}

func newLog(level logLevel, traceID string, m string) {
	if level == debug && !settings.DebugMode {
		return
	}

	logChannel <- logMessage{
		Level:   level,
		Message: m,
		Time:    time.Now(),
		TraceID: traceID,
	}
}

func Info(traceID string, m string) {
	newLog(info, traceID, m)
}
func Warn(traceID string, m string) {
	newLog(warn, traceID, m)
}
func Error(traceID string, m string) {
	newLog(error, traceID, m)
}
func Debug(traceID string, m string) {
	newLog(debug, traceID, m)
}
func System(m string) {
	newLog(system, "-", m)
}
