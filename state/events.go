// Package state contains logic to maintain an event store on Agent to remember the recently
// processed events. This helps in avoiding the processing of duplicate events, just in case an
// agent receives duplicate events (which shouldn't happen ideally).
package state

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/neptuneio/agent/api"
	"github.com/neptuneio/agent/logging"
	"github.com/neptuneio/agent/util"
	"path/filepath"
)

const (
	eventBackupFile      = ".events"
	eventIdTimestampSep  = ":::"
	eventCleanupInterval = time.Second * 30 * 60 // Once every half hour
)

var eventIdToTimestamp = util.NewConcurrentMap()
var eventReloadCh = time.NewTicker(eventCleanupInterval).C
var eventPersistCh = make(chan *api.Event)
var eventsFilePath string

func PersistEvent(event *api.Event) error {
	eventPersistCh <- event
	return nil
}

func InitializeEventsFile(dir string) {
	eventsFilePath = filepath.Join(dir, eventBackupFile)
	logging.Info("Initializing events backup file.", logging.Fields{"filepath": eventsFilePath})

	// Start a GO routine to periodically purge events from store and keep the in-memory map in sync with store.
	go func() {
		// Reload the event ids into global map.
		if err := reloadEventIds(); err != nil {
			// If there was an issue in reloading events, initialize this to empty map.
			eventIdToTimestamp = util.NewConcurrentMap()
		}

		for {
			select {
			case <-eventReloadCh:
				logging.Debug("Reloading all events.", nil)

				// First remove old items from the map.
				currentTime := time.Now()
				eventsToRemove := []string{}
				for entry := range eventIdToTimestamp.Iter() {
					// If the duration of event creation time to now is older than event cleanup interval,
					// go ahead and remove the event.
					if currentTime.Sub(time.Unix(entry.Val, 0)) > eventCleanupInterval {
						eventsToRemove = append(eventsToRemove, entry.Key)
					}
				}

				for _, e := range eventsToRemove {
					eventIdToTimestamp.Remove(e)
				}

				// Now, write the complete map to a new file.
				if err := os.Remove(eventsFilePath); err != nil {
					logging.Warn("Could not remove the file.", logging.Fields{"error": err})
				} else {
					writeToBackupFile()
				}
			case event := <-eventPersistCh:
				logging.Debug("Persisting the event id.", logging.Fields{"eventId": event.EventId})
				currentTime := time.Now().Unix()
				eventIdToTimestamp.Set(event.EventId, currentTime)
				writeOneRecord(event.EventId, currentTime)
			}
		}
	}()
}

func reloadEventIds() error {

	var file *os.File
	if _, err := os.Stat(eventsFilePath); os.IsNotExist(err) {
		logging.Info("Events backup file does not exist so creating it.", logging.Fields{"file": eventsFilePath})
		file, err = os.Create(eventsFilePath)
		if err != nil {
			logging.Warn("Could not create events backup file.", nil)
			return err
		}
		defer file.Close()
		return err
	} else {
		file, err = os.Open(eventsFilePath)
		if err != nil {
			logging.Warn("Could not open the backup file.", logging.Fields{"error": err})
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	concurrentMap := util.NewConcurrentMap()
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), eventIdTimestampSep)
		if len(parts) > 1 {
			if timestamp, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				concurrentMap.Set(parts[0], timestamp)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logging.Warn("Could not read text from the file.", logging.Fields{"file": eventsFilePath})
		return err
	} else {
		eventIdToTimestamp = concurrentMap
		return nil
	}
}

func writeOneRecord(eventId string, timestamp int64) {
	// Now write the values to file.
	f, err := os.OpenFile(eventsFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	defer f.Close()
	if err != nil {
		logging.Error("Could not open event file.", logging.Fields{"error": err})
	} else {
		writeToFile(f, eventId, timestamp)
	}
}

func writeToBackupFile() error {
	if _, err := os.Stat(eventsFilePath); os.IsNotExist(err) {
		logging.Info("Creating events backup file.", logging.Fields{"file": eventsFilePath})
		file, err := os.Create(eventsFilePath)
		if err != nil {
			logging.Info("Could not create file.", nil)
		}
		defer file.Close()
	}

	f, err := os.OpenFile(eventsFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	defer f.Close()
	if err != nil {
		logging.Error("Could not open event file.", logging.Fields{"error": err})
		return err
	}

	logging.Info("Writing event ids to file.", nil)

	for entry := range eventIdToTimestamp.Iter() {
		writeToFile(f, entry.Key, entry.Val)
	}

	return nil
}

// Function to persist the event id to event store.
func writeToFile(f *os.File, eventId string, timestamp int64) error {
	logging.Debug("Writing event id to file.", logging.Fields{"eventId": eventId})

	record := strings.Join([]string{eventId, eventIdTimestampSep, strconv.FormatInt(timestamp, 10), "\n"}, "")
	if _, err := f.WriteString(record); err != nil {
		logging.Error("Could not write to event file.", logging.Fields{"error": err})
		return err
	}

	return nil
}

// Function to check if the given event id was already processed by this agent or not.
func HasProcessedEvent(eventId string) bool {
	return (eventIdToTimestamp != nil && eventIdToTimestamp.Has(eventId))
}
