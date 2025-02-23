package file_watcher

import "os"

type FileChangeEvent int

const (
	FileRemoved FileChangeEvent = iota + 1
	FileCreated
	FileModified
)

type NewFileChangeEvent struct {
	Event FileChangeEvent
	Error error
}

type FileWatcher interface {
	ChangeEvents() <-chan NewFileChangeEvent
	Cancel()
}

func fileExists(file string) (bool, error) {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
