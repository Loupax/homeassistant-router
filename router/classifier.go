package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Classification struct {
	Intent  string `json:"intent"`
	Payload string `json:"payload"`
}

const classifierSystemPrompt = `You are an intent classifier for a home assistant. Classify the user input into exactly one intent and extract a payload if relevant.

Respond with JSON only, no other text:
{"intent": "<intent>", "payload": "<payload>"}

Valid intents:
- play: user wants to play music. payload = search query to use
- stop: user wants to stop music. payload = ""
- volume_up: user wants to increase volume. payload = ""
- volume_down: user wants to decrease volume. payload = ""
- pause: user wants to pause. payload = ""
- resume: user wants to resume. payload = ""
- discuss: anything else, a question, statement, or conversation. payload = ""
- unknown: the user is asking for something the assistant does not support (e.g. playlist control, alarms, timers). payload = ""

Examples:
{"intent":"play","payload":"jazz music"}
{"intent":"stop","payload":""}
{"intent":"unknown","payload":""}
{"intent":"discuss","payload":""}`

type classifierRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []classifierMessage `json:"messages"`
}

type classifierMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type classifierContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type classifierResponse struct {
	Content []classifierContentBlock `json:"content"`
}

func Classify(input, llmEndpoint, apiKey, model string) (Classification, error) {
	reqBody, err := json.Marshal(classifierRequest{
		Model:     model,
		MaxTokens: 64,
		System:    classifierSystemPrompt,
		Messages: []classifierMessage{
			{Role: "user", Content: input},
		},
	})
	if err != nil {
		return Classification{}, fmt.Errorf("classifier: marshal request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, llmEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return Classification{}, fmt.Errorf("classifier: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Classification{}, fmt.Errorf("classifier: network: %w", err)
	}
	defer resp.Body.Close()

	var llmResp classifierResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return Classification{}, fmt.Errorf("classifier: decode response: %w", err)
	}

	if len(llmResp.Content) == 0 {
		return Classification{}, fmt.Errorf("classifier: empty content in response")
	}

	var classification Classification
	if err := json.Unmarshal([]byte(llmResp.Content[0].Text), &classification); err != nil {
		return Classification{}, fmt.Errorf("classifier: decode classification JSON: %w", err)
	}

	return classification, nil
}
