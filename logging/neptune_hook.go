package logging

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
)

// NeptuneHook to send logs to Neptune service.
type NeptuneHook struct {
	errorsCh chan string
	levels   []logrus.Level
}

// NewNeptuneHook creates a hook to be added to an instance of logger.
func NewNeptuneHook(level logrus.Level, ch chan string) *NeptuneHook {

	levels := []logrus.Level{}
	for _, l := range []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	} {
		if l <= level {
			levels = append(levels, l)
		}
	}

	return &NeptuneHook{
		errorsCh: ch,
		levels:   levels,
	}
}

// Fire sends the event to Neptune
func (hook *NeptuneHook) Fire(entry *logrus.Entry) error {
	s := []string{entry.Message}
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/Sirupsen/logrus/issues/137
			s = append(s, fmt.Sprintf("%v=%v", k, v.Error()))
		default:
			s = append(s, fmt.Sprintf("%v=%v", k, v))
		}
	}

	hook.errorsCh <- strings.Join(s, " ")
	return nil
}

// Levels returns the list of logging levels that we want to send to Neptune.
func (hook *NeptuneHook) Levels() []logrus.Level {
	return hook.levels
}
