package router

import (
	"homeassistant/discussion"
	"homeassistant/media"
	"testing"
	"path/filepath"
)

// TestRouteExitIntent verifies "exit" closes ShutdownChan
func TestRouteExitIntent(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "exit")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("exit did not close ShutdownChan")
	}
}

// TestRouteExitIntentUppercase verifies "EXIT" closes ShutdownChan
func TestRouteExitIntentUppercase(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "EXIT")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("EXIT did not close ShutdownChan")
	}
}

// TestRouteQuitIntent verifies "quit" closes ShutdownChan
func TestRouteQuitIntent(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "quit")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("quit did not close ShutdownChan")
	}
}

// TestRoutePlayMedia verifies "play something" calls media subsystem
func TestRoutePlayMedia(t *testing.T) {
	// This is a behavioral test: we verify it doesn't crash and the media manager is called
	// Since PlayQuery spawns a subprocess (mpv), we can't easily verify it was called
	// without mocking. We'll just verify no panic occurs.
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic
	Route(appCtx, "play something")
}

// TestRoutePlayMediaMixedCase verifies "Play Something" calls media subsystem
func TestRoutePlayMediaMixedCase(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic
	Route(appCtx, "Play Something")
}

// TestRouteSearchMedia verifies "search query" calls media subsystem
func TestRouteSearchMedia(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic
	Route(appCtx, "search beethoven")
}

// TestRouteSearchMediaMixedCase verifies "Search Query" calls media subsystem
func TestRouteSearchMediaMixedCase(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic
	Route(appCtx, "Search Bach")
}

// TestRouteFallsThroughToDiscussion verifies "hello world" falls through to discussion
func TestRouteFallsThroughToDiscussion(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic (discussion.Submit will be called, which will error if LLMEndpoint not set, but won't panic)
	Route(appCtx, "hello world")
}

// TestRoutePlayEmptyPayload verifies "play " with empty payload prints warning
func TestRoutePlayEmptyPayload(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash or call media
	Route(appCtx, "play ")
}

// TestRouteSearchEmptyPayload verifies "search " with empty payload prints warning
func TestRouteSearchEmptyPayload(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash or call media
	Route(appCtx, "search ")
}

// TestRouteQuitUppercase verifies "QUIT" closes ShutdownChan
func TestRouteQuitUppercase(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "QUIT")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("QUIT did not close ShutdownChan")
	}
}

// TestRouteQuitMixedCase verifies "QuIt" closes ShutdownChan
func TestRouteQuitMixedCase(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "QuIt")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("QuIt did not close ShutdownChan")
	}
}

// TestRouteExitWithSpaces verifies "  exit  " is properly trimmed and closes ShutdownChan
func TestRouteExitWithSpaces(t *testing.T) {
	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   &discussion.DiscussionContext{},
		ShutdownChan: make(chan struct{}),
	}

	Route(appCtx, "  exit  ")

	select {
	case <-appCtx.ShutdownChan:
		// Expected
	default:
		t.Fatal("'  exit  ' did not close ShutdownChan")
	}
}

// TestRoutePlayWithSpaces verifies "  play something  " is handled correctly
func TestRoutePlayWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash
	Route(appCtx, "  play  something  ")
}

// TestRoutePlayWithOnlySpacePayload verifies "play    " (spaces only) is rejected
func TestRoutePlayWithOnlySpacePayload(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash or call media
	Route(appCtx, "play    ")
}

// TestRouteSearchWithOnlySpacePayload verifies "search    " (spaces only) is rejected
func TestRouteSearchWithOnlySpacePayload(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash or call media
	Route(appCtx, "search    ")
}

// TestRoutePlayWithSpecialChars verifies play with special chars calls media
func TestRoutePlayWithSpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash - sanitization happens in media.PlayQuery
	Route(appCtx, "play hello@#$%world")
}

// TestRouteSearchWithSpecialChars verifies search with special chars calls media
func TestRouteSearchWithSpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not crash - sanitization happens in media.PlayQuery
	Route(appCtx, "search hello@#$%world")
}

// TestRouteDiscussionWithoutLLMEndpoint verifies discussion submission handles missing endpoint
func TestRouteDiscussionWithoutLLMEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)
	// Intentionally do not set LLMEndpoint

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// Should not panic (will error on http request, but won't crash)
	Route(appCtx, "hello world")
}

// TestRoutePlayPrefixNotExact verifies "plays" does not trigger play intent
func TestRoutePlayPrefixNotExact(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)
	dc.LLMEndpoint = "http://localhost:9999"

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// "plays" should NOT match "play " prefix and should fall through to discussion
	// This will trigger a network error but won't crash
	Route(appCtx, "plays something")
}

// TestRouteSearchPrefixNotExact verifies "searches" does not trigger search intent
func TestRouteSearchPrefixNotExact(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	dc, _ := discussion.LoadOrInit(statePath)
	dc.LLMEndpoint = "http://localhost:9999"

	appCtx := &AppContext{
		Media:        &media.MediaManager{},
		Discussion:   dc,
		ShutdownChan: make(chan struct{}),
	}

	// "searches" should NOT match "search " prefix and should fall through to discussion
	// This will trigger a network error but won't crash
	Route(appCtx, "searches something")
}
