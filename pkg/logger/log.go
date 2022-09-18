package logger

import (
	"log"
	"os"
	"strings"
)

type Logger struct {
	*log.Logger
	logFile *os.File
}

var infoLogger Logger
var errorLogger Logger

func createLoggers(level string) Logger {
	fileName := level + ".log"
	logFile, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Fatal(err)
	}

	var flags int

	switch level {
	case "error":
		flags = log.Ldate | log.Ltime | log.Lshortfile
	default:
		flags = log.Ldate | log.Ltime
	}

	prefix := strings.ToLower(level) + "\t"

	return Logger{log.New(logFile, prefix, flags), logFile}
}

func Error(err error) {
	errorLogger.Fatal(err)
}

func Info(subject string) {
	infoLogger.Printf(subject)
}

func CreateLogger() {
	infoLogger = createLoggers("info")
	errorLogger = createLoggers("error")

}

func Close() {
	infoLogger.logFile.Close()
	errorLogger.logFile.Close()
}
