# Build script for ScreanerAi

$ErrorActionPreference = "Stop"

$BuildDir = "build"
if (Test-Path $BuildDir) {
    Get-ChildItem -Path $BuildDir -Exclude "screener.log" | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
}
if (-not (Test-Path $BuildDir)) {
    New-Item -ItemType Directory -Path $BuildDir | Out-Null
}
if (-not (Test-Path "$BuildDir\session")) {
    New-Item -ItemType Directory -Path "$BuildDir\session" | Out-Null
}

$IconFile = "landscape.ico"
if (Test-Path $IconFile) {
    Write-Host "Generating Windows resources with icon..."
    # Check if rsrc is installed, if not try to install it
    if (-not (Get-Command rsrc -ErrorAction SilentlyContinue)) {
        Write-Host "rsrc not found, installing..."
        go install github.com/akavel/rsrc@latest
    }
    & rsrc -ico $IconFile -o rsrc.syso
}

Write-Host "Building ScreanerAi.exe (Main App + Qwen Server)..."
go build -ldflags="-H windowsgui" -o "$BuildDir\ScreanerAi.exe" .

Write-Host "Building GetQwenToken.exe (Auth Tool)..."
# We need to build from the QwenServer directory or use a relative path
pushd QwenServer
go build -o "..\$BuildDir\GetQwenToken.exe" .\cmd\freeqwen-auth\main.go
popd

Write-Host "Copying configuration files..."
if (Test-Path "config.yaml") {
    Copy-Item "config.yaml" "$BuildDir\"
} else {
    Write-Warning "config.yaml not found, creating a default one."
    @'
hotkey: "ctrl+shift+a"
root_directory: "screenshots"
qwen_server_url: "http://127.0.0.1:3264"
ai_model: "qwen-vl-max"
prompt_template: "Choose the most appropriate folder for this screenshot from the following list: {{.Folders}}"
'@ | Out-File -FilePath "$BuildDir\config.yaml" -Encoding utf8
}

if (Test-Path "QwenServer\Authorization.txt") {
    Copy-Item "QwenServer\Authorization.txt" "$BuildDir\"
} else {
    New-Item -ItemType File -Path "$BuildDir\Authorization.txt" | Out-Null
}

if (Test-Path "README_BUILD.md") {
    Copy-Item "README_BUILD.md" "$BuildDir\README.md"
}

# Create empty tokens.json if not exists
if (-not (Test-Path "$BuildDir\session\tokens.json")) {
    "[]" | Out-File -FilePath "$BuildDir\session\tokens.json" -Encoding utf8
}

Write-Host "Build complete! Output is in the '$BuildDir' folder."
Write-Host "Files generated:"
Get-ChildItem -Path $BuildDir -Recurse | Select-Object FullName
