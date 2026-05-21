package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/gorilla/websocket"
	_ "github.com/klauspost/compress/zstd"
	_ "github.com/quic-go/quic-go"

	"github.com/omnitun/omnitun/pkg/config"
	omnilog "github.com/omnitun/omnitun/pkg/log"
)

type relayConfig struct {
	RelayID        string
	Region         string
	SiblingRelays  []string
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	relayIDFlag := flag.String("relay-id", "", "unique relay identifier (auto-generated UUID if not provided)")
	regionFlag := flag.String("region", "", "region label (e.g. ap-southeast-1)")
	siblingRelaysFlag := flag.String("sibling-relays", "", "comma-separated list of sibling Relay addresses")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	relayCfg := relayConfig{
		RelayID:       *relayIDFlag,
		Region:        *regionFlag,
		SiblingRelays: parseSiblingRelays(*siblingRelaysFlag),
	}

	if relayCfg.RelayID == "" {
		relayCfg.RelayID = generateRelayUUID()
	}

	if relayCfg.Region == "" {
		relayCfg.Region = "unknown"
	}

	logger := omnilog.New(cfg.LogLevel, "json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = omnilog.WithTraceID(ctx, "relay-bootstrap")
	logger.InfoContext(ctx, "starting omnitun relay node",
		"relay_port", cfg.RelayPort,
		"relay_id", relayCfg.RelayID,
		"region", relayCfg.Region,
		"sibling_relays", relayCfg.SiblingRelays,
	)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		select {
		case sig := <-sigCh:
			logger.InfoContext(ctx, "received signal, shutting down",
				"signal", sig.String(),
			)
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := runRelay(ctx, logger, cfg, &relayCfg); err != nil {
		logger.ErrorContext(ctx, "relay node exited with error", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "relay node shutdown complete")
}

func runRelay(ctx context.Context, logger *slog.Logger, cfg *config.Config, relayCfg *relayConfig) error {
	logger.InfoContext(ctx, "relay services initializing",
		"relay_id", relayCfg.RelayID,
		"region", relayCfg.Region,
	)

	shutdownCh := make(chan error, 2)

	go func() {
		logger.InfoContext(ctx, "quic server starting", "port", cfg.RelayPort)
		if err := serveQUIC(ctx, logger, cfg); err != nil {
			shutdownCh <- fmt.Errorf("quic server error: %w", err)
		}
	}()

	go func() {
		logger.InfoContext(ctx, "websocket server starting", "port", cfg.RelayPort+1)
		if err := serveWebSocket(ctx, logger, cfg); err != nil {
			shutdownCh <- fmt.Errorf("websocket server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.InfoContext(ctx, "context cancelled, stopping relay services")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(),
			30*time.Second)
		defer shutdownCancel()

		select {
		case err := <-shutdownCh:
			if err != nil {
				logger.ErrorContext(ctx, "service shutdown error", "error", err)
			}
		case <-shutdownCtx.Done():
			logger.WarnContext(ctx, "shutdown timed out")
		}

		return nil
	case err := <-shutdownCh:
		return err
	}
}

func serveQUIC(ctx context.Context, logger *slog.Logger, cfg *config.Config) error {
	<-ctx.Done()
	return nil
}

func serveWebSocket(ctx context.Context, logger *slog.Logger, cfg *config.Config) error {
	<-ctx.Done()
	return nil
}

func parseSiblingRelays(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func generateRelayUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
