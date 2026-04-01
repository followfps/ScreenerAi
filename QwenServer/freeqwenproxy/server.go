package freeqwenproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	cfg          Config
	tokenMgr     *TokenManager
	qwen         *QwenClient
	httpClient   *http.Client
	loadedModels []string
}

func NewServer(cfg Config) *Server {
	def := DefaultConfig()
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = def.Addr
	}
	if strings.TrimSpace(cfg.QwenBaseURL) == "" {
		cfg.QwenBaseURL = def.QwenBaseURL
	}
	if strings.TrimSpace(cfg.ModelsFilePath) == "" {
		cfg.ModelsFilePath = def.ModelsFilePath
	}
	if strings.TrimSpace(cfg.AuthKeysPath) == "" {
		cfg.AuthKeysPath = def.AuthKeysPath
	}
	if strings.TrimSpace(cfg.TokensFilePath) == "" {
		cfg.TokensFilePath = def.TokensFilePath
	}
	if strings.TrimSpace(cfg.UploadsDirPath) == "" {
		cfg.UploadsDirPath = def.UploadsDirPath
	}
	if cfg.ChunkRuneSize <= 0 {
		cfg.ChunkRuneSize = def.ChunkRuneSize
	}
	if cfg.ChunkDelayMilli <= 0 {
		cfg.ChunkDelayMilli = def.ChunkDelayMilli
	}

	tm := NewTokenManager(cfg.TokensFilePath)
	return &Server{
		cfg:      cfg,
		tokenMgr: tm,
		qwen:     NewQwenClient(cfg.QwenBaseURL, tm),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		loadedModels: loadModels(cfg.ModelsFilePath),
	}
}

func (s *Server) HTTPHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/api", s.handleAPI)

	return s.withCORS(s.withAuth(mux))
}

func (s *Server) ListenAndServe(ctx context.Context) (net.Listener, *http.Server, error) {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return nil, nil, err
	}

	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.HTTPHandler(),
	}

	go func() {
		<-ctx.Done()
		log.Printf("[QwenServer] Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[QwenServer] Shutdown error: %v", err)
		}
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[QwenServer] Serve error: %v", err)
		}
	}()

	return ln, srv, nil
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}

		apiKeys := loadAuthKeys(s.cfg.AuthKeysPath)
		if len(apiKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Требуется авторизация"})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		ok := false
		for _, k := range apiKeys {
			if token == k {
				ok = true
				break
			}
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Недействительный токен"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	p := normalizeAPIVersionPath(r.URL.Path)

	switch {
	case r.Method == http.MethodPost && p == "/api/chat":
		s.handleChat(w, r)
		return
	case r.Method == http.MethodPost && p == "/api/chat/completions":
		s.handleChatCompletions(w, r)
		return
	case r.Method == http.MethodGet && p == "/api/models":
		s.handleModels(w, r)
		return
	case r.Method == http.MethodGet && p == "/api/status":
		s.handleStatus(w, r)
		return
	case r.Method == http.MethodPost && p == "/api/chats":
		s.handleCreateChat(w, r)
		return
	case r.Method == http.MethodPost && p == "/api/files/getstsToken":
		s.handleGetStsToken(w, r)
		return
	case r.Method == http.MethodPost && p == "/api/files/upload":
		s.handleUploadFile(w, r)
		return
	case r.Method == http.MethodPost && p == "/api/analyze/network":
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Эндпоинт не найден"})
		return
	}
}

func normalizeAPIVersionPath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	p = strings.ReplaceAll(p, "/v1/", "/")
	p = strings.ReplaceAll(p, "/v2/", "/")
	p = strings.TrimSuffix(p, "/v1")
	p = strings.TrimSuffix(p, "/v2")
	return p
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models := s.loadedModels
	if len(models) == 0 {
		models = []string{"qwen-max-latest"}
	}
	data := make([]any, 0, len(models))
	for _, m := range models {
		id := strings.TrimSpace(m)
		if id == "" {
			continue
		}
		data = append(data, map[string]any{
			"id":         id,
			"object":     "model",
			"created":    0,
			"owned_by":   "openai",
			"permission": []any{},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   data,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.tokenMgr.ListTokens()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"message":       "Не удалось прочитать tokens.json",
			"accounts":      []any{},
		})
		return
	}

	accounts := make([]any, 0, len(tokens))
	now := time.Now()
	for _, t := range tokens {
		acc := map[string]any{
			"id":      t.ID,
			"status":  "UNKNOWN",
			"resetAt": nil,
			"error":   nil,
		}
		if strings.TrimSpace(t.ResetAt) != "" {
			acc["resetAt"] = t.ResetAt
			if resetAt, err := time.Parse(time.RFC3339, t.ResetAt); err == nil && resetAt.After(now) {
				acc["status"] = "WAIT"
				accounts = append(accounts, acc)
				continue
			}
		}
		if t.Invalid {
			acc["status"] = "INVALID"
			accounts = append(accounts, acc)
			continue
		}
		if strings.TrimSpace(t.Token) == "" {
			acc["status"] = "INVALID"
			accounts = append(accounts, acc)
			continue
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, testErr := s.qwen.CreateChatV2WithToken(ctx, t.Token, "qwen-max-latest", "ping")
		cancel()
		if testErr == nil {
			acc["status"] = "OK"
		} else {
			msg := strings.ToLower(testErr.Error())
			if strings.Contains(msg, "http 401") || strings.Contains(msg, "http 403") {
				acc["status"] = "INVALID"
				_ = s.tokenMgr.MarkInvalid(t.ID)
			} else if strings.Contains(msg, "http 429") {
				acc["status"] = "WAIT"
				_ = s.tokenMgr.MarkRateLimited(t.ID, 24)
			} else {
				acc["status"] = "ERROR"
				errText := strings.TrimSpace(testErr.Error())
				if errText != "" {
					if len(errText) > 600 {
						errText = errText[:600] + "..."
					}
					acc["error"] = errText
				}
			}
		}
		accounts = append(accounts, acc)
	}

	if len(tokens) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"message":       "Не найдено ни одного аккаунта (tokens.json пуст или отсутствует)",
			"accounts":      accounts,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts": accounts,
	})
}

type chatRequest struct {
	Message  any    `json:"message"`
	Messages []any  `json:"messages"`
	Model    string `json:"model"`
	ChatID   string `json:"chatId"`
	ParentID string `json:"parentId"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := decodeJSON(r, &req, 150<<20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный JSON"})
		return
	}

	messageContent := req.Message
	var systemMessage any
	if len(req.Messages) > 0 {
		msgs, lastUser, sys := extractMessages(req.Messages)
		_ = msgs
		if lastUser != nil {
			messageContent = lastUser
		}
		if sys != nil {
			systemMessage = sys
		}
	}

	if messageContent == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Сообщение не указано"})
		return
	}

	mappedModel := GetMappedModel(req.Model, "qwen-max-latest")

	result, err := s.sendViaSelectedBackend(r.Context(), messageContent, mappedModel, req.ChatID, req.ParentID, nil, nil, systemMessage, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type openAIChatCompletionsRequest struct {
	Messages   []map[string]any `json:"messages"`
	Model      string           `json:"model"`
	Stream     bool             `json:"stream"`
	Tools      any              `json:"tools"`
	Functions  []any            `json:"functions"`
	ToolChoice any              `json:"tool_choice"`
	ChatID     string           `json:"chatId"`
	ParentID   string           `json:"parentId"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req openAIChatCompletionsRequest
	if err := decodeJSON(r, &req, 150<<20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Сообщения не указаны"})
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Сообщения не указаны"})
		return
	}

	var lastUser any
	var systemMessage any
	var files []any
	for _, m := range req.Messages {
		role, _ := m["role"].(string)
		if role == "system" {
			systemMessage = m["content"]
		}
		if role == "user" {
			lastUser = m["content"]
			if arr, ok := lastUser.([]any); ok {
				var textContent strings.Builder
				for _, item := range arr {
					obj, ok := item.(map[string]any)
					if !ok {
						continue
					}
					t, _ := obj["type"].(string)
					switch t {
					case "text":
						if s, _ := obj["text"].(string); s != "" {
							textContent.WriteString(s)
							textContent.WriteString("\n")
						}
					case "image_url":
						if iu, ok := obj["image_url"].(map[string]any); ok {
							fid, _ := iu["file_id"].(string)
							if fid != "" {
								entry := map[string]any{
									"id":      fid,
									"file_id": fid,
									"type":    "image",
								}
								if url, ok := iu["url"].(string); ok && url != "" {
									entry["url"] = url
								}
								files = append(files, entry)
							}
						}
					}
				}
				if len(files) > 0 {
					lastUser = strings.TrimSpace(textContent.String())
				}
			}
		}
	}
	if lastUser == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "В запросе нет сообщений от пользователя"})
		return
	}

	mappedModel := GetMappedModel(req.Model, "qwen-max-latest")
	combinedTools := req.Tools
	if combinedTools == nil && len(req.Functions) > 0 {
		var tools []any
		for _, fn := range req.Functions {
			tools = append(tools, map[string]any{"type": "function", "function": fn})
		}
		combinedTools = tools
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		writeSSE(w, map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   mappedModel,
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         map[string]any{"role": "assistant"},
					"finish_reason": nil,
				},
			},
		})

		result, err := s.sendViaSelectedBackend(r.Context(), lastUser, mappedModel, req.ChatID, req.ParentID, combinedTools, req.ToolChoice, systemMessage, files)
		if err != nil {
			writeSSE(w, map[string]any{
				"id":      "chatcmpl-stream",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   mappedModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Error: " + err.Error()}, "finish_reason": nil}},
			})
			writeDone(w)
			return
		}

		content := ""
		if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
			if c0, ok := choices[0].(map[string]any); ok {
				if msg, ok := c0["message"].(map[string]any); ok {
					if s, ok := msg["content"].(string); ok {
						content = s
					}
				}
			}
		}

		runes := []rune(content)
		chunkSize := s.cfg.ChunkRuneSize
		if chunkSize <= 0 {
			chunkSize = 16
		}
		delay := time.Duration(s.cfg.ChunkDelayMilli) * time.Millisecond
		if delay <= 0 {
			delay = 20 * time.Millisecond
		}

		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			chunk := string(runes[i:end])
			writeSSE(w, map[string]any{
				"id":      "chatcmpl-stream",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   mappedModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": chunk}, "finish_reason": nil}},
			})
			select {
			case <-r.Context().Done():
				return
			case <-time.After(delay):
			}
		}

		writeSSE(w, map[string]any{
			"id":      "chatcmpl-stream",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   mappedModel,
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		})
		writeDone(w)
		return
	}

	result, err := s.sendViaSelectedBackend(r.Context(), lastUser, mappedModel, req.ChatID, req.ParentID, combinedTools, req.ToolChoice, systemMessage, files)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}

	openaiResponse := map[string]any{
		"id":       result["id"],
		"object":   "chat.completion",
		"created":  time.Now().Unix(),
		"model":    mappedModel,
		"choices":  result["choices"],
		"usage":    result["usage"],
		"chatId":   result["chatId"],
		"parentId": result["parentId"],
	}
	if openaiResponse["id"] == nil {
		openaiResponse["id"] = "chatcmpl-" + fmt.Sprint(time.Now().UnixMilli())
	}
	writeJSON(w, http.StatusOK, openaiResponse)
}

func (s *Server) handleCreateChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	}
	if err := decodeJSON(r, &req, 1<<20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный JSON"})
		return
	}
	mappedModel := GetMappedModel(req.Model, "qwen-max-latest")

	if s.hasUpstream() {
		writeJSON(w, http.StatusOK, map[string]any{"chatId": "chat-" + newHexID(8), "success": true})
		return
	}

	created, _, err := s.qwen.CreateChatV2(r.Context(), mappedModel, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chatId": created.ChatID, "success": true})
}

func (s *Server) handleGetStsToken(w http.ResponseWriter, r *http.Request) {
	var info FileInfo
	if err := decodeJSON(r, &info, 1<<20); err != nil || info.Filename == "" || info.Filesize <= 0 || info.Filetype == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректные данные о файле"})
		return
	}
	if s.hasUpstream() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "files/getstsToken не поддерживается в upstream режиме"})
		return
	}

	raw, _, err := s.qwen.GetStsTokenRaw(r.Context(), info)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	if s.hasUpstream() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "files/upload не поддерживается в upstream режиме"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Файл не был загружен"})
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Файл не был загружен"})
		return
	}
	defer f.Close()

	localPath, size, err := saveMultipartFile(s.cfg.UploadsDirPath, f, hdr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Внутренняя ошибка сервера"})
		return
	}
	defer os.Remove(localPath)

	mimeType := hdr.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(hdr.Filename))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	fileInfo := FileInfo{
		Filename: hdr.Filename,
		Filesize: size,
		Filetype: detectQwenFileType(hdr.Filename),
		Category: "chat",
	}

	sts, _, err := s.qwen.GetStsToken(r.Context(), fileInfo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Ошибка при получении токена: %v", err)})
		return
	}

	if err := uploadToAliyunOSS(r.Context(), localPath, sts); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Ошибка при загрузке файла: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"file": map[string]any{
			"name": hdr.Filename,
			"url":  sts.FileURL,
			"id":   sts.FileID,
			"size": size,
			"type": mimeType,
		},
	})
}

func (s *Server) sendViaSelectedBackend(ctx context.Context, message any, model, chatID, parentID string, tools any, toolChoice any, systemMessage any, files []any) (map[string]any, error) {
	if s.hasUpstream() {
		content, err := s.upstreamChatCompletion(ctx, model, message)
		if err != nil {
			return nil, err
		}
		now := time.Now().Unix()
		id := "chatcmpl-" + fmt.Sprint(time.Now().UnixMilli())
		out := map[string]any{
			"id":      id,
			"object":  "chat.completion",
			"created": now,
			"model":   model,
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": content,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
			"chatId":   emptyIfZero(chatID, "chat-"+newHexID(8)),
			"parentId": emptyIfZero(parentID, ""),
		}
		return out, nil
	}

	res, err := s.qwen.SendMessageV2(ctx, message, model, chatID, parentID, tools, toolChoice, systemMessage, files)
	if err != nil {
		return nil, err
	}
	return res.Completion, nil
}

func emptyIfZero(s string, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func (s *Server) hasUpstream() bool {
	return strings.TrimSpace(s.cfg.UpstreamBaseURL) != "" && strings.TrimSpace(s.cfg.UpstreamAPIKey) != ""
}

func (s *Server) upstreamChatCompletion(ctx context.Context, model string, message any) (string, error) {
	content, ok := message.(string)
	if !ok {
		if b, err := json.Marshal(message); err == nil {
			content = string(b)
		} else {
			content = fmt.Sprint(message)
		}
	}
	payload := map[string]any{
		"model": model,
		"messages": []any{
			map[string]any{"role": "user", "content": content},
		},
		"stream": false,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	u := normalizeAPIBaseURL(s.cfg.UpstreamBaseURL) + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.cfg.UpstreamAPIKey))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", httpStatusError(resp.StatusCode, body)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", errors.New(strings.TrimSpace(parsed.Error.Message))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("upstream: empty choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func decodeJSON(r *http.Request, dst any, limit int64) error {
	if limit <= 0 {
		limit = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, limit))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func writeSSE(w http.ResponseWriter, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeDone(w http.ResponseWriter) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func extractMessages(messages []any) (all []any, lastUser any, system any) {
	all = messages
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			if system == nil {
				system = msg["content"]
			}
		case "user":
			lastUser = msg["content"]
		}
	}
	return all, lastUser, system
}
