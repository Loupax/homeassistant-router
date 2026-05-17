package main

import (
	"bufio"
	"fmt"
	"homeassistant/config"
	"homeassistant/discussion"
	"homeassistant/media"
	"homeassistant/router"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	missingDeps := false
	for _, bin := range []string{"mpv", "yt-dlp"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: required binary not found in $PATH: %s\n", bin)
			missingDeps = true
		}
	}
	if missingDeps {
		os.Exit(1)
	}

	statePath, err := config.StateFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not resolve state path: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not create state directory: %v\n", err)
		os.Exit(1)
	}

	llmURL := os.Getenv("HOMEASSISTANT_LLM_URL")
	if llmURL == "" {
		fmt.Fprintf(os.Stderr, "FATAL: HOMEASSISTANT_LLM_URL environment variable is required\n")
		os.Exit(1)
	}

	apiKey := os.Getenv("HOMEASSISTANT_API_KEY")

	model := os.Getenv("HOMEASSISTANT_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	routesPath, err := config.RoutesFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not resolve routes path: %v\n", err)
		os.Exit(1)
	}
	if err := config.EnsureRoutesFile(routesPath); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not ensure routes file: %v\n", err)
		os.Exit(1)
	}
	routes, err := config.LoadRoutes(routesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not load routes: %v\n", err)
		os.Exit(1)
	}

	dc, err := discussion.LoadOrInit(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: could not load state: %v\n", err)
		os.Exit(1)
	}
	dc.LLMEndpoint = llmURL
	dc.APIKey = apiKey
	dc.Model = model

	appCtx := &router.AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
		Routes:       routes,
		LLMEndpoint:  llmURL,
		APIKey:       apiKey,
		Model:        model,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			close(appCtx.ShutdownChan)
		case <-appCtx.ShutdownChan:
		}
	}()

	go func() {
		<-appCtx.ShutdownChan
		appCtx.Media.KillActive()
		_ = appCtx.Discussion.Persist()
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				appCtx.Media.KillActive()
				_ = appCtx.Discussion.Persist()
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		router.Route(appCtx, line)
	}
}
