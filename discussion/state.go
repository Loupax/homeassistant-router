package discussion

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StateFile struct {
	LastUpdated time.Time `json:"last_updated"`
	History     []Message `json:"history"`
}

type DiscussionContext struct {
	mu          sync.Mutex
	State       *StateFile
	StatePath   string
	LLMEndpoint string
	APIKey      string
	Model       string
}

func LoadOrInit(path string) (*DiscussionContext, error) {
	dc := &DiscussionContext{StatePath: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		sf := &StateFile{
			LastUpdated: time.Now().UTC(),
			History:     []Message{},
		}
		dc.State = sf
		if err := PersistState(path, sf); err != nil {
			return nil, err
		}
		return dc, nil
	}
	if err != nil {
		return nil, err
	}

	var sf StateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}

	storedDay := sf.LastUpdated.UTC().Truncate(24 * time.Hour)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if today.After(storedDay) {
		sf.History = []Message{}
		sf.LastUpdated = time.Now().UTC()
		if err := PersistState(path, &sf); err != nil {
			return nil, err
		}
		fmt.Println("[context evicted: new day detected]")
	}

	dc.State = &sf
	return dc, nil
}

func (d *DiscussionContext) Persist() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return PersistState(d.StatePath, d.State)
}

func PersistState(path string, s *StateFile) error {
	tmp := path + ".tmp"
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
