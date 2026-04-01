//go:build windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/followfps/ScreanerAi/QwenServer/freeqwenproxy"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows/registry"
)

var (
	settingsWindowMutex sync.Mutex
	isSettingsOpen      bool
)

func showSettingsWindow() {
	settingsWindowMutex.Lock()
	if isSettingsOpen {
		settingsWindowMutex.Unlock()
		return
	}
	isSettingsOpen = true
	settingsWindowMutex.Unlock()

	defer func() {
		settingsWindowMutex.Lock()
		isSettingsOpen = false
		settingsWindowMutex.Unlock()
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Printf("Error loading config: %v", err)
		return
	}

	w := webview2.New(false)
	if w == nil {
		log.Printf("Failed to create webview. Make sure WebView2 Runtime is installed.")
		return
	}
	defer w.Destroy()

	hwnd := uintptr(w.Window())

	style, _, _ := pGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= 0x00C00000
	style &^= 0x00040000
	pSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	darkValue := uintptr(1)
	pDwmSetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&darkValue)), 4)

	w.SetTitle("ScreanerAi Settings")
	w.SetSize(500, 750, webview2.HintFixed)

	w.Bind("saveSettings", func(settingsJson string) {
		var newCfg Config
		if err := json.Unmarshal([]byte(settingsJson), &newCfg); err != nil {
			log.Printf("Error parsing settings from UI: %v", err)
			return
		}

		if newCfg.QwenToken != "" {
			tokenPath := filepath.Join("session", "tokens.json")
			tm := freeqwenproxy.NewTokenManager(tokenPath)
			if err := tm.AddOrUpdate("manual_user", newCfg.QwenToken); err != nil {
				log.Printf("Error updating session tokens: %v", err)
			} else {
				log.Printf("Session tokens updated successfully")
			}
		}

		if err := saveConfig(&newCfg); err != nil {
			log.Printf("Error saving config: %v", err)
		} else {
			log.Printf("Settings saved successfully")
			updateStartupRegistry(newCfg.RunAtStartup)
			reloadConfigTrigger()
			w.Dispatch(func() {
				w.Eval("showToast('Settings saved successfully!')")
			})
		}
	})

	w.Bind("closeWindow", func() {
		w.Dispatch(func() {
			w.Destroy()
		})
	})

	w.Bind("dragWindow", func() {
		pReleaseCapture.Call()
		pSendMessageW.Call(hwnd, uintptr(wmNcLButtonDown), uintptr(htCaption), 0)
	})

	w.Bind("pickFolder", func() string {
		if path, ok := browseForFolder(hwnd); ok {
			return path
		}
		return ""
	})

	initialData, _ := json.Marshal(cfg)

	html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        :root {
            --bg: #ffffff;
            --text: #202020;
            --input-bg: #f9f9f9;
            --input-border: #e0e0e0;
            --accent: #0078d4;
            --accent-hover: #106ebe;
            --button-bg: #efefef;
            --button-text: #202020;
            --header-bg: #ffffff;
        }
        body.dark {
            --bg: #121212;
            --text: #ffffff;
            --input-bg: #1e1e1e;
            --input-border: #333333;
            --button-bg: #2d2d2d;
            --button-text: #ffffff;
            --header-bg: #121212;
        }
        body {
            font-family: 'Segoe UI Variable', 'Segoe UI', sans-serif;
            background-color: var(--bg); 
            color: var(--text);
            margin: 0;
            padding: 0;
            overflow: hidden;
            display: flex;
            flex-direction: column;
            user-select: none;
            height: 100vh;
            border: 1px solid var(--input-border);
            box-sizing: border-box;
        }
        
        .title-bar {
            height: 40px;
            background-color: var(--header-bg);
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 0 0 15px;
            flex-shrink: 0;
            border-bottom: 1px solid var(--input-border);
            -webkit-app-region: drag;
        }
        .title-drag {
            flex: 1;
            height: 100%;
            display: flex;
            align-items: center;
            font-size: 12px;
            font-weight: 600;
            opacity: 0.7;
        }
        .close-btn {
            width: 45px;
            height: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            transition: all 0.2s;
            font-size: 16px;
            font-family: Arial, sans-serif;
        }
        .close-btn:hover { background-color: #e81123; color: white; }

        .content {
            flex: 1;
            overflow-y: auto;
            padding: 20px 25px;
        }
        
        .header-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        h2 { margin: 0; font-size: 18px; font-weight: 600; }

        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-size: 11px; font-weight: 700; opacity: 0.6; text-transform: uppercase; letter-spacing: 0.5px; }
        
        .hotkey-container {
            position: relative;
            display: flex;
            align-items: center;
        }
        .hotkey-hint {
            position: absolute;
            right: 12px;
            font-size: 10px;
            opacity: 0.5;
            pointer-events: none;
        }
        input.recording {
            border-color: #e81123 !important;
            box-shadow: 0 0 0 2px rgba(232, 17, 35, 0.2) !important;
            background-color: rgba(232, 17, 35, 0.05) !important;
        }

        input, textarea {
            width: 100%;
            padding: 8px 12px;
            border: 1px solid var(--input-border);
            background-color: var(--input-bg);
            color: var(--text);
            border-radius: 4px;
            box-sizing: border-box;
            font-size: 14px;
            outline: none;
            transition: all 0.2s;
        }
        input:focus, textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 2px rgba(0,120,212,0.2); }
        textarea { height: 220px; resize: none; font-family: inherit; line-height: 1.4; }
        
        .row { display: flex; gap: 8px; }
        .row input { flex: 1; }
        
        button {
            padding: 8px 16px;
            border: 1px solid var(--input-border);
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            transition: all 0.2s;
            background-color: var(--button-bg);
            color: var(--button-text);
        }
        button:hover { filter: brightness(1.05); }
        button.primary { background-color: var(--accent); color: white; border: none; }
        button.primary:hover { background-color: var(--accent-hover); }
        
        .footer {
            display: flex;
            justify-content: flex-end;
            gap: 10px;
            padding: 15px 25px;
            background-color: var(--header-bg);
            border-top: 1px solid var(--input-border);
        }
        
        .theme-btn {
            background: none;
            border: 1px solid var(--input-border);
            font-size: 12px;
            padding: 6px 14px;
            opacity: 0.9;
            border-radius: 15px;
            cursor: pointer;
            color: var(--text);
            transition: all 0.2s;
        }
        .theme-btn:hover { background: var(--button-bg); }

        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-track { background: transparent; }
        ::-webkit-scrollbar-thumb { background: var(--input-border); border-radius: 10px; }
        ::-webkit-scrollbar-thumb:hover { background: #888; }

        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 12px;
            margin: 8px 0 12px 0;
            padding: 8px 12px;
            background-color: var(--input-bg);
            border: 1px solid var(--input-border);
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .checkbox-group:hover {
            border-color: var(--accent);
            background-color: rgba(0, 120, 212, 0.05);
        }
        .checkbox-group input[type="checkbox"] {
            width: 18px;
            height: 18px;
            margin: 0;
            cursor: pointer;
            accent-color: var(--accent);
        }
        .checkbox-group label {
            margin: 0;
            cursor: pointer;
            text-transform: none;
            font-size: 13px;
            font-weight: 500;
            opacity: 0.9;
            letter-spacing: normal;
            flex: 1;
        }

        /* Toast styling */
        #toast {
            position: fixed;
            bottom: 30px;
            left: 50%;
            transform: translateX(-50%) translateY(20px);
            background-color: #4CAF50;
            color: white;
            padding: 10px 20px;
            border-radius: 20px;
            font-size: 13px;
            font-weight: 600;
            box-shadow: 0 4px 12px rgba(0,0,0,0.3);
            transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
            z-index: 1000;
            opacity: 0;
            pointer-events: none;
        }
        #toast.show {
            transform: translateX(-50%) translateY(0);
            opacity: 1;
        }
    </style>
</head>
<body id="body">
    <div id="toast">Settings saved successfully!</div>
    <div class="title-bar">
        <div class="title-drag" onmousedown="window.dragWindow()">
            ScreanerAi Settings
        </div>
        <div class="close-btn" onclick="window.closeWindow()">X</div>
    </div>

    <div class="content">
        <div class="header-row">
            <h2>Configuration</h2>
            <button onclick="toggleTheme()" id="themeBtn" class="theme-btn">Dark Mode</button>
        </div>

        <div class="form-group">
            <label>Screenshot Hotkey</label>
            <div class="hotkey-container">
                <input type="text" id="hotkey" placeholder="Click to record..." readonly 
                       onclick="startHotkeyRecording('hotkey', 'hotkeyHint')" 
                       onblur="stopHotkeyRecording('hotkey', 'hotkeyHint')">
                <div id="hotkeyHint" class="hotkey-hint">Click to record</div>
            </div>
        </div>

        <div class="form-group">
            <label>Clipboard Monitor Hotkey</label>
            <div class="hotkey-container">
                <input type="text" id="clipboardHotkey" placeholder="Click to record..." readonly 
                       onclick="startHotkeyRecording('clipboardHotkey', 'clipboardHotkeyHint')" 
                       onblur="stopHotkeyRecording('clipboardHotkey', 'clipboardHotkeyHint')">
                <div id="clipboardHotkeyHint" class="hotkey-hint">Click to record</div>
            </div>
        </div>

        <div class="form-group">
            <label>Root Directory</label>
            <div class="row">
                <input type="text" id="rootDir" readonly>
                <button onclick="browseFolder()">Browse</button>
            </div>
        </div>

        <div class="form-group">
            <label>Qwen Server URL</label>
            <input type="text" id="qwenURL">
        </div>

        <div class="form-group">
            <label>AI Model</label>
            <input type="text" id="aiModel">
        </div>

        <div class="form-group">
            <label>Qwen Session Token</label>
            <input type="password" id="qwenToken" placeholder="Paste your token from chat.qwen.ai">
        </div>

        <div class="row">
            <div class="form-group" style="flex: 1;">
                <label>Overlay Opacity (0-255)</label>
                <input type="number" id="overlayOpacity" min="0" max="255">
            </div>
            <div class="form-group" style="flex: 1;">
                <label>Selection Color</label>
                <input type="color" id="selectionColor" style="height: 38px; padding: 2px;">
            </div>
        </div>

        <div class="checkbox-group" onclick="document.getElementById('copyToClipboard').click(); event.stopPropagation();">
            <input type="checkbox" id="copyToClipboard" onclick="event.stopPropagation();">
            <label for="copyToClipboard">Copy screenshot to buffer</label>
        </div>

        <div class="checkbox-group" onclick="document.getElementById('runAtStartup').click(); event.stopPropagation();">
            <input type="checkbox" id="runAtStartup" onclick="event.stopPropagation();">
            <label for="runAtStartup">Run at system startup</label>
        </div>

        <div class="form-group">
            <label>Prompt Template</label>
            <textarea id="prompt"></textarea>
        </div>
    </div>

    <div class="footer">
        <button onclick="window.closeWindow()">Cancel</button>
        <button class="primary" onclick="save()">Save Changes</button>
    </div>

    <script>
        let currentTheme = 'dark';
        let config = ` + string(initialData) + `;
        let isRecording = false;
        let recordingInputId = '';
        let recordingHintId = '';

        function startHotkeyRecording(inputId, hintId) {
            isRecording = true;
            recordingInputId = inputId;
            recordingHintId = hintId;
            
            const input = document.getElementById(inputId);
            const hint = document.getElementById(hintId);
            input.classList.add('recording');
            input.value = "Press combination...";
            hint.innerText = "Recording...";
            
            window.addEventListener('keydown', handleHotkeyInput);
        }

        function stopHotkeyRecording(inputId, hintId) {
            isRecording = false;
            const input = document.getElementById(inputId || recordingInputId);
            const hint = document.getElementById(hintId || recordingHintId);
            if (!input) return;
            
            input.classList.remove('recording');
            if (hint) hint.innerText = "Click to record";
            window.removeEventListener('keydown', handleHotkeyInput);
            
            if (input.value === "Press combination...") {
                if (input.id === 'hotkey') {
                    input.value = config.hotkey || "";
                } else if (input.id === 'clipboardHotkey') {
                    input.value = config.clipboard_hotkey || "";
                }
            }
        }

        function handleHotkeyInput(e) {
            e.preventDefault();
            e.stopPropagation();
            
            if (!isRecording) return;

            const keys = [];
            if (e.ctrlKey) keys.push('ctrl');
            if (e.shiftKey) keys.push('shift');
            if (e.altKey) keys.push('alt');
            if (e.metaKey) keys.push('cmd');

            const ignoredKeys = ['Control', 'Shift', 'Alt', 'Meta', 'CapsLock', 'Tab'];
            if (!ignoredKeys.includes(e.key)) {
                let keyName = e.key.toLowerCase();
                if (keyName === ' ') keyName = 'space';
                keys.push(keyName);
                
                const combination = keys.join('+');
                document.getElementById(recordingInputId).value = combination;
                stopHotkeyRecording(recordingInputId, recordingHintId);
            }
        }

        function init() {
            if (config) {
                document.getElementById('hotkey').value = config.hotkey || "";
                document.getElementById('clipboardHotkey').value = config.clipboard_hotkey || "";
                document.getElementById('rootDir').value = config.root_directory || "";
                document.getElementById('qwenURL').value = config.qwen_server_url || "";
                document.getElementById('aiModel').value = config.ai_model || "";
                document.getElementById('qwenToken').value = config.qwen_token || "";
                document.getElementById('prompt').value = config.prompt_template || "";
                document.getElementById('overlayOpacity').value = config.overlay_opacity || 140;
                document.getElementById('selectionColor').value = config.selection_color || "#00FF00";
                document.getElementById('copyToClipboard').checked = config.copy_to_clipboard || false;
                document.getElementById('runAtStartup').checked = config.run_at_startup || false;
                setTheme(config.theme || 'dark');
            }
        }

        function setTheme(theme) {
            currentTheme = theme;
            const body = document.getElementById('body');
            const btn = document.getElementById('themeBtn');
            if (theme === 'dark') {
                body.classList.add('dark');
                btn.innerText = 'Switch to Light';
            } else {
                body.classList.remove('dark');
                btn.innerText = 'Switch to Dark';
            }
        }

        function toggleTheme() {
            setTheme(currentTheme === 'dark' ? 'light' : 'dark');
        }

        function showToast(message) {
            const toast = document.getElementById('toast');
            toast.innerText = message;
            toast.classList.add('show');
            setTimeout(() => {
                toast.classList.remove('show');
            }, 3000);
        }

        async function browseFolder() {
            const path = await window.pickFolder();
            if (path) {
                document.getElementById('rootDir').value = path;
            }
        }

        function save() {
            const newCfg = {
                hotkey: document.getElementById('hotkey').value,
                clipboard_hotkey: document.getElementById('clipboardHotkey').value,
                root_directory: document.getElementById('rootDir').value,
                qwen_server_url: document.getElementById('qwenURL').value,
                ai_model: document.getElementById('aiModel').value,
                qwen_token: document.getElementById('qwenToken').value,
                prompt_template: document.getElementById('prompt').value,
                overlay_opacity: parseInt(document.getElementById('overlayOpacity').value) || 140,
                selection_color: document.getElementById('selectionColor').value,
                copy_to_clipboard: document.getElementById('copyToClipboard').checked,
                run_at_startup: document.getElementById('runAtStartup').checked,
                theme: currentTheme
            };
            window.saveSettings(JSON.stringify(newCfg));
        }

        init();
    </script>
</body>
</html>
	`

	encodedHTML := base64.StdEncoding.EncodeToString([]byte(html))
	w.Navigate(fmt.Sprintf("data:text/html;base64,%s", encodedHTML))

	w.Run()
}

func updateStartupRegistry(enabled bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		log.Printf("Error opening registry key: %v", err)
		return
	}
	defer k.Close()

	appName := "ScreanerAi"
	if enabled {
		execPath, err := os.Executable()
		if err != nil {
			log.Printf("Error getting executable path: %v", err)
			return
		}

		absPath, err := filepath.Abs(execPath)
		if err != nil {
			log.Printf("Error getting absolute path: %v", err)
			return
		}

		quotedPath := fmt.Sprintf("\"%s\"", absPath)
		err = k.SetStringValue(appName, quotedPath)
		if err != nil {
			log.Printf("Error setting registry value: %v", err)
		} else {
			log.Printf("Added to startup: %s", quotedPath)
		}
	} else {
		err = k.DeleteValue(appName)
		if err != nil && err != registry.ErrNotExist {
			log.Printf("Error deleting registry value: %v", err)
		} else {
			log.Printf("Removed from startup")
		}
	}
}

func browseForFolder(owner uintptr) (string, bool) {
	title, _ := syscall.UTF16PtrFromString("Select Screenshot Root Directory")
	bi := browseInfoW{
		HwndOwner: owner,
		LpszTitle: title,
		UlFlags:   0x00000040,
	}

	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", false
	}

	var path [32768]uint16
	ret, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ret == 0 {
		return "", false
	}

	return syscall.UTF16ToString(path[:]), true
}

type browseInfoW struct {
	HwndOwner      uintptr
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}
