package freeqwenproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TokenEntry struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	ResetAt string `json:"resetAt,omitempty"`
	Invalid bool   `json:"invalid,omitempty"`
}

type TokenManager struct {
	path string
	mu   sync.Mutex
	ptr  int
}

func NewTokenManager(path string) *TokenManager {
	return &TokenManager{path: path}
}

func (m *TokenManager) ensureSessionDir() error {
	dir := filepath.Dir(m.path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (m *TokenManager) loadTokensUnlocked() ([]TokenEntry, error) {
	if err := m.ensureSessionDir(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}

	content := string(b)
	content = strings.TrimPrefix(content, "\uFEFF")
	b = []byte(content)

	var tokens []TokenEntry
	if err := json.Unmarshal(b, &tokens); err != nil {
		var obj map[string]any
		if err2 := json.Unmarshal(b, &obj); err2 == nil && len(obj) == 0 {
			tokens = []TokenEntry{}
		} else {
			return nil, err
		}
	}
	return tokens, nil
}

func (m *TokenManager) saveTokensUnlocked(tokens []TokenEntry) error {
	if err := m.ensureSessionDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, b, 0o644)
}

func (m *TokenManager) ListTokens() ([]TokenEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadTokensUnlocked()
}

func (m *TokenManager) hasValidTokenUnlocked(tokens []TokenEntry, now time.Time) bool {
	for _, t := range tokens {
		if t.Invalid {
			continue
		}
		if t.Token == "" {
			continue
		}
		if t.ResetAt == "" {
			return true
		}
		if resetAt, err := time.Parse(time.RFC3339, t.ResetAt); err == nil && !resetAt.After(now) {
			return true
		}
	}
	return false
}

func (m *TokenManager) HasValidTokens() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return false, err
	}
	return m.hasValidTokenUnlocked(tokens, time.Now()), nil
}

func (m *TokenManager) GetAvailableToken() (*TokenEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return nil, err
	}
	now := time.Now()

	var valid []TokenEntry
	for _, t := range tokens {
		if t.Invalid || t.Token == "" {
			continue
		}
		if t.ResetAt != "" {
			resetAt, err := time.Parse(time.RFC3339, t.ResetAt)
			if err == nil && resetAt.After(now) {
				continue
			}
		}
		valid = append(valid, t)
	}
	if len(valid) == 0 {
		return nil, nil
	}
	idx := m.ptr % len(valid)
	m.ptr = (m.ptr + 1) % len(valid)

	out := valid[idx]
	return &out, nil
}

func (m *TokenManager) MarkRateLimited(id string, hours int) error {
	if hours <= 0 {
		hours = 24
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return err
	}
	for i := range tokens {
		if tokens[i].ID == id {
			until := time.Now().Add(time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
			tokens[i].ResetAt = until
			return m.saveTokensUnlocked(tokens)
		}
	}
	return nil
}

func (m *TokenManager) MarkInvalid(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return err
	}
	for i := range tokens {
		if tokens[i].ID == id {
			tokens[i].Invalid = true
			return m.saveTokensUnlocked(tokens)
		}
	}
	return nil
}

func (m *TokenManager) MarkValid(id string, newToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return err
	}
	for i := range tokens {
		if tokens[i].ID == id {
			tokens[i].Invalid = false
			tokens[i].ResetAt = ""
			if newToken != "" {
				tokens[i].Token = newToken
			}
			return m.saveTokensUnlocked(tokens)
		}
	}
	return nil
}

func (m *TokenManager) AddOrUpdate(id string, token string) error {
	id = strings.TrimSpace(id)
	token = strings.TrimSpace(token)
	if id == "" || token == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tokens, err := m.loadTokensUnlocked()
	if err != nil {
		return err
	}

	for i := range tokens {
		if tokens[i].ID == id {
			tokens[i].Token = token
			tokens[i].Invalid = false
			tokens[i].ResetAt = ""
			return m.saveTokensUnlocked(tokens)
		}
	}

	tokens = append(tokens, TokenEntry{ID: id, Token: token})
	return m.saveTokensUnlocked(tokens)
}
