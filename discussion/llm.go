package discussion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)


type llmContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type llmResponse struct {
	Content []llmContentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (d *DiscussionContext) Submit(userInput string) {
	d.mu.Lock()
	d.State.History = append(d.State.History, Message{Role: "user", Content: userInput})
	snapshot := make([]Message, len(d.State.History))
	copy(snapshot, d.State.History)
	d.mu.Unlock()

	reqBody, err := json.Marshal(struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		Messages  []Message `json:"messages"`
	}{
		Model:     d.Model,
		MaxTokens: 8192,
		Messages:  snapshot,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling request: %v\n", err)
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodPost, d.LLMEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if d.APIKey != "" {
		req.Header.Set("x-api-key", d.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error: %v\n", err)
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	var llmResp llmResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}

	if llmResp.Error != nil {
		fmt.Fprintf(os.Stderr, "API error: %s\n", llmResp.Error.Message)
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}

	if len(llmResp.Content) == 0 {
		fmt.Fprintf(os.Stderr, "API error: empty response content\n")
		d.mu.Lock()
		d.State.History = d.State.History[:len(d.State.History)-1]
		d.mu.Unlock()
		return
	}

	assistantContent := llmResp.Content[0].Text

	d.mu.Lock()
	d.State.History = append(d.State.History, Message{Role: "assistant", Content: assistantContent})
	d.State.LastUpdated = time.Now().UTC()
	_ = PersistState(d.StatePath, d.State)
	d.mu.Unlock()

	fmt.Printf("\n%s\n\n", assistantContent)
}
