package router

import (
	"fmt"
	"homeassistant/discussion"
	"homeassistant/media"
	"strings"
)

type AppContext struct {
	Media        *media.MediaManager
	Discussion   *discussion.DiscussionContext
	ShutdownChan chan struct{}
}

func Route(appCtx *AppContext, line string) {
	lower := strings.TrimSpace(strings.ToLower(line))

	switch lower {
	case "exit", "quit":
		close(appCtx.ShutdownChan)
		return
	case "stop", "stop playing", "stop music":
		appCtx.Media.Stop()
		return
	case "louder", "volume up":
		appCtx.Media.Volume(10)
		return
	case "quieter", "quiet", "volume down", "lower":
		appCtx.Media.Volume(-10)
		return
	case "pause":
		appCtx.Media.Pause()
		return
	case "resume", "continue", "unpause":
		appCtx.Media.Resume()
		return
	}

	if strings.HasPrefix(lower, "play ") || strings.HasPrefix(lower, "search ") {
		var payload string
		if strings.HasPrefix(lower, "play ") {
			payload = strings.TrimSpace(line[len("play "):])
		} else {
			payload = strings.TrimSpace(line[len("search "):])
		}
		if payload == "" {
			fmt.Println("Warning: empty payload for media command")
			return
		}
		appCtx.Media.PlayQuery(payload)
		return
	}

	appCtx.Discussion.Submit(line)
}
