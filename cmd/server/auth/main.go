package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/omnitun/omnitun/internal/auth"
	"github.com/omnitun/omnitun/pkg/config"
	omnilog "github.com/omnitun/omnitun/pkg/log"
	"github.com/omnitun/omnitun/pkg/metrics"
	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
	"golang.org/x/sync/errgroup"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	grpcPort := flag.Int("grpc-port", 9091, "gRPC server port")
	metricsPort := flag.Int("metrics-port", 9090, "metrics server port")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := omnilog.New(cfg.LogLevel, "json")
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = omnilog.WithTraceID(ctx, "auth-service")
	logger.InfoContext(ctx, "starting OmniTun auth service",
		"grpc_port", *grpcPort,
		"metrics_port", *metricsPort,
	)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := auth.NewRepository(pool)

	jwtMgr, err := auth.NewJWTManager(cfg.Auth)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize JWT manager", "error", err)
		os.Exit(1)
	}

	oauthMgr := auth.NewOAuthManager(cfg, repo, jwtMgr)

	svc := auth.NewService(repo, jwtMgr, oauthMgr)

	grpcServer := grpc.NewServer()
	omnitunv1.RegisterAuthServiceServer(grpcServer, svc)
	reflection.Register(grpcServer)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
		if err != nil {
			return fmt.Errorf("failed to listen on gRPC port: %w", err)
		}
		logger.InfoContext(ctx, "gRPC server listening", "addr", lis.Addr().String())
		go func() {
			<-ctx.Done()
			logger.InfoContext(ctx, "shutting down gRPC server")
			grpcServer.GracefulStop()
		}()
		return grpcServer.Serve(lis)
	})

	g.Go(func() error {
		return metrics.Serve(fmt.Sprintf(":%d", *metricsPort))
	})

	g.Go(func() error {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		select {
		case sig := <-sigCh:
			logger.InfoContext(ctx, "received signal, initiating graceful shutdown",
				"signal", sig.String(),
			)
			cancel()
			return nil
		case <-ctx.Done():
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		logger.ErrorContext(ctx, "auth service exited with error", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "auth service shutdown complete")
}
