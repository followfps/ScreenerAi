package freeqwenproxy

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func loadNonCommentLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 128*1024)
	sc.Buffer(buf, 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ensureAuthKeysFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}

	const template = `# Файл API-ключей для прокси
# --------------------------------------------
# В этом файле перечислены токены, которые
# прокси будет считать «действительными».
# Один ключ — одна строка без пробелов.
#
# 1) Хотите ОТКЛЮЧИТЬ авторизацию целиком?
#    Оставьте файл пустым — сервер перестанет
#    проверять заголовок Authorization.
#
# 2) Хотите разрешить доступ нескольким людям?
#    Впишите каждый ключ в отдельной строке:
#      d35ab3e1-a6f9-4d...
#      f2b1cd9c-1b2e-4a...
#
# Пустые строки и строки, начинающиеся с «#»,
# игнорируются.
`
	return os.WriteFile(path, []byte(template), 0o644)
}

func loadModels(path string) []string {
	lines, err := loadNonCommentLines(path)
	if err != nil || len(lines) == 0 {
		return []string{"qwen-max-latest"}
	}
	return lines
}

func loadAuthKeys(path string) []string {
	_ = ensureAuthKeysFile(path)
	lines, err := loadNonCommentLines(path)
	if err != nil {
		return nil
	}
	return lines
}

