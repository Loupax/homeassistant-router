package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RouteEntry struct {
	Pattern string `json:"pattern"`
	Intent  string `json:"intent"`
}

func LoadRoutes(path string) ([]RouteEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []RouteEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry RouteEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Fprintf(os.Stderr, "routes: skipping invalid line %d: %v\n", lineNum, err)
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func DefaultRoutes() []RouteEntry {
	return []RouteEntry{
		{Pattern: "stop", Intent: "stop"},
		{Pattern: "stop playing", Intent: "stop"},
		{Pattern: "stop music", Intent: "stop"},
		{Pattern: "louder", Intent: "volume_up"},
		{Pattern: "volume up", Intent: "volume_up"},
		{Pattern: "quieter", Intent: "volume_down"},
		{Pattern: "quiet", Intent: "volume_down"},
		{Pattern: "volume down", Intent: "volume_down"},
		{Pattern: "lower", Intent: "volume_down"},
		{Pattern: "pause", Intent: "pause"},
		{Pattern: "resume", Intent: "resume"},
		{Pattern: "continue", Intent: "resume"},
		{Pattern: "unpause", Intent: "resume"},
	}
}

func EnsureRoutesFile(path string, defaults []RouteEntry) error {
	_, err := os.Stat(path)
	if err == nil {
		// File exists — do nothing.
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, entry := range defaults {
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

func RoutesFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "homeassistant", "routes.jsonl"), nil
}
