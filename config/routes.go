package config

import (
	"bufio"
	"encoding/json"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed routes.default.jsonl
var defaultRoutesData []byte

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
	return entries, scanner.Err()
}

func EnsureRoutesFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultRoutesData, 0644)
}

func RoutesFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "homeassistant", "routes.jsonl"), nil
}
