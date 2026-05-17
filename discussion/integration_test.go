package discussion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStateEvictionOnStartup verifies that old state files have history evicted on load
func TestStateEvictionOnStartup(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create a state file with yesterday's timestamp and history
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	oldState := &StateFile{
		LastUpdated: yesterday,
		History: []Message{
			{Role: "user", Content: "old message 1"},
			{Role: "assistant", Content: "old response 1"},
			{Role: "user", Content: "old message 2"},
		},
	}

	data, _ := json.Marshal(oldState)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("Could not write test state file: %v", err)
	}

	// Load the state, which should evict history
	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	// Verify history was evicted
	if len(dc.State.History) != 0 {
		t.Errorf("Expected empty history after eviction, got %d messages", len(dc.State.History))
	}

	// Verify the LastUpdated was updated to today
	todayTrunc := time.Now().UTC().Truncate(24 * time.Hour)
	loadedDayTrunc := dc.State.LastUpdated.UTC().Truncate(24 * time.Hour)
	if !loadedDayTrunc.Equal(todayTrunc) {
		t.Errorf("Expected LastUpdated to be updated to today, got %v", dc.State.LastUpdated)
	}

	// Verify the file was updated with the new state
	reloadedData, _ := os.ReadFile(statePath)
	var reloadedState StateFile
	if err := json.Unmarshal(reloadedData, &reloadedState); err != nil {
		t.Fatalf("Could not unmarshal reloaded state: %v", err)
	}

	if len(reloadedState.History) != 0 {
		t.Errorf("Persisted file still contains old history after eviction")
	}
}

// TestAtomicWriteNoTmpFile verifies that .tmp files don't persist after PersistState
func TestAtomicWriteNoTmpFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &StateFile{
		LastUpdated: time.Now().UTC(),
		History: []Message{
			{Role: "user", Content: "test"},
		},
	}

	if err := PersistState(statePath, state); err != nil {
		t.Fatalf("PersistState failed: %v", err)
	}

	// Check main file exists
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("Main state file does not exist: %v", err)
	}

	// Check .tmp file does NOT exist
	tmpFile := statePath + ".tmp"
	if _, err := os.Stat(tmpFile); err == nil {
		t.Fatalf(".tmp file was not cleaned up after PersistState")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Unexpected error checking .tmp file: %v", err)
	}
}

// TestMultipleWritesAreAtomic verifies that concurrent writes don't leave .tmp files
func TestMultipleWritesAreAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &StateFile{
		LastUpdated: time.Now().UTC(),
		History:     []Message{},
	}

	// Write multiple times
	for i := 0; i < 10; i++ {
		state.History = append(state.History, Message{
			Role:    "user",
			Content: "message " + string(rune(i)),
		})
		if err := PersistState(statePath, state); err != nil {
			t.Fatalf("PersistState failed at iteration %d: %v", i, err)
		}

		// After each write, .tmp should not exist
		tmpFile := statePath + ".tmp"
		if _, err := os.Stat(tmpFile); err == nil {
			t.Fatalf(".tmp file exists after PersistState at iteration %d", i)
		}
	}

	// Final file should be valid and contain all messages
	data, _ := os.ReadFile(statePath)
	var final StateFile
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatalf("Final state file is invalid JSON: %v", err)
	}

	if len(final.History) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(final.History))
	}
}
