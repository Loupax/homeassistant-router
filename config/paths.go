package config

import (
	"os"
	"path/filepath"
)

func StateFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "homeassistant", "state.json"), nil
}
