// Command api is the platform's HTTP API gateway and monolith entrypoint.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/platform/cache"
	"github.com/vpnsaas/platform/internal/platform/config"
	"github.com/vpnsaas/platform/internal/platform/database"
	"github.com/vpnsaas/platform/internal/platform/health"
	"github.com/vpnsaas/platform/internal/platform/httpx"
	"github.com/vpnsaas/platform/internal/platform/logger"
	"github.com/vpnsaas/platform/internal/platform/observability"
	"github.com/vpnsaas/platform/internal/platform/storage"
	"github.com/vpnsaas/platform/internal/user"
	"github.com/vpnsaas/platform/internal/vpn"
)

func main() {
	if err := run(); err != nil {
		// Logger may not be up yet; stderr is the safe fallback.
		os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.LogLevel, cfg.Env)
	log.Info("starting api", "env", cfg.Env)

	// Cancel everything on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Info("connected to postgres")

	rdb, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	log.Info("connected to redis")

	metrics := observability.NewMetrics()
	checker := health.New(db, rdb)

	// Auth module.
	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, tokenMgr, rdb, cfg.TelegramBotToken)
	authMW := auth.NewMiddleware(authSvc)
	authHandler := auth.NewHandler(authSvc, authMW)

	// User module.
	userRepo := user.NewRepository(db)
	userHandler := user.NewHandler(userRepo, authMW)

	// VPN module.
	objStore, err := storage.New(ctx, storage.Config{
		Endpoint:  cfg.MinioEndpoint,
		AccessKey: cfg.MinioAccessKey,
		SecretKey: cfg.MinioSecretKey,
		Bucket:    cfg.MinioBucket,
		UseSSL:    cfg.MinioUseSSL,
	})
	if err != nil {
		return err
	}
	log.Info("connected to object storage", "bucket", cfg.MinioBucket)

	vpnCipher, err := vpn.NewCipher(cfg.VPNConfigKeyHex)
	if err != nil {
		return err
	}
	provisioner := vpn.NewProvisioner(cfg.ProvisionerMode, log, cfg.ProvisionerAgentToken)
	vpnSvc := vpn.NewService(vpn.NewRepository(db), objStore, vpnCipher, provisioner, log)
	vpnHandler := vpn.NewHandler(vpnSvc, authMW)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", checker.Live)
	mux.HandleFunc("GET /ready", checker.Ready)
	mux.Handle("GET /metrics", metrics.Handler())

	authHandler.RegisterRoutes(mux)
	userHandler.RegisterRoutes(mux)
	vpnHandler.RegisterRoutes(mux)

	handler := httpx.Chain(mux,
		httpx.RequestID,
		httpx.Recover(log),
		httpx.Observe(log, metrics),
	)

	srv := httpx.NewServer(cfg.HTTPAddr, handler, log)
	if err := srv.Start(ctx, cfg.ShutdownTimeout); err != nil {
		return err
	}

	log.Info("shutdown complete")
	return nil
}
