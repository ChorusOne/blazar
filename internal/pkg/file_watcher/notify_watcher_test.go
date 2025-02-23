package file_watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blazar/internal/pkg/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyFileWatcher_FileDoesNotExistInitially(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")

	exists, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")
	defer watcher.Cancel()

	assert.False(t, exists, "Expected file to not exist, but got exists=true")
}

func TestNotifyFileWatcher_DetectsFileCreation(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")

	_, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")
	defer watcher.Cancel()

	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(filePath, []byte("hello"), 0644)
	require.NoError(t, err, "Failed to create test file")

	select {
	case event := <-watcher.ChangeEvents():
		assert.Equal(t, FileCreated, event.Event, "Expected FileCreated event")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for file creation event")
	}
}

func TestNotifyFileWatcher_DetectsFileModification(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")

	err := os.WriteFile(filePath, []byte("initial"), 0644)
	require.NoError(t, err, "Failed to create initial test file")

	_, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")
	defer watcher.Cancel()

	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(filePath, []byte("updated"), 0644)
	require.NoError(t, err, "Failed to modify test file")

	select {
	case event := <-watcher.ChangeEvents():
		assert.Equal(t, FileModified, event.Event, "Expected FileModified event")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for file modification event")
	}
}

func TestNotifyFileWatcher_DetectsFileDeletion(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")

	err := os.WriteFile(filePath, []byte("delete me"), 0644)
	require.NoError(t, err, "Failed to create test file")

	_, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")
	defer watcher.Cancel()

	time.Sleep(100 * time.Millisecond)
	err = os.Remove(filePath)
	require.NoError(t, err, "Failed to delete test file")

	select {
	case event := <-watcher.ChangeEvents():
		assert.Equal(t, FileRemoved, event.Event, "Expected FileRemoved event")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for file removal event")
	}
}

func TestNotifyFileWatcher_DetectsPreExistingFile(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pre_existing.txt")

	err := os.WriteFile(filePath, []byte("already here"), 0644)
	require.NoError(t, err, "Failed to create pre-existing file")

	exists, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create watcher for pre-existing file")
	defer watcher.Cancel()

	assert.True(t, exists, "Expected file to exist at initialization")

	select {
	case event := <-watcher.ChangeEvents():
		t.Fatal("Unexpected event received: ", event)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestNotifyFileWatcher_DetectsFileCreatedAfterInitialization(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	exists, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create watcher for non-existing file")
	defer watcher.Cancel()

	assert.False(t, exists, "Expected file to not exist at initialization")

	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(filePath, []byte("created later"), 0644)
	require.NoError(t, err, "Failed to create new file")

	select {
	case event := <-watcher.ChangeEvents():
		assert.Equal(t, FileCreated, event.Event, "Expected FileCreated event for new file")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for creation event on non-existing file")
	}
}

func TestNotifyFileWatcher_HandlesRapidFileChanges(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rapid_changes.txt")

	err := os.WriteFile(filePath, []byte("start"), 0644)
	require.NoError(t, err, "Failed to create initial test file")

	_, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")
	defer watcher.Cancel()

	// Perform multiple rapid modifications
	for range 5 {
		time.Sleep(50 * time.Millisecond)
		err := os.WriteFile(filePath, []byte("update"), 0644)
		require.NoError(t, err, "Failed to modify test file")
	}

	// Collect events
	var modificationCount int
	timeout := time.After(1 * time.Second)
	for {
		select {
		case event := <-watcher.ChangeEvents():
			if event.Event == FileModified {
				modificationCount++
			}
		case <-timeout:
			assert.Equal(t, 3, modificationCount, "Expected at least 3 modification events")
			return
		}
	}
}

func TestNotifyFileWatcher_StopsProperlyOnCancel(t *testing.T) {
	logger := log.FromContext(context.Background())
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cancel_test.txt")

	err := os.WriteFile(filePath, []byte("before cancel"), 0644)
	require.NoError(t, err, "Failed to create test file")

	_, watcher, err := NewNotifyFileWatcher(logger, filePath)
	require.NoError(t, err, "Failed to create file watcher")

	// Ensure initial modification is detected
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(filePath, []byte("modified before cancel"), 0644)
	require.NoError(t, err, "Failed to modify test file")

	select {
	case event := <-watcher.ChangeEvents():
		assert.Equal(t, FileModified, event.Event, "Expected FileModified event before cancel")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for file modification event before cancel")
	}

	// Cancel the watcher
	watcher.Cancel()

	// Try modifying after cancel
	time.Sleep(100 * time.Millisecond)
	err = os.Remove(filePath)
	require.NoError(t, err, "Failed to remove file after cancel")

	// Ensure no more events are received
	for range 3 {
		select {
		case event := <-watcher.ChangeEvents():
			assert.NotEqual(t, FileRemoved, event.Event, "Expected no file removal events")
		case <-time.After(300 * time.Millisecond):
			// Expected timeout since watcher should be stopped
		}
	}
}
