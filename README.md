# Screenshot Organizer with Qwen Vision AI

A Go desktop application that captures a screenshot on a global hotkey press and uses the local **QwenServer** (Qwen Vision AI proxy) to automatically sort it into the most relevant folder.

## How It Works

1. **Press the hotkey** (default: `Ctrl+Shift+S`) — the app captures your primary monitor
2. **Image uploaded to QwenServer** — via `/api/files/upload`
3. **AI analyzes the screenshot** — sends the image to Qwen Vision model along with your folder names
4. **Saves to the right folder** — the screenshot is saved with a timestamped filename in the AI-chosen directory

## Prerequisites

- **Go 1.21+** — [Download Go](https://go.dev/dl/)
- **QwenServer** running locally (from the `QwenServer` folder)
- **Windows** (primary target; also works on macOS/Linux)

## Setup

### 1. Start QwenServer

```bash
cd QwenServer
./qwen_server.exe
```

The server runs at `http://localhost:3264` by default.

### 2. Configure

Edit `config.yaml`:

```yaml
hotkey: "ctrl+shift+s"
root_directory: "C:\\screanshots"
qwen_server_url: "http://localhost:3264"
qwen_api_key: ""          # leave empty if no auth
ai_model: "qwen-vl-max"   # vision model
```

Create subdirectories inside your `root_directory`:
```
Screenshots/
├── Work/
├── Gaming/
├── Code/
├── Social/
└── Other/
```

### 3. Build & Run

```bash
go mod tidy
go build -o screener.exe .
./screener.exe
```

Press the configured hotkey to capture and organize a screenshot. Press `Ctrl+C` to stop.

## Configuration Options

| Option            | Description                                           | Default                  |
|-------------------|-------------------------------------------------------|--------------------------|
| `hotkey`          | Global shortcut (format: `modifier+modifier+key`)     | `ctrl+shift+s`           |
| `root_directory`  | Path containing subdirectories for sorting            | —                        |
| `qwen_server_url` | QwenServer proxy URL                                  | `http://localhost:3264`  |
| `qwen_api_key`    | API key (if `Authorization.txt` has keys)             | empty                    |
| `ai_model`        | Qwen vision model                                     | `qwen-vl-max`            |
| `prompt_template` | Instruction sent to AI (`{folders}` = placeholder)    | Built-in default         |

### Available Vision Models

| Model | Description |
|-------|-------------|
| `qwen-vl-max` | Best quality vision |
| `qwen3-vl-plus` | Qwen3 vision |
| `qvq-72b-preview-0310` | QVQ reasoning + vision |
| `qwen2.5-vl-32b-instruct` | Qwen2.5 VL |

## Notes

- No CGO required on Windows
- No external API keys needed — uses your local QwenServer
- Screenshots saved as PNG with timestamp filenames (e.g., `2024-01-15_14-30-45.png`)
- If the AI returns an invalid folder name, the first folder is used as a fallback
