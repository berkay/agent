// Package logging is the logging package used by Neptune.io agent.
// The implementation might route the logging via any other Go logging package but
// should mask those details from remaining agent packages.
package logging

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	maxLogFileSizeInMB = 10
	maxNumLogFiles     = 10
)

// Fields type, used to pass to key value pairs.
type Fields map[string]interface{}

var log *logrus.Logger

func init() {
	// Create a new instance of the logger. You can have any number of instances.
	log = logrus.New()
}

func convertToLogrusFields(fields Fields) logrus.Fields {
	result := logrus.Fields{}
	for k := range fields {
		result[k] = fields[k]
	}
	return result
}

func Debug(msg string, fields Fields) {
	if fields != nil {
		log.WithFields(convertToLogrusFields(fields)).Debug(msg)
	} else {
		log.Debug(msg)
	}
}

func Info(msg string, fields Fields) {
	if fields != nil {
		log.WithFields(convertToLogrusFields(fields)).Info(msg)
	} else {
		log.Info(msg)
	}
}

func Warn(msg string, fields Fields) {
	if fields != nil {
		log.WithFields(convertToLogrusFields(fields)).Warn(msg)
	} else {
		log.Warn(msg)
	}
}

func Error(msg string, fields Fields) {
	if fields != nil {
		log.WithFields(convertToLogrusFields(fields)).Error(msg)
	} else {
		log.Error(msg)
	}
}

// Function to setup logger for agent.
func SetupLogger(logfile string, debugMode bool, errorsChannel chan string) error {
	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return err
	}
	defer f.Close()

	log.Out = &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    maxLogFileSizeInMB, // megabytes
		MaxBackups: maxNumLogFiles,
		LocalTime:  true,
	}

	hook := NewNeptuneHook(logrus.ErrorLevel, errorsChannel)
	log.Hooks.Add(hook)

	// Only log the info severity or above.
	if debugMode {
		log.Level = logrus.DebugLevel
	} else {
		log.Level = logrus.InfoLevel
	}

	log.Formatter = &logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	}
	return nil
}
