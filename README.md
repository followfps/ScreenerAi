# ScreenerAi — Intelligent Screenshot Organizer 📸🤖

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-Windows-blue.svg)
![Go](https://img.shields.io/badge/go-1.21%2B-blue.svg)

**ScreanerAi** is a modern Windows application that transforms screenshot chaos into a structured library. Using the power of the **Qwen-VL** multimodal neural network, the app automatically analyzes the content of your screen captures and saves them into the most relevant thematic folders.

---

## ✨ Key Features

- 🧠 **Smart Classification**: The AI "sees" the content of the screenshot and distributes it among your folders (Work, Code, Design, Games, etc.).
- 🖼️ **Area Capture**: A convenient selection tool for any part of the screen (similar to "Snipping Tool", but smarter).
- 📋 **Clipboard Monitoring**: Automatic processing of images copied to the clipboard from other applications.
- ⚙️ **Flexible Configuration**: 
  - Record custom global hotkeys.
  - Adjust selection area opacity and color.
  - Dark and Light theme support (Fluent Design).
- 🚀 **Auto-startup**: Option to launch with Windows for instant access.
- 🔔 **Native Notifications**: Information about where the screenshot was saved, right in the Windows notification center.

---

## 🛠 Tech Stack

- **Language**: [Go (Golang)](https://go.dev/)
- **UI**: [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (HTML/JS/CSS for the settings window)
- **Graphics**: Win32 GDI API (for the capture overlay)
- **AI**: Integration with Qwen-VL via a local proxy server.

---

## 🚀 Quick Start

### Build from Source
You will need Go 1.21 or higher installed.

1. Clone the repository:
   ```bash
   git clone https://github.com/your-username/ScreanerAi.git
   cd ScreanerAi
   ```
2. Run the build script in PowerShell:
   ```powershell
   .\build.ps1
   ```
3. The ready files will be located in the `build/` directory.

### AI Configuration
To enable the AI, you need to add an authorization token:
1. Go to [chat.qwen.ai](https://chat.qwen.ai/) and log in.
2. Open developer tools (`F12`), go to the **Application** tab -> **Local Storage**.
3. Find the `token` key and copy its value.
4. In ScreanerAi, open **Settings** (right-click the tray icon) and paste the token into the **Qwen Session Token** field.

---

## 🔍 How It Works?

1. **Trigger**: You press a hotkey (default `Ctrl+Shift+S`) or copy an image to the buffer.
2. **Capture**: The `selector_windows.go` module allows you to select an area.
3. **Analysis**: The screenshot is sent to the built-in `QwenServer`.
4. **Decision**: The neural network receives a list of your subfolders from the **Root Directory** and chooses the best one.
5. **Saving**: The file is saved with a timestamp: `RootDirectory/Category/2024-01-01_12-00-00.png`.

---

## 🤝 Acknowledgments and Dependencies

This project was made possible thanks to the following libraries:
- **systray**: For system tray integration.
- **webview2**: For the modern settings interface.
- **hotkey**: For global hotkey support.
- **screenshot**: For screen capture capabilities.

### Special Thanks
The project uses components from the **freeQwen** library (integrated into `QwenServer`) to provide seamless interaction with the Qwen neural network API. Without this module, implementing seamless classification would be significantly more difficult.
https://github.com/y13sint/FreeQwenApi

---

## 📄 License

This project is distributed under the MIT License. See the [LICENSE](LICENSE) file for details.
