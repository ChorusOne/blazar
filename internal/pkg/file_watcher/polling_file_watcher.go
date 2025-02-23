package file_watcher

import (
	"os"
	"time"

	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"
)

type PollingFileWatcher struct {
	// full path to a watched file
	lastModTime  time.Time
	exists       bool
	changeEvents <-chan NewFileChangeEvent
	cancel       chan<- struct{}
}

// Returns if the file exists, file watcher, error
func NewPollingFileWatcher(logger *log.MultiLogger, filepath string, interval time.Duration) (bool, *PollingFileWatcher, error) {
	// In case file doesn't exist, modTime will be "zero"
	// so we can still use it to check for "file change"
	// as modTime of created file will be be greater than this
	initExists, initModTime, err := getFileStatus(filepath)
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to check %s status", filepath)
	}

	events := make(chan NewFileChangeEvent)
	cancel := make(chan struct{})

	fw := &PollingFileWatcher{
		lastModTime:  initModTime,
		exists:       initExists,
		changeEvents: events,
		cancel:       cancel,
	}

	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				var newEvent NewFileChangeEvent
				exists, modTime, err := getFileStatus(filepath)
				if err != nil {
					newEvent.Error = err
				} else {
					if exists != fw.exists {
						if exists {
							fw.lastModTime = modTime
							newEvent.Event = FileCreated
						} else {
							newEvent.Event = FileRemoved
						}
						fw.exists = exists
					} else if modTime.After(fw.lastModTime) {
						fw.lastModTime = modTime
						newEvent.Event = FileModified
					}
				}
				select {
				case events <- newEvent:
				// to prevent deadlock with events channel
				case <-cancel:
					logger.Debug("File watcher exiting")
					return
				}
			// this isn't necessary since we exit in the above select statement
			// but this will help in early exit in case cancel is called before the ticker fires
			case <-cancel:
				logger.Debug("File watcher exiting")
				return
			}
		}
	}()
	return initExists, fw, nil
}

func (fw *PollingFileWatcher) ChangeEvents() <-chan NewFileChangeEvent {
	return fw.changeEvents
}

func (fw *PollingFileWatcher) Cancel() {
	fw.cancel <- struct{}{}
}

// Checks if the file exists and returns the timestamp of the last modification
// returns exists, modTime, error
func getFileStatus(file string) (bool, time.Time, error) {
	stat, err := os.Stat(file)

	switch {
	case os.IsNotExist(err):
		return false, time.Time{}, nil
	case err != nil:
		return false, time.Time{}, err
	}

	return true, stat.ModTime(), nil
}
