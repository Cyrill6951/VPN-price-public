package telegram

import (
	"fmt"

	"github.com/vpnsaas/platform/internal/vpn"
)

func mainMenuKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "🛒 Купить VPN", CallbackData: "buy"}},
		{{Text: "📄 Мои VPN", CallbackData: "myvpn"}},
		{{Text: "❓ FAQ", CallbackData: "faq"}},
	}}
}

func backToMenuRow() []InlineKeyboardButton {
	return []InlineKeyboardButton{{Text: "⬅️ В меню", CallbackData: "menu"}}
}

func countriesKeyboard(countries []vpn.CountryRef) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(countries)+1)
	for _, c := range countries {
		flag := ""
		if c.Flag != nil {
			flag = *c.Flag + " "
		}
		rows = append(rows, []InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s%s", flag, c.Name),
			CallbackData: "country:" + c.ID.String(),
		}})
	}
	rows = append(rows, backToMenuRow())
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func protocolsKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "WireGuard (весь трафик)", CallbackData: "proto:wireguard"}},
		{{Text: "WireGuard 🇷🇺 (РФ-ресурсы напрямую)", CallbackData: "proto:wireguard_split"}},
		{{Text: "VLESS · Reality", CallbackData: "proto:vless_reality"}},
		{{Text: "Shadowsocks", CallbackData: "proto:shadowsocks"}},
		backToMenuRow(),
	}}
}

func plansKeyboard(plans []vpn.PlanRef) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(plans)+1)
	for _, p := range plans {
		label := fmt.Sprintf("%s — %d дн. · $%.2f", p.Name, p.Days, p.Price)
		rows = append(rows, []InlineKeyboardButton{{
			Text:         label,
			CallbackData: "plan:" + p.ID.String(),
		}})
	}
	rows = append(rows, backToMenuRow())
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func confirmKeyboard(stars int) *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: fmt.Sprintf("💳 Оплатить ⭐ %d", stars), CallbackData: "pay"}},
		backToMenuRow(),
	}}
}

func myVPNKeyboard(views []vpn.VPNView) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(views)*2+1)
	for _, v := range views {
		id := v.Subscription.ID.String()
		rows = append(rows, []InlineKeyboardButton{
			{Text: "📥 " + v.Country + " · конфиг", CallbackData: "cfg:" + id},
			{Text: "🔄 Продлить", CallbackData: "renew:" + id},
		})
	}
	rows = append(rows, backToMenuRow())
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}
