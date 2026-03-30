package freeqwenproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type QwenClient struct {
	baseURL     string
	httpClient  *http.Client
	tokenSource *TokenManager
}

func NewQwenClient(baseURL string, tokens *TokenManager) *QwenClient {
	b := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if b == "" {
		b = "https://chat.qwen.ai"
	}
	return &QwenClient{
		baseURL: b,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		tokenSource: tokens,
	}
}

type CreateChatResult struct {
	ChatID    string
	RequestID string
}

func (c *QwenClient) RegisterFile(ctx context.Context, bearerToken string, info FileInfo, sts StsTokenResponse) error {
	payload := map[string]any{
		"file_id":    sts.FileID,
		"file_name":  info.Filename,
		"file_size":  info.Filesize,
		"file_type":  info.Filetype,
		"oss_key":    sts.FilePath,
		"oss_bucket": sts.Bucket,
		"category":   info.Category,
	}
	raw, err := json.Marshal(map[string]any{"file": payload})
	if err != nil {
		return err
	}

	u := c.baseURL + "/api/v1/files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return httpStatusError(resp.StatusCode, body)
	}

	return nil
}

func (c *QwenClient) UploadFile(ctx context.Context, bearerToken string, localPath string) (MapFileInfo, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return MapFileInfo{}, err
	}
	defer f.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add category field
	_ = writer.WriteField("category", "chat")

	// Add the file
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return MapFileInfo{}, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return MapFileInfo{}, err
	}
	writer.Close()

	u := c.baseURL + "/api/v1/files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return MapFileInfo{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MapFileInfo{}, err
	}
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return MapFileInfo{}, httpStatusError(resp.StatusCode, resBody)
	}

	var parsed struct {
		Success bool `json:"success"`
		Data    struct {
			FileID string `json:"file_id"`
			URL    string `json:"url"`
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			Type   string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return MapFileInfo{}, fmt.Errorf("upload json: %w", err)
	}
	if !parsed.Success {
		return MapFileInfo{}, fmt.Errorf("upload failed: %s", string(resBody))
	}

	return MapFileInfo{
		FileID:   parsed.Data.FileID,
		Filename: parsed.Data.Name,
		Filesize: parsed.Data.Size,
		Filetype: parsed.Data.Type,
		URL:      parsed.Data.URL,
	}, nil
}

type MapFileInfo struct {
	FileID   string
	Filename string
	Filesize int64
	Filetype string
	URL      string
}

func (c *QwenClient) CreateChatV2(ctx context.Context, model, title string) (CreateChatResult, *TokenEntry, error) {
	if strings.TrimSpace(model) == "" {
		model = "qwen-max-latest"
	}
	if strings.TrimSpace(title) == "" {
		title = "Новый чат"
	}

	token, err := c.tokenSource.GetAvailableToken()
	if err != nil {
		return CreateChatResult{}, nil, err
	}
	if token == nil || strings.TrimSpace(token.Token) == "" {
		return CreateChatResult{}, nil, fmt.Errorf("no available qwen token")
	}

	created, err := c.CreateChatV2WithToken(ctx, token.Token, model, title)
	if err != nil {
		return CreateChatResult{}, token, err
	}
	return created, token, nil
}

func (c *QwenClient) CreateChatV2WithToken(ctx context.Context, bearerToken string, model, title string) (CreateChatResult, error) {
	reqBody := map[string]any{
		"title":     title,
		"models":    []string{model},
		"chat_mode": "normal",
		"chat_type": "t2t",
		"timestamp": time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return CreateChatResult{}, err
	}

	u := c.baseURL + "/api/v2/chats/new"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return CreateChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CreateChatResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return CreateChatResult{}, httpStatusError(resp.StatusCode, body)
	}

	var parsed struct {
		Success   bool   `json:"success"`
		RequestID string `json:"request_id"`
		Data      struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CreateChatResult{}, fmt.Errorf("qwen create chat json: %w", err)
	}
	if !parsed.Success || strings.TrimSpace(parsed.Data.ID) == "" {
		return CreateChatResult{}, fmt.Errorf("qwen create chat failed")
	}

	return CreateChatResult{ChatID: parsed.Data.ID, RequestID: parsed.RequestID}, nil
}

type SendMessageResult struct {
	Completion map[string]any
	TokenUsed  *TokenEntry
}

func (c *QwenClient) SendMessageV2(ctx context.Context, message any, model, chatID, parentID string, tools any, toolChoice any, systemMessage any, files []any) (SendMessageResult, error) {
	if strings.TrimSpace(model) == "" {
		model = "qwen-max-latest"
	}

	if strings.TrimSpace(chatID) == "" {
		created, _, err := c.CreateChatV2(ctx, model, "Новый чат")
		if err != nil {
			return SendMessageResult{}, fmt.Errorf("create chat: %w", err)
		}
		chatID = created.ChatID
	}

	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		token, err := c.tokenSource.GetAvailableToken()
		if err != nil {
			return SendMessageResult{}, err
		}
		if token == nil || strings.TrimSpace(token.Token) == "" {
			if lastErr != nil {
				return SendMessageResult{}, lastErr
			}
			return SendMessageResult{}, fmt.Errorf("no available qwen token")
		}

		comp, status, body, err := c.sendMessageOnce(ctx, token.Token, message, model, chatID, parentID, tools, toolChoice, systemMessage, files)
		if err == nil {
			if token.ID != "" {
				_ = c.tokenSource.MarkValid(token.ID, "")
			}
			return SendMessageResult{Completion: comp, TokenUsed: token}, nil
		}

		lastErr = err

		if status == 401 || status == 403 || strings.Contains(strings.ToLower(string(body)), "unauthorized") || strings.Contains(strings.ToLower(string(body)), "token has expired") {
			if token.ID != "" {
				_ = c.tokenSource.MarkInvalid(token.ID)
			}
			continue
		}
		if status == 429 && bytes.Contains(bytes.ToLower(body), []byte("ratelimited")) {
			hours := parseRateLimitHours(body)
			if token.ID != "" {
				_ = c.tokenSource.MarkRateLimited(token.ID, hours)
			}
			continue
		}

		return SendMessageResult{}, err
	}

	if lastErr != nil {
		return SendMessageResult{}, lastErr
	}
	return SendMessageResult{}, fmt.Errorf("send message failed")
}

func (c *QwenClient) sendMessageOnce(ctx context.Context, bearerToken string, message any, model, chatID, parentID string, tools any, toolChoice any, systemMessage any, files []any) (map[string]any, int, []byte, error) {
	nowSec := time.Now().Unix()

	userMessageID := newHexID(16)
	assistantChildID := newHexID(16)

	contentBody := message
	if len(files) > 0 {
		if s, ok := contentBody.(string); ok && !strings.Contains(s, "[file_0]") {
			contentBody = "[file_0]\n" + s
		}
	}

	newMessage := map[string]any{
		"fid":           userMessageID,
		"parentId":      emptyToNil(parentID),
		"parent_id":     emptyToNil(parentID),
		"role":          "user",
		"content":       contentBody,
		"chat_type":     "t2t",
		"sub_chat_type": "t2t",
		"timestamp":     nowSec,
		"user_action":   "chat",
		"models":        []string{model},
		"files":         files,
		"childrenIds":   []string{assistantChildID},
		"extra": map[string]any{
			"meta": map[string]any{
				"subChatType": "t2t",
			},
		},
		"feature_config": map[string]any{
			"thinking_enabled": false,
		},
	}

	payload := map[string]any{
		"stream":             true,
		"incremental_output": true,
		"chat_id":            chatID,
		"chat_mode":          "normal",
		"messages":           []any{newMessage},
		"model":              model,
		"parent_id":          emptyToNil(parentID),
		"timestamp":          nowSec,
	}

	if systemMessage != nil {
		payload["system_message"] = systemMessage
	}
	if tools != nil {
		payload["tools"] = tools
		if toolChoice == nil {
			payload["tool_choice"] = "auto"
		} else {
			payload["tool_choice"] = toolChoice
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, nil, err
	}

	u, err := url.Parse(c.baseURL + "/api/v2/chat/completions")
	if err != nil {
		return nil, 0, nil, err
	}
	q := u.Query()
	q.Set("chat_id", chatID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://chat.qwen.ai")
	req.Header.Set("Referer", "https://chat.qwen.ai/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return nil, resp.StatusCode, body, httpStatusError(resp.StatusCode, body)
	}

	fullContent, responseID, usage := parseQwenSSE(resp.Body)

	completion := map[string]any{
		"id":      responseID,
		"object":  "chat.completion",
		"created": nowSec,
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": fullContent,
				},
				"finish_reason": "stop",
			},
		},
		"usage":       usage,
		"response_id": responseID,
		"chatId":      chatID,
		"parentId":    responseID,
	}
	if strings.TrimSpace(responseID) == "" {
		completion["id"] = "chatcmpl-" + fmt.Sprint(time.Now().UnixMilli())
		completion["response_id"] = nil
		completion["parentId"] = nil
	}

	return completion, resp.StatusCode, nil, nil
}

func emptyToNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func parseQwenSSE(r io.Reader) (fullContent string, responseID string, usage any) {
	var contentBuilder strings.Builder

	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 8*1024*1024)

	for sc.Scan() {
		line := sc.Text()
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if jsonStr == "" {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
			continue
		}

		if v, ok := raw["response.created"]; ok && len(v) > 0 {
			var created struct {
				ResponseID string `json:"response_id"`
			}
			if err := json.Unmarshal(v, &created); err == nil && strings.TrimSpace(created.ResponseID) != "" {
				responseID = created.ResponseID
			}
		}

		if v, ok := raw["choices"]; ok && len(v) > 0 {
			var choices []struct {
				Delta struct {
					Content string `json:"content"`
					Status  string `json:"status"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(v, &choices); err == nil && len(choices) > 0 {
				if choices[0].Delta.Content != "" {
					contentBuilder.WriteString(choices[0].Delta.Content)
				}
				if strings.EqualFold(choices[0].Delta.Status, "finished") {
					break
				}
			}
		}

		if v, ok := raw["usage"]; ok && len(v) > 0 {
			var u any
			if err := json.Unmarshal(v, &u); err == nil {
				usage = u
			}
		}
	}

	if usage == nil {
		usage = map[string]any{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}
	}
	return contentBuilder.String(), responseID, usage
}

func parseRateLimitHours(body []byte) int {
	var parsed struct {
		Num any `json:"num"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 24
	}
	switch v := parsed.Num.(type) {
	case float64:
		if v <= 0 {
			return 24
		}
		return int(v)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 24
		}
		if i, err := strconvAtoi(v); err == nil && i > 0 {
			return i
		}
		return 24
	default:
		return 24
	}
}

func strconvAtoi(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not int")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func httpStatusError(status int, body []byte) error {
	preview := strings.TrimSpace(string(body))
	if len(preview) > 800 {
		preview = preview[:800]
	}
	if preview == "" {
		return fmt.Errorf("qwen http %d", status)
	}
	return fmt.Errorf("qwen http %d: %s", status, preview)
}
