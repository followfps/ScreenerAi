package main

import (
	"context"
	"github.com/followfps/ScreanerAi/QwenServer/freeqwenproxy"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	cfg := freeqwenproxy.DefaultConfig()

	host := strings.TrimSpace(os.Getenv("HOST"))
	if host == "" {
		host = "0.0.0.0"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, _, _ = srv.ListenAndServe(ctx)
	<-ctx.Done()
}

