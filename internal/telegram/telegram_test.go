package telegram

import (
	"testing"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/vpn"
)

func TestStarsConversion(t *testing.T) {
	b := &Bot{starsPerUSD: 1}
	cases := map[float64]int{
		4.99:  5,
		0:     1, // minimum one star
		0.4:   1,
		12.49: 12,
		12.5:  13,
	}
	for usd, want := range cases {
		if got := b.stars(usd); got != want {
			t.Errorf("stars(%v) = %d, want %d", usd, want, got)
		}
	}

	b2 := &Bot{starsPerUSD: 100}
	if got := b2.stars(1); got != 100 {
		t.Errorf("stars at rate 100 = %d, want 100", got)
	}
}

func TestPlansKeyboard(t *testing.T) {
	plans := []vpn.PlanRef{
		{ID: uuid.New(), Name: "Monthly", Days: 30, Price: 4.99},
		{ID: uuid.New(), Name: "Yearly", Days: 365, Price: 39.99},
	}
	kb := plansKeyboard(plans)
	// 2 plan rows + 1 back row.
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("rows = %d, want 3", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].CallbackData != "plan:"+plans[0].ID.String() {
		t.Errorf("unexpected callback data: %q", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestMainMenuKeyboard(t *testing.T) {
	kb := mainMenuKeyboard()
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("main menu rows = %d, want 3", len(kb.InlineKeyboard))
	}
}
