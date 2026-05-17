package discussion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadOrInitMissingFile verifies missing file creates fresh state with empty history
func TestLoadOrInitMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if dc.State == nil {
		t.Fatal("State is nil")
	}
	if len(dc.State.History) != 0 {
		t.Errorf("Expected empty history, got %d messages", len(dc.State.History))
	}

	// Verify file was created
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("State file was not created: %v", err)
	}
}

// TestLoadOrInitValidFile verifies loading a valid state file
func TestLoadOrInitValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create a valid state file
	testState := &StateFile{
		LastUpdated: time.Now().UTC(),
		History: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}
	data, _ := json.Marshal(testState)
	os.WriteFile(statePath, data, 0644)

	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if len(dc.State.History) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(dc.State.History))
	}
	if dc.State.History[0].Content != "hello" {
		t.Errorf("Expected first message 'hello', got %q", dc.State.History[0].Content)
	}
}

// TestLoadOrInitCorruptJSON verifies corrupt JSON returns error
func TestLoadOrInitCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Write invalid JSON
	os.WriteFile(statePath, []byte("{invalid json}"), 0644)

	_, err := LoadOrInit(statePath)
	if err == nil {
		t.Fatal("Expected error on corrupt JSON, got nil")
	}
}

// TestLoadOrInitSameDayPreservesHistory verifies same-day timestamp preserves history
func TestLoadOrInitSameDayPreservesHistory(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	now := time.Now().UTC()
	testState := &StateFile{
		LastUpdated: now,
		History: []Message{
			{Role: "user", Content: "message 1"},
			{Role: "assistant", Content: "message 2"},
		},
	}
	data, _ := json.Marshal(testState)
	os.WriteFile(statePath, data, 0644)

	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if len(dc.State.History) != 2 {
		t.Errorf("Expected 2 messages (history preserved), got %d", len(dc.State.History))
	}
}

// TestLoadOrInitYesterdayEvictsHistory verifies yesterday's timestamp clears history
func TestLoadOrInitYesterdayEvictsHistory(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	testState := &StateFile{
		LastUpdated: yesterday,
		History: []Message{
			{Role: "user", Content: "old message"},
		},
	}
	data, _ := json.Marshal(testState)
	os.WriteFile(statePath, data, 0644)

	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if len(dc.State.History) != 0 {
		t.Errorf("Expected empty history (evicted), got %d messages", len(dc.State.History))
	}

	// Verify file was updated
	reloadedData, _ := os.ReadFile(statePath)
	var reloadedState StateFile
	json.Unmarshal(reloadedData, &reloadedState)
	if len(reloadedState.History) != 0 {
		t.Errorf("Persisted state still has history after eviction")
	}
}

// TestPersistStateWritesValidJSON verifies JSON is written correctly
func TestPersistStateWritesValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := &StateFile{
		LastUpdated: time.Now().UTC(),
		History: []Message{
			{Role: "user", Content: "test"},
		},
	}

	err := PersistState(statePath, testState)
	if err != nil {
		t.Fatalf("PersistState failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Could not read persisted file: %v", err)
	}

	var reloaded StateFile
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Persisted JSON is invalid: %v", err)
	}

	if len(reloaded.History) != 1 || reloaded.History[0].Content != "test" {
		t.Errorf("Reloaded state does not match original")
	}
}

// TestPersistStateAtomicWrite verifies .tmp file is cleaned up after write
func TestPersistStateAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := &StateFile{
		LastUpdated: time.Now().UTC(),
		History:     []Message{},
	}

	err := PersistState(statePath, testState)
	if err != nil {
		t.Fatalf("PersistState failed: %v", err)
	}

	// Check that .tmp file does not exist
	tmpFile := statePath + ".tmp"
	if _, err := os.Stat(tmpFile); err == nil {
		t.Fatalf(".tmp file was not cleaned up")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Unexpected error checking .tmp file: %v", err)
	}

	// Verify main file exists
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("Main state file does not exist: %v", err)
	}
}

// TestPersistStateRereadIdentical verifies re-read produces identical struct
func TestPersistStateRereadIdentical(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	original := &StateFile{
		LastUpdated: time.Now().UTC().Truncate(time.Millisecond), // Truncate for consistency
		History: []Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "msg2"},
		},
	}

	// Write
	if err := PersistState(statePath, original); err != nil {
		t.Fatalf("PersistState failed: %v", err)
	}

	// Read back
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Could not read: %v", err)
	}

	var reread StateFile
	if err := json.Unmarshal(data, &reread); err != nil {
		t.Fatalf("Could not unmarshal: %v", err)
	}

	// Compare
	if len(reread.History) != len(original.History) {
		t.Errorf("History length mismatch: %d vs %d", len(reread.History), len(original.History))
	}
	for i, msg := range reread.History {
		if msg.Role != original.History[i].Role || msg.Content != original.History[i].Content {
			t.Errorf("Message %d mismatch", i)
		}
	}
}

// TestDiscussionContextPersist verifies Persist() method calls PersistState
func TestDiscussionContextPersist(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	dc := &DiscussionContext{
		StatePath: statePath,
		State: &StateFile{
			LastUpdated: time.Now().UTC(),
			History: []Message{
				{Role: "user", Content: "test"},
			},
		},
	}

	err := dc.Persist()
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("State file was not persisted: %v", err)
	}
}

// TestLoadOrInitManyDaysAgoEvictsHistory verifies old timestamps evict history
func TestLoadOrInitManyDaysAgoEvictsHistory(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	weekAgo := time.Now().UTC().Add(-7 * 24 * time.Hour)
	testState := &StateFile{
		LastUpdated: weekAgo,
		History: []Message{
			{Role: "user", Content: "old message"},
			{Role: "assistant", Content: "old response"},
		},
	}
	data, _ := json.Marshal(testState)
	os.WriteFile(statePath, data, 0644)

	dc, err := LoadOrInit(statePath)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if len(dc.State.History) != 0 {
		t.Errorf("Expected empty history after 7 days, got %d", len(dc.State.History))
	}
}
