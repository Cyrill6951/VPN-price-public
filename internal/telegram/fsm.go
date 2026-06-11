package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// draft is the per-chat purchase/renew selection, stored in Redis.
type draft struct {
	CountryID  string `json:"country_id,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	PlanID     string `json:"plan_id,omitempty"`
	RenewSubID string `json:"renew_sub_id,omitempty"`
}

const draftTTL = time.Hour

func draftKey(chatID int64) string { return fmt.Sprintf("tg:draft:%d", chatID) }

func (b *Bot) getDraft(ctx context.Context, chatID int64) (draft, error) {
	var d draft
	raw, err := b.redis.Get(ctx, draftKey(chatID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	_ = json.Unmarshal(raw, &d)
	return d, nil
}

func (b *Bot) saveDraft(ctx context.Context, chatID int64, d draft) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return b.redis.Set(ctx, draftKey(chatID), raw, draftTTL).Err()
}

func (b *Bot) clearDraft(ctx context.Context, chatID int64) {
	_ = b.redis.Del(ctx, draftKey(chatID)).Err()
}
