package file_watcher

import (
	"path/filepath"

	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"

	"github.com/fsnotify/fsnotify"
)

type NotifyFileWatcher struct {
	watcher      *fsnotify.Watcher
	changeEvents chan NewFileChangeEvent
	cancel       chan struct{}
}

func NewNotifyFileWatcher(logger *log.MultiLogger, filePath string) (bool, *NotifyFileWatcher, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to resolve absolute path for %s", filePath)
	}

	exists, err := fileExists(absPath)
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to check initial file status")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to create file watcher")
	}

	events := make(chan NewFileChangeEvent)
	cancel := make(chan struct{})

	fw := &NotifyFileWatcher{
		watcher:      watcher,
		changeEvents: events,
		cancel:       cancel,
	}

	go fw.watchFile(logger, absPath)

	err = watcher.Add(filepath.Dir(absPath))
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to watch directory")
	}

	return exists, fw, nil
}

func (fw *NotifyFileWatcher) watchFile(logger *log.MultiLogger, filePath string) {
	logger.Debugf("File watcher started on: %s", filePath)

	defer close(fw.changeEvents)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if event.Name == filePath {
				logger.Debugf("File detected fsnotify event %v", event)

				var newEvent NewFileChangeEvent

				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					newEvent.Event = FileCreated
				case event.Op&fsnotify.Write == fsnotify.Write:
					newEvent.Event = FileModified
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					newEvent.Event = FileRemoved
				}

				if newEvent.Event != 0 {
					logger.Debugf("File watcher sent event %v", newEvent.Event)
					fw.changeEvents <- newEvent
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.changeEvents <- NewFileChangeEvent{Error: err}

		case <-fw.cancel:
			logger.Debug("File watcher event loop stopping")
			return
		}
	}
}

func (fw *NotifyFileWatcher) ChangeEvents() <-chan NewFileChangeEvent {
	return fw.changeEvents
}

func (fw *NotifyFileWatcher) Cancel() {
	fw.watcher.Close()
	close(fw.cancel)
}
