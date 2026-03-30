package freeqwenproxy

import (
	"path/filepath"
)

type Config struct {
	Addr string

	QwenBaseURL string

	UpstreamBaseURL string
	UpstreamAPIKey  string

	ModelsFilePath  string
	AuthKeysPath    string
	TokensFilePath  string
	UploadsDirPath  string
	ChunkRuneSize   int
	ChunkDelayMilli int
}

func DefaultConfig() Config {
	root := "."
	return Config{
		Addr:            "127.0.0.1:3264",
		QwenBaseURL:     "https://chat.qwen.ai",
		UpstreamBaseURL: "",
		UpstreamAPIKey:  "",
		ModelsFilePath:  filepath.Join(root, "AvaibleModels.txt"),
		AuthKeysPath:    filepath.Join(root, "Authorization.txt"),
		TokensFilePath:  filepath.Join(root, "session", "tokens.json"),
		UploadsDirPath:  filepath.Join(root, "uploads"),
		ChunkRuneSize:   16,
		ChunkDelayMilli: 20,
	}
}
