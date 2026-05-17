package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMissingMPVExits verifies missing mpv causes exit code 1 and stderr message
func TestMissingMPVExits(t *testing.T) {
	// Create a temporary directory to use as PATH
	tmpDir := t.TempDir()
	// Create a fake yt-dlp but not mpv
	ytdlpPath := filepath.Join(tmpDir, "yt-dlp")
	os.WriteFile(ytdlpPath, []byte("#!/bin/sh\necho ok"), 0755)

	cmd := exec.Command("./ha_test_bin")
	cmd.Env = append(os.Environ(), "PATH="+tmpDir)
	// Don't set HOMEASSISTANT_LLM_URL, but this should fail on missing mpv first
	cmd.Env = append(cmd.Env, "HOMEASSISTANT_LLM_URL=http://localhost:9999")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected non-zero exit, but got success")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1, got %v", err)
	}

	stderrStr := stderr.String()
	if !contains(stderrStr, "mpv") {
		t.Errorf("Expected 'mpv' in stderr, got: %s", stderrStr)
	}
}

// TestMissingYtdlpExits verifies missing yt-dlp causes exit code 1 and stderr message
func TestMissingYtdlpExits(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a fake mpv but not yt-dlp
	mpvPath := filepath.Join(tmpDir, "mpv")
	os.WriteFile(mpvPath, []byte("#!/bin/sh\necho ok"), 0755)

	cmd := exec.Command("./ha_test_bin")
	cmd.Env = append(os.Environ(), "PATH="+tmpDir)
	cmd.Env = append(cmd.Env, "HOMEASSISTANT_LLM_URL=http://localhost:9999")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected non-zero exit, but got success")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1, got %v", err)
	}

	stderrStr := stderr.String()
	if !contains(stderrStr, "yt-dlp") {
		t.Errorf("Expected 'yt-dlp' in stderr, got: %s", stderrStr)
	}
}

// TestMissingBothExits verifies both missing binaries are reported
func TestMissingBothExits(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create either binary

	cmd := exec.Command("./ha_test_bin")
	cmd.Env = append(os.Environ(), "PATH="+tmpDir)
	cmd.Env = append(cmd.Env, "HOMEASSISTANT_LLM_URL=http://localhost:9999")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected non-zero exit, but got success")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1, got %v", err)
	}

	stderrStr := stderr.String()
	if !contains(stderrStr, "mpv") || !contains(stderrStr, "yt-dlp") {
		t.Errorf("Expected both 'mpv' and 'yt-dlp' in stderr, got: %s", stderrStr)
	}
}

// TestMissingLLMUrlExits verifies missing HOMEASSISTANT_LLM_URL causes exit code 1
func TestMissingLLMUrlExits(t *testing.T) {
	cmd := exec.Command("./ha_test_bin")
	// Clear HOMEASSISTANT_LLM_URL
	env := []string{}
	for _, e := range os.Environ() {
		if !starts(e, "HOMEASSISTANT_LLM_URL") {
			env = append(env, e)
		}
	}
	cmd.Env = env

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected non-zero exit, but got success")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1, got %v", err)
	}

	stderrStr := stderr.String()
	if !contains(stderrStr, "HOMEASSISTANT_LLM_URL") {
		t.Errorf("Expected 'HOMEASSISTANT_LLM_URL' in stderr, got: %s", stderrStr)
	}
}

// Helper functions
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func starts(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
