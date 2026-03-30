package main

import (
	"bufio"
	"flag"
	"fmt"
	"mintahahahahah/freeqwenproxy"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	var (
		idFlag        = flag.String("id", "", "id аккаунта (по умолчанию acc_<timestamp>)")
		tokensPath    = flag.String("tokens", filepath.Join("session", "tokens.json"), "путь к tokens.json")
		accountsDir   = flag.String("accounts-dir", filepath.Join("session", "accounts"), "директория accounts")
		urlFlag       = flag.String("url", "https://chat.qwen.ai/", "страница Qwen для логина")
		tokenFlag     = flag.String("token", "", "токен (если не указан — будет запрошен через stdin)")
	)
	flag.Parse()

	id := strings.TrimSpace(*idFlag)
	if id == "" {
		id = fmt.Sprintf("acc_%d", time.Now().UnixMilli())
	}

	accDir := filepath.Join(*accountsDir, id)
	if err := os.MkdirAll(accDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir accounts dir failed:", err)
		os.Exit(1)
	}

	fmt.Printf("Account: %s\n", id)
	fmt.Printf("URL: %s\n", *urlFlag)

	if strings.TrimSpace(*tokenFlag) == "" {
		_ = openBrowser(*urlFlag)
		fmt.Print("Откройте DevTools Console на странице Qwen и выполните:\n")
		fmt.Print("  localStorage.getItem('token')\n")
		fmt.Print("Скопируйте значение и вставьте сюда (Enter):\n")
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(readLineFromStdin())
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Fprintln(os.Stderr, "empty token")
		os.Exit(1)
	}

	if err := os.WriteFile(filepath.Join(accDir, "token.txt"), []byte(token), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "write token.txt failed:", err)
		os.Exit(1)
	}

	tm := freeqwenproxy.NewTokenManager(*tokensPath)
	if err := tm.AddOrUpdate(id, token); err != nil {
		fmt.Fprintln(os.Stderr, "update tokens.json failed:", err)
		os.Exit(1)
	}

	fmt.Printf("OK: токен сохранён в %s и %s\n", filepath.Join(accDir, "token.txt"), *tokensPath)
	fmt.Printf("Token: %s\n", maskToken(token))
}

func readLineFromStdin() string {
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func openBrowser(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func maskToken(token string) string {
	t := strings.TrimSpace(token)
	if len(t) <= 12 {
		return t
	}
	return t[:6] + "..." + t[len(t)-4:]
}
