// Command api is the platform's HTTP API gateway and monolith entrypoint.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vpnsaas/platform/internal/admin"
	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/billing"
	"github.com/vpnsaas/platform/internal/cabinet"
	"github.com/vpnsaas/platform/internal/monitoring"
	"github.com/vpnsaas/platform/internal/platform/cache"
	"github.com/vpnsaas/platform/internal/platform/config"
	"github.com/vpnsaas/platform/internal/platform/database"
	"github.com/vpnsaas/platform/internal/platform/health"
	"github.com/vpnsaas/platform/internal/platform/httpx"
	"github.com/vpnsaas/platform/internal/platform/logger"
	"github.com/vpnsaas/platform/internal/platform/observability"
	"github.com/vpnsaas/platform/internal/platform/storage"
	"github.com/vpnsaas/platform/internal/telegram"
	"github.com/vpnsaas/platform/internal/user"
	"github.com/vpnsaas/platform/internal/vpn"
)

// tgNotifier adapts the Telegram bot to the monitoring.Notifier interface.
type tgNotifier struct{ bot *telegram.Bot }

func (n tgNotifier) NotifyMigration(ctx context.Context, telegramID int64, country string) {
	n.bot.NotifyUser(ctx, telegramID,
		"♻️ Ваш VPN ("+country+") переехал на резервный сервер из-за сбоя или блокировки. "+
			"Конфигурация обновлена — откройте «Мои VPN» и переподключитесь.")
}

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

	// Billing module.
	devMode := !cfg.IsProduction()
	providers := map[string]billing.Provider{}
	if devMode {
		providers["mock"] = billing.NewMockProvider(cfg.PublicBaseURL)
	}
	if cfg.CryptomusMerchant != "" && cfg.CryptomusAPIKey != "" {
		providers["cryptomus"] = billing.NewCryptomusProvider(cfg.CryptomusMerchant, cfg.CryptomusAPIKey)
	}
	if cfg.TelegramBotToken != "" {
		providers[billing.GatewayTelegramStars] = billing.NewStarsProvider()
	}
	billingSvc := billing.NewService(billing.NewRepository(db), vpnSvc, providers, cfg.PublicBaseURL, log)
	billingHandler := billing.NewHandler(billingSvc, authMW, devMode)

	// Background: expire overdue subscriptions after the grace period.
	go billing.NewExpirer(db, cfg.GracePeriod, time.Minute, log).Run(ctx)

	// Telegram bot (long polling) — only when a token is configured.
	var migrationNotifier monitoring.Notifier
	if cfg.TelegramBotToken != "" {
		bot := telegram.NewBot(cfg.TelegramBotToken, rdb, authSvc, vpnSvc, billingSvc, cfg.StarsPerUSD, log)
		go bot.Run(ctx)
		migrationNotifier = tgNotifier{bot: bot}
	}

	// Monitoring: poll node health and auto-migrate clients off failed/blocked nodes.
	go monitoring.NewEngine(db, vpnSvc, migrationNotifier, cfg.ProvisionerAgentToken, 30*time.Second, log).Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", checker.Live)
	mux.HandleFunc("GET /ready", checker.Ready)
	mux.Handle("GET /metrics", metrics.Handler())

	authHandler.RegisterRoutes(mux)
	userHandler.RegisterRoutes(mux)
	vpnHandler.RegisterRoutes(mux)
	billingHandler.RegisterRoutes(mux)

	// CMS admin module + static panel at /admin.
	adminHandler := admin.NewHandler(admin.NewRepository(db), authMW)
	adminHandler.RegisterRoutes(mux)
	adminHandler.RegisterPanel(mux)

	// End-user web cabinet at /app.
	cabinet.Register(mux)

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
