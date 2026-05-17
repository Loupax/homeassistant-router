package media

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

const socketPath = "/tmp/ha-mpv.sock"

type MediaManager struct {
	mu            sync.Mutex
	activeProcess *os.Process
	activeCmd     *exec.Cmd
}

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9 ',\-]`)

func (m *MediaManager) PlayQuery(rawQuery string) {
	query := strings.TrimSpace(sanitizeRe.ReplaceAllString(rawQuery, ""))
	if query == "" {
		fmt.Println("Warning: empty query after sanitization")
		return
	}

	m.mu.Lock()
	if m.activeProcess != nil {
		fmt.Fprintf(os.Stderr, "[media] killing previous mpv (pid %d)\n", m.activeProcess.Pid)
		_ = m.activeProcess.Kill()
		m.activeProcess = nil
		m.activeCmd = nil
	}

	fmt.Printf("[media] searching: %s\n", query)
	cmd := exec.Command("mpv",
		"--no-video",
		"--no-terminal",
		"--input-ipc-server="+socketPath,
		"ytdl://ytsearch:"+query,
	)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		fmt.Fprintf(os.Stderr, "[media] failed to start mpv: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "[media] mpv started (pid %d)\n", cmd.Process.Pid)
	m.activeProcess = cmd.Process
	m.activeCmd = cmd
	m.mu.Unlock()

	go func() {
		err := cmd.Wait()
		fmt.Fprintf(os.Stderr, "[media] mpv (pid %d) exited: %v\n", cmd.Process.Pid, err)
		m.mu.Lock()
		if m.activeProcess == cmd.Process {
			m.activeProcess = nil
			m.activeCmd = nil
		}
		m.mu.Unlock()
	}()
}

func (m *MediaManager) Stop() {
	m.mu.Lock()
	active := m.activeProcess != nil
	m.mu.Unlock()
	if !active {
		fmt.Println("[media] nothing playing")
		return
	}
	_ = m.sendIPC([]any{"quit"})
}

func (m *MediaManager) Volume(delta int) {
	m.mu.Lock()
	active := m.activeProcess != nil
	m.mu.Unlock()
	if !active {
		fmt.Println("[media] nothing playing")
		return
	}
	if err := m.sendIPC([]any{"add", "volume", delta}); err != nil {
		fmt.Fprintf(os.Stderr, "[media] volume error: %v\n", err)
	}
}

func (m *MediaManager) Pause() {
	m.mu.Lock()
	active := m.activeProcess != nil
	m.mu.Unlock()
	if !active {
		fmt.Println("[media] nothing playing")
		return
	}
	_ = m.sendIPC([]any{"set_property", "pause", true})
}

func (m *MediaManager) Resume() {
	m.mu.Lock()
	active := m.activeProcess != nil
	m.mu.Unlock()
	if !active {
		fmt.Println("[media] nothing playing")
		return
	}
	_ = m.sendIPC([]any{"set_property", "pause", false})
}

func (m *MediaManager) KillActive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeProcess != nil {
		_ = m.activeProcess.Kill()
		m.activeProcess = nil
		m.activeCmd = nil
	}
}

func (m *MediaManager) sendIPC(command []any) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("ipc connect: %w", err)
	}
	defer conn.Close()

	msg, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return err
	}
	msg = append(msg, '\n')
	_, err = conn.Write(msg)
	return err
}
