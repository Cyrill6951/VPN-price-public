package telegram

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/billing"
	"github.com/vpnsaas/platform/internal/vpn"
)

// Bot drives the Telegram purchase and management flows over long polling.
type Bot struct {
	api         *Client
	redis       *redis.Client
	auth        *auth.Service
	vpn         *vpn.Service
	billing     *billing.Service
	starsPerUSD float64
	log         *slog.Logger
}

// NewBot wires the bot with the platform services.
func NewBot(token string, rdb *redis.Client, authSvc *auth.Service, vpnSvc *vpn.Service, billingSvc *billing.Service, starsPerUSD float64, log *slog.Logger) *Bot {
	if starsPerUSD <= 0 {
		starsPerUSD = 1
	}
	return &Bot{
		api:         NewClient(token),
		redis:       rdb,
		auth:        authSvc,
		vpn:         vpnSvc,
		billing:     billingSvc,
		starsPerUSD: starsPerUSD,
		log:         log,
	}
}

// Run polls Telegram for updates until the context is cancelled.
func (b *Bot) Run(ctx context.Context) {
	b.log.Info("telegram bot started (long polling)")
	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.api.GetUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Warn("getUpdates failed", "error", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			b.dispatch(ctx, u)
		}
	}
}

func (b *Bot) dispatch(ctx context.Context, u Update) {
	defer func() {
		if rec := recover(); rec != nil {
			b.log.Error("panic in update handler", "error", rec)
		}
	}()

	switch {
	case u.PreCheckoutQuery != nil:
		// Approve all pre-checkouts; the order was validated at invoice time.
		if err := b.api.AnswerPreCheckoutQuery(ctx, u.PreCheckoutQuery.ID, true, ""); err != nil {
			b.log.Warn("answer precheckout failed", "error", err)
		}
	case u.Message != nil && u.Message.SuccessfulPayment != nil:
		b.handlePayment(ctx, u.Message)
	case u.Message != nil && strings.HasPrefix(u.Message.Text, "/"):
		b.handleCommand(ctx, u.Message)
	case u.CallbackQuery != nil:
		b.handleCallback(ctx, u.CallbackQuery)
	}
}

// ensureUser links (or creates) the platform account for a Telegram user.
func (b *Bot) ensureUser(ctx context.Context, from *User) (auth.User, error) {
	return b.auth.EnsureTelegramUser(ctx, from.ID, from.FirstName, from.LastName, from.Username)
}

// stars converts a USD amount into an integer number of Telegram Stars.
func (b *Bot) stars(usd float64) int {
	n := int(usd*b.starsPerUSD + 0.5)
	if n < 1 {
		n = 1
	}
	return n
}
