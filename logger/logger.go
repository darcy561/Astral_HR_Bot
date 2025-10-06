package logger

import (
	"astralHRBot/globals"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type logLevel string

const (
	info        logLevel = "INFO"
	warn        logLevel = "WARN"
	error       logLevel = "ERROR"
	debug       logLevel = "DEBUG"
	system      logLevel = "SYSTEM"
	systemDebug logLevel = "SYSTEM_DEBUG"
)

type LogData map[string]any

type logMessage struct {
	Level   logLevel
	Data    LogData
	Time    time.Time
	TraceID string
	Package string
}

var logChannel = make(chan logMessage, 200)

func StartLogger() {
	go logWorker()
}

func logWorker() {
	for logMsg := range logChannel {
		jsonData, err := json.Marshal(logMsg.Data)
		if err != nil {
			jsonData = []byte(fmt.Sprintf(`{"error": "Failed to marshal log data: %v"}`, err))
		}

		output := fmt.Sprintf("[%s] [%s] [Package: %s]: %s\n",
			logMsg.Time.Format(time.RFC3339),
			logMsg.Level,
			logMsg.Package,
			string(jsonData),
		)
		fmt.Println(output)
	}
}

func getCallerPackage() string {
	pc, _, _, ok := runtime.Caller(3)
	if !ok {
		return "unknown"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	fullName := fn.Name()

	parts := strings.Split(fullName, ".")
	if len(parts) < 2 {
		return "unknown"
	}

	pkgPath := strings.Join(parts[:len(parts)-1], ".")

	pkgName := filepath.Base(pkgPath)

	return pkgName
}

func newLog(level logLevel, data LogData) {
	if (level == debug || level == systemDebug) && !globals.DebugMode {
		return
	}

	traceID := "-"
	if id, ok := data["trace_id"].(string); ok {
		traceID = id
	}

	logChannel <- logMessage{
		Level:   level,
		Data:    data,
		Time:    time.Now(),
		TraceID: traceID,
		Package: getCallerPackage(),
	}
}

func Info(data LogData) {
	newLog(info, data)
}

func Warn(data LogData) {
	newLog(warn, data)
}

func Error(data LogData) {
	newLog(error, data)
}

func Debug(data LogData) {
	newLog(debug, data)
}

func System(data LogData) {
	newLog(system, data)
}

func SystemDebug(data LogData) {
	newLog(systemDebug, data)
}
