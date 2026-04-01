package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/followfps/ScreanerAi/QwenServer/freeqwenproxy"

	"github.com/getlantern/systray"
	"github.com/kbinani/screenshot"
	"golang.design/x/clipboard"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
	"gopkg.in/yaml.v3"
)

//go:embed landscape.ico
var iconBytes []byte

type Config struct {
	Hotkey          string `yaml:"hotkey" json:"hotkey"`
	ClipboardHotkey string `yaml:"clipboard_hotkey" json:"clipboard_hotkey"`
	RootDirectory   string `yaml:"root_directory" json:"root_directory"`
	QwenServerURL   string `yaml:"qwen_server_url" json:"qwen_server_url"`
	AIModel         string `yaml:"ai_model" json:"ai_model"`
	PromptTemplate  string `yaml:"prompt_template" json:"prompt_template"`
	Theme           string `yaml:"theme" json:"theme"`
	QwenToken       string `yaml:"qwen_token" json:"qwen_token"`
	OverlayOpacity  int    `yaml:"overlay_opacity" json:"overlay_opacity"`
	SelectionColor  string `yaml:"selection_color" json:"selection_color"`
	CopyToClipboard bool   `yaml:"copy_to_clipboard" json:"copy_to_clipboard"`
	RunAtStartup    bool   `yaml:"run_at_startup" json:"run_at_startup"`
}

var (
	ctxSrv    context.Context
	cancelSrv context.CancelFunc
)

func main() {
	mainthread.Init(fn)
}

func startQwenServer(ctx context.Context) {
	cfg := freeqwenproxy.DefaultConfig()

	host := strings.TrimSpace(os.Getenv("HOST"))
	if host == "" {
		host = "127.0.0.1"
	}

	port := 3264
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			port = p
		}
	}
	cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))

	cfg.UpstreamBaseURL = strings.TrimSpace(os.Getenv("FREEQWEN_UPSTREAM_BASE_URL"))
	cfg.UpstreamAPIKey = strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
	if cfg.UpstreamAPIKey == "" {
		cfg.UpstreamAPIKey = strings.TrimSpace(os.Getenv("FREEQWEN_API_KEY"))
	}

	srv := freeqwenproxy.NewServer(cfg)

	log.Printf("[QwenServer] Starting server on %s", cfg.Addr)
	_, _, err := srv.ListenAndServe(ctx)
	if err != nil {
		log.Printf("[QwenServer] Error: %v", err)
	}
}

func onReady() {
	if len(iconBytes) > 0 {
		systray.SetIcon(iconBytes)
	}

	systray.SetTitle("ScreenOrganizer")
	systray.SetTooltip("ScreenOrganizer is running")

	mSettings := systray.AddMenuItem("Settings", "Open configuration settings")
	mQuit := systray.AddMenuItem("Exit", "Exit the application")
	go func() {
		for {
			select {
			case <-mSettings.ClickedCh:
				log.Println("Settings clicked")
				go showSettingsWindow()
			case <-mQuit.ClickedCh:
				log.Println("Exiting via tray...")
				if cancelSrv != nil {
					cancelSrv()
				}
				systray.Quit()
				os.Exit(0)
			}
		}
	}()
}

func onExit() {
	if cancelSrv != nil {
		cancelSrv()
	}
}

func fn() {
	execPath, err := os.Executable()
	if err == nil {
		_ = os.Chdir(filepath.Dir(execPath))
	}

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[ScreenOrganizer] ")

	logFile, err := os.OpenFile("screener.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		var writers []io.Writer
		writers = append(writers, logFile)
		if os.Stdout != nil {
			writers = append(writers, os.Stdout)
		}
		multiWriter := io.MultiWriter(writers...)
		log.SetOutput(multiWriter)
	}

	log.Println("Application starting...")

	ctxSrv, cancelSrv = context.WithCancel(context.Background())

	go startQwenServer(ctxSrv)

	go func() {
		systray.Run(onReady, onExit)
	}()

	if err := clipboard.Init(); err != nil {
		log.Printf("Warning: Failed to initialize clipboard: %v", err)
	}

	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Root directory: %s", cfg.RootDirectory)
	log.Printf("QwenServer URL: %s", cfg.QwenServerURL)
	log.Printf("AI model: %s", cfg.AIModel)
	log.Printf("Hotkey: %s", cfg.Hotkey)
	log.Printf("Clipboard Hotkey: %s", cfg.ClipboardHotkey)

	if _, err := os.Stat(cfg.RootDirectory); os.IsNotExist(err) {
		log.Fatalf("Root directory does not exist: %s", cfg.RootDirectory)
	}

	var hk, hkClipboard *hotkey.Hotkey

	registerHotkeys := func(c *Config) {
		if hk != nil {
			hk.Unregister()
			hk = nil
		}
		if hkClipboard != nil {
			hkClipboard.Unregister()
			hkClipboard = nil
		}

		if c.Hotkey != "" {
			mods, key, err := parseHotkey(c.Hotkey)
			if err != nil {
				log.Printf("Failed to parse hotkey '%s': %v", c.Hotkey, err)
			} else {
				hk = hotkey.New(mods, key)
				if err := hk.Register(); err != nil {
					log.Printf("Failed to register hotkey '%s': %v", c.Hotkey, err)
					hk = nil
				} else {
					log.Printf("Global hotkey '%s' registered.", c.Hotkey)
				}
			}
		}

		if c.ClipboardHotkey != "" {
			mods, key, err := parseHotkey(c.ClipboardHotkey)
			if err != nil {
				log.Printf("Failed to parse clipboard hotkey '%s': %v", c.ClipboardHotkey, err)
			} else {
				hkClipboard = hotkey.New(mods, key)
				if err := hkClipboard.Register(); err != nil {
					log.Printf("Failed to register clipboard hotkey '%s': %v", c.ClipboardHotkey, err)
					hkClipboard = nil
				} else {
					log.Printf("Clipboard hotkey '%s' registered.", c.ClipboardHotkey)
				}
			}
		}
	}

	registerHotkeys(cfg)
	defer func() {
		if hk != nil {
			hk.Unregister()
		}
		if hkClipboard != nil {
			hkClipboard.Unregister()
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		var ch1, ch2 <-chan hotkey.Event
		dummy1 := make(chan hotkey.Event)
		dummy2 := make(chan hotkey.Event)

		if hk != nil {
			ch1 = hk.Keydown()
		} else {
			ch1 = dummy1
		}

		if hkClipboard != nil {
			ch2 = hkClipboard.Keydown()
		} else {
			ch2 = dummy2
		}

		select {
		case <-ch1:
			log.Println("Hotkey pressed! Capturing screenshot...")
			go handleScreenshot(cfg)
		case <-ch2:
			log.Println("Clipboard Hotkey pressed! Toggling clipboard monitor...")
			toggleClipboardMonitor(cfg)
		case sig := <-sigChan:
			log.Printf("Received signal %v. Shutting down...", sig)
			return
		case <-reloadChan:
			log.Println("Reloading configuration...")
			newCfg, err := loadConfig("config.yaml")
			if err != nil {
				log.Printf("Failed to load new config: %v", err)
				continue
			}

			if newCfg.Hotkey != cfg.Hotkey || newCfg.ClipboardHotkey != cfg.ClipboardHotkey {
				log.Println("Hotkeys changed. Re-registering...")
				registerHotkeys(newCfg)
			}
			cfg = newCfg
			log.Printf("Configuration reloaded")
		}
	}
}

var (
	reloadChan      = make(chan struct{}, 1)
	clipboardCtx    context.Context
	clipboardCancel context.CancelFunc
	ignoreHash      string
	ignoreHashMu    sync.Mutex
)

func toggleClipboardMonitor(cfg *Config) {
	if clipboardCancel != nil {
		clipboardCancel()
		clipboardCancel = nil
		log.Println("Clipboard monitor OFF")
		go showNotification("Clipboard monitor OFF")
		return
	}

	clipboardCtx, clipboardCancel = context.WithCancel(context.Background())
	ch := clipboard.Watch(clipboardCtx, clipboard.FmtImage)
	log.Println("Clipboard monitor ON")
	go showNotification("Clipboard monitor ON")

	go func(ctx context.Context, config *Config) {
		initialData := clipboard.Read(clipboard.FmtImage)
		initialHash := string(initialData)

		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-ch:
				if !ok {
					return
				}

				ignoreHashMu.Lock()
				if ignoreHash != "" && string(data) == ignoreHash {
					ignoreHash = ""
					ignoreHashMu.Unlock()
					continue
				}
				ignoreHashMu.Unlock()

				if string(data) == initialHash {
					initialHash = ""
					continue
				}
				initialHash = ""

				log.Printf("Image detected in clipboard! Processing %d bytes...", len(data))
				go processImageData(config, data)
			}
		}
	}(clipboardCtx, cfg)
}

func reloadConfigTrigger() {
	select {
	case reloadChan <- struct{}{}:
	default:
	}
}

func saveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	if err := os.WriteFile("config.yaml", data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

func handleScreenshot(cfg *Config) {
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

	selectedRect, ok := selectRegion(vRect, cfg.OverlayOpacity, cfg.SelectionColor)
	if !ok {
		log.Println("Selection cancelled.")
		return
	}
	log.Printf("Region selected: (%d,%d)-(%d,%d)",
		selectedRect.Min.X, selectedRect.Min.Y,
		selectedRect.Max.X, selectedRect.Max.Y)

	cropped := fullImg.SubImage(selectedRect)

	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		log.Printf("ERROR: Failed to encode screenshot as PNG: %v", err)
		return
	}
	pngData := buf.Bytes()
	log.Printf("Cropped screenshot: %dx%d, %d bytes",
		selectedRect.Dx(), selectedRect.Dy(), len(pngData))

	if cfg.CopyToClipboard {
		ignoreHashMu.Lock()
		ignoreHash = string(pngData)
		ignoreHashMu.Unlock()
		clipboard.Write(clipboard.FmtImage, pngData)
	}

	processImageData(cfg, pngData)
}

func processImageData(cfg *Config, pngData []byte) {
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

	imageURL, fileID, err := uploadFileToQwen(cfg, pngData, "screenshot.png")
	if err != nil {
		log.Printf("ERROR: Failed to upload screenshot to QwenServer: %v", err)
		return
	}
	log.Printf("Screenshot uploaded, URL: %s", imageURL)

	chosenFolder, err := askQwenAI(cfg, imageURL, fileID, folders)
	if err != nil {
		log.Printf("ERROR: AI classification failed: %v", err)
		return
	}

	matched := false
	for _, f := range folders {
		if strings.EqualFold(chosenFolder, f) {
			chosenFolder = f
			matched = true
			break
		}
	}

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

	go showNotification(fmt.Sprintf("Successfully saved to %s", chosenFolder))
}

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

func askQwenAI(cfg *Config, imageURL string, fileID string, folders []string) (string, error) {
	folderList := strings.Join(folders, ", ")
	prompt := strings.ReplaceAll(cfg.PromptTemplate, "{folders}", folderList)

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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("QwenServer request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	log.Printf("--- AI REQUEST DEBUG ---")
	log.Printf("Available folders: %v", folders)
	log.Printf("Prompt sent: %s", prompt)
	log.Printf("RAW QwenServer JSON Response: %s", string(respBody))
	log.Printf("------------------------")

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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file '%s': %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML config: %w", err)
	}

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
		cfg.AIModel = "qwen3.5-plus"
	}
	if cfg.PromptTemplate == "" {
		cfg.PromptTemplate = "Based on this screenshot, which of these folders is the most relevant: {folders}? Return ONLY the folder name."
	}
	if cfg.Theme == "" {
		cfg.Theme = "dark"
	}

	return &cfg, nil
}

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

func parseKey(keyStr string) (hotkey.Key, error) {
	if len(keyStr) == 1 && keyStr[0] >= 'a' && keyStr[0] <= 'z' {
		return hotkey.Key(0x41 + (keyStr[0] - 'a')), nil
	}

	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		return hotkey.Key(0x30 + (keyStr[0] - '0')), nil
	}

	fKeys := map[string]hotkey.Key{
		"f1": hotkey.KeyF1, "f2": hotkey.KeyF2, "f3": hotkey.KeyF3,
		"f4": hotkey.KeyF4, "f5": hotkey.KeyF5, "f6": hotkey.KeyF6,
		"f7": hotkey.KeyF7, "f8": hotkey.KeyF8, "f9": hotkey.KeyF9,
		"f10": hotkey.KeyF10, "f11": hotkey.KeyF11, "f12": hotkey.KeyF12,
	}
	if k, ok := fKeys[keyStr]; ok {
		return k, nil
	}

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
