package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/billing"
	"github.com/vpnsaas/platform/internal/vpn"
)

// handleCommand routes slash commands.
func (b *Bot) handleCommand(ctx context.Context, msg *Message) {
	user, err := b.ensureUser(ctx, msg.From)
	if err != nil {
		b.reply(ctx, msg.Chat.ID, textError, nil)
		return
	}
	cmd := strings.Fields(msg.Text)[0]
	switch cmd {
	case "/myvpn":
		b.showMyVPN(ctx, msg.Chat.ID, user.ID)
	case "/faq":
		b.reply(ctx, msg.Chat.ID, textFAQ, withBack())
	case "/buy":
		b.showCountries(ctx, msg.Chat.ID, 0)
	default: // /start, /help, anything else
		b.reply(ctx, msg.Chat.ID, textWelcome, mainMenuKeyboard())
	}
}

// handleCallback routes inline-button presses.
func (b *Bot) handleCallback(ctx context.Context, cq *CallbackQuery) {
	_ = b.api.AnswerCallbackQuery(ctx, cq.ID, "")
	if cq.Message == nil {
		return
	}
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID

	user, err := b.ensureUser(ctx, &cq.From)
	if err != nil {
		b.reply(ctx, chatID, textError, nil)
		return
	}
	data := cq.Data

	switch {
	case data == "menu":
		b.editKB(ctx, chatID, msgID, textWelcome, mainMenuKeyboard())
	case data == "buy":
		b.clearDraft(ctx, chatID)
		b.showCountries(ctx, chatID, msgID)
	case data == "faq":
		b.editKB(ctx, chatID, msgID, textFAQ, withBack())
	case data == "myvpn":
		b.editMyVPN(ctx, chatID, msgID, user.ID)
	case strings.HasPrefix(data, "country:"):
		d, _ := b.getDraft(ctx, chatID)
		d.CountryID = strings.TrimPrefix(data, "country:")
		d.RenewSubID = ""
		_ = b.saveDraft(ctx, chatID, d)
		b.editKB(ctx, chatID, msgID, textChooseProtocol, protocolsKeyboard())
	case strings.HasPrefix(data, "proto:"):
		d, _ := b.getDraft(ctx, chatID)
		switch strings.TrimPrefix(data, "proto:") {
		case "wireguard_split":
			d.Protocol = "wireguard"
			d.Routing = string(vpn.RoutingSplitRU)
		default:
			d.Protocol = strings.TrimPrefix(data, "proto:")
			d.Routing = string(vpn.RoutingFull)
		}
		_ = b.saveDraft(ctx, chatID, d)
		b.editPlans(ctx, chatID, msgID)
	case strings.HasPrefix(data, "plan:"):
		b.selectPlan(ctx, chatID, msgID, strings.TrimPrefix(data, "plan:"))
	case data == "pay":
		b.pay(ctx, chatID, user)
	case strings.HasPrefix(data, "renew:"):
		b.startRenew(ctx, chatID, msgID, user.ID, strings.TrimPrefix(data, "renew:"))
	case strings.HasPrefix(data, "cfg:"):
		b.sendConfig(ctx, chatID, user.ID, strings.TrimPrefix(data, "cfg:"))
	}
}

func (b *Bot) showCountries(ctx context.Context, chatID, msgID int64) {
	countries, err := b.vpn.Countries(ctx)
	if err != nil || len(countries) == 0 {
		b.reply(ctx, chatID, textNoServers, withBack())
		return
	}
	b.editOrSend(ctx, chatID, msgID, textChooseCountry, countriesKeyboard(countries))
}

func (b *Bot) editPlans(ctx context.Context, chatID, msgID int64) {
	plans, err := b.vpn.Plans(ctx)
	if err != nil || len(plans) == 0 {
		b.reply(ctx, chatID, textError, withBack())
		return
	}
	b.editKB(ctx, chatID, msgID, textChoosePlan, plansKeyboard(plans))
}

func (b *Bot) selectPlan(ctx context.Context, chatID, msgID int64, planID string) {
	d, _ := b.getDraft(ctx, chatID)
	d.PlanID = planID
	_ = b.saveDraft(ctx, chatID, d)

	plan, err := b.findPlan(ctx, planID)
	if err != nil {
		b.editKB(ctx, chatID, msgID, textError, withBack())
		return
	}
	stars := b.stars(plan.Price)
	summary := fmt.Sprintf("🧾 <b>Подтверждение</b>\n\nТариф: <b>%s</b>\nСрок: %d дней\nЦена: $%.2f (⭐ %d)\n\n%s",
		plan.Name, plan.Days, plan.Price, stars, textPayClue)
	b.editKB(ctx, chatID, msgID, summary, confirmKeyboard(stars))
}

func (b *Bot) startRenew(ctx context.Context, chatID, msgID int64, userID uuid.UUID, subID string) {
	views, err := b.vpn.List(ctx, userID)
	if err != nil {
		b.editKB(ctx, chatID, msgID, textError, withBack())
		return
	}
	var target *vpn.VPNView
	for i := range views {
		if views[i].Subscription.ID.String() == subID {
			target = &views[i]
			break
		}
	}
	if target == nil {
		b.editKB(ctx, chatID, msgID, textError, withBack())
		return
	}
	d := draft{
		RenewSubID: subID,
		CountryID:  target.Subscription.CountryID.String(),
		Protocol:   string(target.Subscription.Protocol),
	}
	_ = b.saveDraft(ctx, chatID, d)
	b.editPlans(ctx, chatID, msgID)
}

func (b *Bot) pay(ctx context.Context, chatID int64, user auth.User) {
	d, _ := b.getDraft(ctx, chatID)
	planID, err1 := uuid.Parse(d.PlanID)
	countryID, err2 := uuid.Parse(d.CountryID)
	if err1 != nil || err2 != nil || d.Protocol == "" {
		b.reply(ctx, chatID, textError, mainMenuKeyboard())
		return
	}
	in := billing.BuyInput{
		UserID:    user.ID,
		PlanID:    planID,
		CountryID: countryID,
		Protocol:  vpn.Protocol(d.Protocol),
		Gateway:   billing.GatewayTelegramStars,
		Routing:   vpn.RoutingMode(d.Routing),
	}
	if d.RenewSubID != "" {
		if sub, err := uuid.Parse(d.RenewSubID); err == nil {
			in.RenewSubscriptionID = &sub
		}
	}

	order, _, err := b.billing.Buy(ctx, in)
	if err != nil {
		b.log.Warn("bot buy failed", "error", err)
		b.reply(ctx, chatID, textError, mainMenuKeyboard())
		return
	}
	stars := b.stars(order.Amount)
	title := "VPN подписка"
	if d.RenewSubID != "" {
		title = "Продление VPN"
	}
	if err := b.api.SendInvoice(ctx, chatID, title, "Доступ к VPN-сервису", order.ID.String(),
		[]LabeledPrice{{Label: title, Amount: stars}}); err != nil {
		b.log.Warn("send invoice failed", "error", err)
		b.reply(ctx, chatID, textError, mainMenuKeyboard())
	}
}

// handlePayment finalizes the order after a successful Stars payment.
func (b *Bot) handlePayment(ctx context.Context, msg *Message) {
	user, err := b.ensureUser(ctx, msg.From)
	if err != nil {
		return
	}
	orderID, err := uuid.Parse(msg.SuccessfulPayment.InvoicePayload)
	if err != nil {
		return
	}
	subID, err := b.billing.FulfillOrder(ctx, orderID)
	if err != nil {
		b.log.Warn("fulfill order failed", "error", err, "order", orderID)
		b.reply(ctx, msg.Chat.ID, textError, mainMenuKeyboard())
		return
	}
	b.clearDraft(ctx, msg.Chat.ID)
	b.reply(ctx, msg.Chat.ID, textPaymentOK, nil)
	if subID != uuid.Nil {
		b.sendConfig(ctx, msg.Chat.ID, user.ID, subID.String())
	}
}

func (b *Bot) sendConfig(ctx context.Context, chatID int64, userID uuid.UUID, subIDStr string) {
	subID, err := uuid.Parse(subIDStr)
	if err != nil {
		b.reply(ctx, chatID, textError, nil)
		return
	}
	data, _, err := b.vpn.FetchConfig(ctx, userID, subID)
	if err != nil {
		b.reply(ctx, chatID, textError, mainMenuKeyboard())
		return
	}
	qr, qrErr := b.vpn.FetchQR(ctx, userID, subID)

	if strings.HasPrefix(string(data), "[Interface]") {
		_ = b.api.SendDocument(ctx, chatID, "vpn.conf", data, "🔐 WireGuard конфигурация")
	} else {
		_ = b.api.SendMessage(ctx, chatID,
			"🔑 <b>Ссылка для импорта</b>:\n<code>"+html.EscapeString(string(data))+"</code>", nil)
	}
	if qrErr == nil {
		_ = b.api.SendPhoto(ctx, chatID, "qr.png", qr, "📷 QR для быстрого импорта")
	}
	b.reply(ctx, chatID, "Готово! Управление — в меню.", mainMenuKeyboard())
}

// --- view helpers ---

func (b *Bot) showMyVPN(ctx context.Context, chatID int64, userID uuid.UUID) {
	views, err := b.vpn.List(ctx, userID)
	if err != nil {
		b.reply(ctx, chatID, textError, mainMenuKeyboard())
		return
	}
	if len(views) == 0 {
		b.reply(ctx, chatID, textNoVPNs, mainMenuKeyboard())
		return
	}
	b.reply(ctx, chatID, b.vpnListText(views), myVPNKeyboard(views))
}

func (b *Bot) editMyVPN(ctx context.Context, chatID, msgID int64, userID uuid.UUID) {
	views, err := b.vpn.List(ctx, userID)
	if err != nil {
		b.editKB(ctx, chatID, msgID, textError, mainMenuKeyboard())
		return
	}
	if len(views) == 0 {
		b.editKB(ctx, chatID, msgID, textNoVPNs, mainMenuKeyboard())
		return
	}
	b.editKB(ctx, chatID, msgID, b.vpnListText(views), myVPNKeyboard(views))
}

func (b *Bot) vpnListText(views []vpn.VPNView) string {
	var sb strings.Builder
	sb.WriteString("📄 <b>Ваши VPN</b>\n")
	for _, v := range views {
		sb.WriteString(fmt.Sprintf("\n• %s · %s · до %s",
			v.Country, v.Subscription.Protocol, v.Subscription.ExpiresAt.Format("2006-01-02")))
	}
	return sb.String()
}

func (b *Bot) findPlan(ctx context.Context, planID string) (vpn.PlanRef, error) {
	plans, err := b.vpn.Plans(ctx)
	if err != nil {
		return vpn.PlanRef{}, err
	}
	for _, p := range plans {
		if p.ID.String() == planID {
			return p, nil
		}
	}
	return vpn.PlanRef{}, fmt.Errorf("plan not found")
}

// --- send helpers ---

func (b *Bot) reply(ctx context.Context, chatID int64, text string, kb *InlineKeyboardMarkup) {
	if err := b.api.SendMessage(ctx, chatID, text, kb); err != nil {
		b.log.Warn("send message failed", "error", err)
	}
}

func (b *Bot) editKB(ctx context.Context, chatID, msgID int64, text string, kb *InlineKeyboardMarkup) {
	if err := b.api.EditMessageText(ctx, chatID, msgID, text, kb); err != nil {
		// Fall back to a fresh message if the edit fails (e.g. content unchanged).
		b.reply(ctx, chatID, text, kb)
	}
}

func (b *Bot) editOrSend(ctx context.Context, chatID, msgID int64, text string, kb *InlineKeyboardMarkup) {
	if msgID == 0 {
		b.reply(ctx, chatID, text, kb)
		return
	}
	b.editKB(ctx, chatID, msgID, text, kb)
}

func withBack() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{backToMenuRow()}}
}
