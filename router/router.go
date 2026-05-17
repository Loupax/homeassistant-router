package router

import (
	"fmt"
	"homeassistant/config"
	"homeassistant/discussion"
	"homeassistant/media"
	"os"
	"strings"
)

type AppContext struct {
	Media        *media.MediaManager
	Discussion   *discussion.DiscussionContext
	ShutdownChan chan struct{}
	Routes       []config.RouteEntry
	LLMEndpoint  string
	APIKey       string
	Model        string
}

func dispatch(appCtx *AppContext, intent, payload string) {
	switch intent {
	case "play":
		appCtx.Media.PlayQuery(payload)
	case "stop":
		appCtx.Media.Stop()
	case "volume_up":
		appCtx.Media.Volume(10)
	case "volume_down":
		appCtx.Media.Volume(-10)
	case "pause":
		appCtx.Media.Pause()
	case "resume":
		appCtx.Media.Resume()
	default:
		fmt.Fprintf(os.Stderr, "router: unknown intent %q\n", intent)
	}
}

func Route(appCtx *AppContext, line string) {
	lower := strings.TrimSpace(strings.ToLower(line))

	// exit/quit — hardcoded, not in routes file, not in classifier
	if lower == "exit" || lower == "quit" {
		close(appCtx.ShutdownChan)
		return
	}

	// Fast path — exact match from routes file (case-insensitive)
	for _, entry := range appCtx.Routes {
		if lower == strings.ToLower(entry.Pattern) {
			dispatch(appCtx, entry.Intent, "")
			return
		}
	}

	// Fast path — hardcoded prefix for play/search (need payload extraction)
	if strings.HasPrefix(lower, "play ") {
		payload := strings.TrimSpace(line[5:])
		if payload != "" {
			appCtx.Media.PlayQuery(payload)
			return
		}
	}
	if strings.HasPrefix(lower, "search ") {
		payload := strings.TrimSpace(line[7:])
		if payload != "" {
			appCtx.Media.PlayQuery(payload)
			return
		}
	}

	// Slow path — LLM classifier
	classification, err := Classify(line, appCtx.LLMEndpoint, appCtx.APIKey, appCtx.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "router: classifier error: %v\n", err)
		fmt.Println("I don't know how to do that yet.")
		return
	}

	switch classification.Intent {
	case "discuss":
		appCtx.Discussion.Submit(line)
	case "unknown":
		fmt.Println("I don't know how to do that yet.")
	default:
		dispatch(appCtx, classification.Intent, classification.Payload)
	}
}
