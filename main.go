package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kbinani/screenshot"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration loaded from config.yaml.
type Config struct {
	Hotkey         string `yaml:"hotkey"`
	RootDirectory  string `yaml:"root_directory"`
	QwenServerURL  string `yaml:"qwen_server_url"`
	QwenAPIKey     string `yaml:"qwen_api_key"`
	AIModel        string `yaml:"ai_model"`
	PromptTemplate string `yaml:"prompt_template"`
}

func main() {
	mainthread.Init(fn)
}

func fn() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[ScreenOrganizer] ")

	// Initialize logging to both stdout and a file for debugging
	logFile, err := os.OpenFile("screener.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)
	} else {
		log.Printf("Failed to open log file, using stdout only: %v", err)
	}

	// Load configuration
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Root directory: %s", cfg.RootDirectory)
	log.Printf("QwenServer URL: %s", cfg.QwenServerURL)
	log.Printf("AI model: %s", cfg.AIModel)
	log.Printf("Hotkey: %s", cfg.Hotkey)

	// Validate root directory exists
	if _, err := os.Stat(cfg.RootDirectory); os.IsNotExist(err) {
		log.Fatalf("Root directory does not exist: %s", cfg.RootDirectory)
	}

	// Parse hotkey combination
	mods, key, err := parseHotkey(cfg.Hotkey)
	if err != nil {
		log.Fatalf("Failed to parse hotkey '%s': %v", cfg.Hotkey, err)
	}

	// Register global hotkey
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		log.Fatalf("Failed to register hotkey '%s': %v", cfg.Hotkey, err)
	}
	defer hk.Unregister()

	log.Printf("Global hotkey '%s' registered. Press it to capture and organize a screenshot.", cfg.Hotkey)
	log.Printf("Press Ctrl+C to exit.")

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Main event loop
	for {
		select {
		case <-hk.Keydown():
			log.Println("Hotkey pressed! Capturing screenshot...")
			go handleScreenshot(cfg)
		case sig := <-sigChan:
			log.Printf("Received signal %v. Shutting down...", sig)
			return
		}
	}
}

// handleScreenshot shows the selection overlay, captures the selected region,
// sends it to the QwenServer AI, and saves it to the chosen folder.
func handleScreenshot(cfg *Config) {
	// Step 1: Capture full screen first (across ALL monitors before overlay appears)
	var vRect image.Rectangle
	numDisplays := screenshot.NumActiveDisplays()
	if numDisplays > 0 {
		vRect = screenshot.GetDisplayBounds(0)
		for i := 1; i < numDisplays; i++ {
			vRect = vRect.Union(screenshot.GetDisplayBounds(i))
		}
	} else {
		log.Println("ERROR: No active displays found.")
		return
	}

	fullImg, err := screenshot.CaptureRect(vRect)
	if err != nil {
		log.Printf("ERROR: Failed to capture screenshot: %v", err)
		return
	}
	log.Printf("Full screen captured (%v)", vRect)

	// Step 2: Show selection overlay — user drags to select region across virtual screen
	selectedRect, ok := selectRegion(vRect)
	if !ok {
		log.Println("Selection cancelled.")
		return
	}
	log.Printf("Region selected: (%d,%d)-(%d,%d)",
		selectedRect.Min.X, selectedRect.Min.Y,
		selectedRect.Max.X, selectedRect.Max.Y)

	// Step 3: Crop the selected area from the full capture
	cropped := fullImg.SubImage(selectedRect)

	// Step 4: Encode cropped region to PNG in memory
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		log.Printf("ERROR: Failed to encode screenshot as PNG: %v", err)
		return
	}
	pngData := buf.Bytes()
	log.Printf("Cropped screenshot: %dx%d, %d bytes",
		selectedRect.Dx(), selectedRect.Dy(), len(pngData))

	// Step 5: Get list of subdirectories in root_directory
	folders, err := getSubdirectories(cfg.RootDirectory)
	if err != nil {
		log.Printf("ERROR: Failed to read subdirectories: %v", err)
		return
	}
	if len(folders) == 0 {
		log.Printf("WARNING: No subdirectories found in '%s'. AI will default to fallback 'etc'.", cfg.RootDirectory)
	} else {
		log.Printf("Found %d folders: %v", len(folders), folders)
	}

	// Step 6: Upload the screenshot to QwenServer
	imageURL, fileID, err := uploadFileToQwen(cfg, pngData, "screenshot.png")
	if err != nil {
		log.Printf("ERROR: Failed to upload screenshot to QwenServer: %v", err)
		return
	}
	log.Printf("Screenshot uploaded, URL: %s", imageURL)

	// Step 7: Ask the AI which folder to use
	chosenFolder, err := askQwenAI(cfg, imageURL, fileID, folders)
	if err != nil {
		log.Printf("ERROR: AI request failed: %v", err)
		return
	}
	chosenFolder = strings.TrimSpace(chosenFolder)

	// Step 8: Validate the AI's choice (robust parsing)
	matched := false
	// Fast exact match first
	for _, f := range folders {
		if strings.EqualFold(chosenFolder, f) {
			chosenFolder = f
			matched = true
			break
		}
	}

	// Fallback to substring matching (AI often includes chatter like "Folder: Work")
	if !matched {
		lowerResp := strings.ToLower(chosenFolder)
		for _, f := range folders {
			if strings.Contains(lowerResp, strings.ToLower(f)) {
				log.Printf("INFO: Extracted folder '%s' from AI response: '%s'", f, chosenFolder)
				chosenFolder = f
				matched = true
				break
			}
		}
	}

	if !matched {
		log.Printf("WARNING: AI returned unrecognized folder: '%s'. Using 'etc' as fallback.", chosenFolder)
		chosenFolder = "etc"
	} else {
		log.Printf("AI chose folder: '%s'", chosenFolder)
	}

	// Step 9: Save the screenshot with a timestamped filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s.png", timestamp)
	destDir := filepath.Join(cfg.RootDirectory, chosenFolder)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create target directory '%s': %v", destDir, err)
		return
	}

	if err := os.WriteFile(destPath, pngData, 0644); err != nil {
		log.Printf("ERROR: Failed to save screenshot to '%s': %v", destPath, err)
		return
	}

	log.Printf("Screenshot saved to: %s", destPath)
}

// uploadFileToQwen uploads an image to the QwenServer file upload endpoint
// and returns the file URL and ID for use in multimodal messages.
func uploadFileToQwen(cfg *Config, fileData []byte, filename string) (string, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", "", fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return "", "", fmt.Errorf("writing file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", "", fmt.Errorf("closing multipart writer: %w", err)
	}

	uploadURL := normalizeBaseURL(cfg.QwenServerURL) + "/files/upload"
	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", "", fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cfg.QwenAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.QwenAPIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", "", fmt.Errorf("upload HTTP %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Success bool `json:"success"`
		File    struct {
			URL  string `json:"url"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"file"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("parsing upload response: %w", err)
	}
	if !result.Success || result.File.URL == "" {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return "", "", fmt.Errorf("upload failed: %s", errMsg)
	}

	return result.File.URL, result.File.ID, nil
}

// askQwenAI sends the screenshot URL and folder list to the QwenServer vision model
// and returns the chosen folder name.
func askQwenAI(cfg *Config, imageURL string, fileID string, folders []string) (string, error) {
	folderList := strings.Join(folders, ", ")
	prompt := strings.ReplaceAll(cfg.PromptTemplate, "{folders}", folderList)

	// Build multimodal content: text prompt + image URL (Qwen VL format)
	content := []map[string]any{
		{
			"type": "text",
			"text": prompt,
		},
		{
			"type":      "image_url",
			"image_url": map[string]string{"url": imageURL, "file_id": fileID},
		},
	}

	payload := map[string]any{
		"model": cfg.AIModel,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"stream": false,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	completionsURL := normalizeBaseURL(cfg.QwenServerURL) + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, completionsURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.QwenAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.QwenAPIKey)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("QwenServer request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	// VERBOSE LOGGING START
	log.Printf("--- AI REQUEST DEBUG ---")
	log.Printf("Available folders: %v", folders)
	log.Printf("Prompt sent: %s", prompt)
	log.Printf("RAW QwenServer JSON Response: %s", string(respBody))
	log.Printf("------------------------")
	// VERBOSE LOGGING END

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("QwenServer HTTP %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
		return "", fmt.Errorf("QwenServer error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("QwenServer returned no choices")
	}

	answer := strings.TrimSpace(result.Choices[0].Message.Content)
	log.Printf("AI parsed answer: '%s'", answer)
	return answer, nil
}

// normalizeBaseURL ensures the URL ends with /api.
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

// getSubdirectories returns a list of immediate subdirectory names inside the given path.
func getSubdirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading directory '%s': %w", root, err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

// contains checks if a string exists in a slice (case-insensitive).
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// loadConfig reads and parses the YAML configuration file.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file '%s': %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML config: %w", err)
	}

	// Validate required fields
	if cfg.RootDirectory == "" {
		return nil, fmt.Errorf("root_directory is not configured in %s", path)
	}
	if cfg.Hotkey == "" {
		cfg.Hotkey = "ctrl+shift+s"
	}
	if cfg.QwenServerURL == "" {
		cfg.QwenServerURL = "http://localhost:3264"
	}
	if cfg.AIModel == "" {
		cfg.AIModel = "qwen-vl-max"
	}
	if cfg.PromptTemplate == "" {
		cfg.PromptTemplate = "Based on this screenshot, which of these folders is the most relevant: {folders}? Return ONLY the folder name."
	}

	return &cfg, nil
}

// parseHotkey converts a string like "ctrl+shift+s" into hotkey modifiers and a key.
func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(hotkeyStr), "+")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("hotkey must have at least one modifier and a key (e.g., 'ctrl+shift+s')")
	}

	var mods []hotkey.Modifier
	for _, part := range parts[:len(parts)-1] {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "alt":
			mods = append(mods, hotkey.ModAlt)
		case "win", "super", "cmd", "command":
			mods = append(mods, hotkey.ModWin)
		default:
			return nil, 0, fmt.Errorf("unknown modifier: '%s'", part)
		}
	}

	keyStr := strings.TrimSpace(parts[len(parts)-1])
	key, err := parseKey(keyStr)
	if err != nil {
		return nil, 0, err
	}

	return mods, key, nil
}

// parseKey converts a key string to a hotkey.Key value.
func parseKey(keyStr string) (hotkey.Key, error) {
	// Letters A-Z
	if len(keyStr) == 1 && keyStr[0] >= 'a' && keyStr[0] <= 'z' {
		return hotkey.Key(0x41 + (keyStr[0] - 'a')), nil
	}

	// Digits 0-9
	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		return hotkey.Key(0x30 + (keyStr[0] - '0')), nil
	}

	// Function keys
	fKeys := map[string]hotkey.Key{
		"f1": hotkey.KeyF1, "f2": hotkey.KeyF2, "f3": hotkey.KeyF3,
		"f4": hotkey.KeyF4, "f5": hotkey.KeyF5, "f6": hotkey.KeyF6,
		"f7": hotkey.KeyF7, "f8": hotkey.KeyF8, "f9": hotkey.KeyF9,
		"f10": hotkey.KeyF10, "f11": hotkey.KeyF11, "f12": hotkey.KeyF12,
	}
	if k, ok := fKeys[keyStr]; ok {
		return k, nil
	}

	// Special keys
	special := map[string]hotkey.Key{
		"space":  hotkey.KeySpace,
		"return": hotkey.KeyReturn,
		"enter":  hotkey.KeyReturn,
		"escape": hotkey.KeyEscape,
		"esc":    hotkey.KeyEscape,
		"tab":    hotkey.KeyTab,
		"up":     hotkey.KeyUp,
		"down":   hotkey.KeyDown,
		"left":   hotkey.KeyLeft,
		"right":  hotkey.KeyRight,
	}
	if k, ok := special[keyStr]; ok {
		return k, nil
	}

	return 0, fmt.Errorf("unknown key: '%s' (supported: a-z, 0-9, f1-f12, space, enter, escape, tab, arrow keys)", keyStr)
}
