package main

import (
	"bufio"
	"flag"
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
	"time"
)

func main() {
	pipePath := flag.String("input-pipe", "", "path to a named pipe (FIFO) to read commands from")
	flag.Parse()

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

	lineCh := make(chan string, 16)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			if *pipePath != "" {
				os.Remove(*pipePath)
			}
			close(appCtx.ShutdownChan)
		case <-appCtx.ShutdownChan:
			if *pipePath != "" {
				os.Remove(*pipePath)
			}
		}
	}()

	go func() {
		<-appCtx.ShutdownChan
		appCtx.Media.KillActive()
		_ = appCtx.Discussion.Persist()
		os.Exit(0)
	}()

	if *pipePath != "" {
		// Create the FIFO if it doesn't already exist.
		err := syscall.Mkfifo(*pipePath, 0600)
		if err != nil && err != syscall.EEXIST {
			fmt.Fprintf(os.Stderr, "FATAL: could not create FIFO %s: %v\n", *pipePath, err)
			os.Exit(1)
		}

		go func() {
			for {
				f, err := os.OpenFile(*pipePath, os.O_RDONLY, os.ModeNamedPipe)
				if err != nil {
					fmt.Fprintf(os.Stderr, "pipe: open error: %v\n", err)
					time.Sleep(1 * time.Second)
					continue
				}
				reader := bufio.NewReader(f)
				for {
					line, err := reader.ReadString('\n')
					if err == io.EOF {
						f.Close()
						break
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "pipe: read error: %v\n", err)
						f.Close()
						break
					}
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						lineCh <- trimmed
					}
				}
			}
		}()
	} else {
		go func() {
			reader := bufio.NewReader(os.Stdin)
			for {
				line, err := reader.ReadString('\n')
				if err == io.EOF {
					close(appCtx.ShutdownChan)
					return
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
					continue
				}
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					lineCh <- trimmed
				}
			}
		}()
	}

	for {
		select {
		case <-appCtx.ShutdownChan:
			return
		case line := <-lineCh:
			router.Route(appCtx, line)
		}
	}
}
