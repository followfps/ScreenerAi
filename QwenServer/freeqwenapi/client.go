package freeqwenapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	u := normalizeBaseURL(baseURL)
	return &Client{
		baseURL: u,
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

func (c *Client) ChatCompletions(ctx context.Context, model string, messages []Message, temperature float64, maxTokens int) (string, error) {
	type payload struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature float64   `json:"temperature,omitempty"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Stream      bool      `json:"stream,omitempty"`
	}
	p := payload{
		Model:       strings.TrimSpace(model),
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
	}
	if p.Model == "" {
		p.Model = "qwen-max-latest"
	}
	if maxTokens <= 0 {
		p.MaxTokens = 64
	}

	body, err := json.Marshal(&p)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := readAll(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		preview := strings.TrimSpace(string(raw))
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("freeqwen http %d: %s", resp.StatusCode, preview)
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		preview := strings.TrimSpace(string(raw))
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("freeqwen response json: %v; body=%s", err, preview)
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", fmt.Errorf("freeqwen api error: %s", strings.TrimSpace(parsed.Error.Message))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("freeqwen response: empty choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func normalizeBaseURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		u = "http://localhost:3264/api"
	}
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/api") {
		return u
	}
	return u + "/api"
}

func readAll(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
