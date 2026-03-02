package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"webapp/internal/model"
	"webapp/internal/push"
	"webapp/internal/repository"
)

// PushService manages Web Push subscriptions and sends notifications.
type PushService struct {
	repo   *repository.PushRepo
	client *push.Client // nil when VAPID keys are not configured
}

func NewPushService(repo *repository.PushRepo, client *push.Client) *PushService {
	return &PushService{repo: repo, client: client}
}

// PublicKey returns the VAPID public key to send to browsers (may be empty if unconfigured).
func (s *PushService) PublicKey() string {
	if s.client == nil {
		return ""
	}
	return s.client.PublicKey()
}

// Subscribe saves a push subscription for the calling user.
func (s *PushService) Subscribe(ctx context.Context, endpoint, p256dh, auth string) error {
	auth_ := authFromCtx(ctx)
	return s.repo.Upsert(ctx, model.PushSubscription{
		UserID:   auth_.UserID,
		Endpoint: endpoint,
		P256DH:   p256dh,
		Auth:     auth,
	})
}

// Unsubscribe removes a push subscription for the calling user.
func (s *PushService) Unsubscribe(ctx context.Context, endpoint string) error {
	auth := authFromCtx(ctx)
	return s.repo.Delete(ctx, auth.UserID, endpoint)
}

// SendDueReminders sends a push notification to every user who has cards due
// today and has an active push subscription.
// Intended to be called by the daily background scheduler.
func (s *PushService) SendDueReminders(ctx context.Context) {
	if s.client == nil {
		slog.Debug("push: vapid not configured, skipping reminders")
		return
	}

	subs, err := s.repo.ListWithDueCards(ctx)
	if err != nil {
		slog.Error("push: list due users", "error", err)
		return
	}

	sent, gone, failed := 0, 0, 0
	for _, sub := range subs {
		body := buildPayload(sub.DueCount)
		result := s.client.Send(sub.Endpoint, sub.P256DH, sub.Auth, body)
		switch result {
		case push.SendOK:
			sent++
		case push.SendGone:
			gone++
			// Clean up the stale subscription asynchronously.
			ep := sub.Endpoint
			go func() {
				if err := s.repo.DeleteGone(context.Background(), ep); err != nil {
					slog.Warn("push: delete gone subscription", "error", err)
				}
			}()
		default:
			failed++
		}
	}
	slog.Info("push: daily reminders sent",
		"total", len(subs), "sent", sent, "gone", gone, "failed", failed)
}

func buildPayload(dueCount int) []byte {
	body := "Você tem 1 card para revisar hoje!"
	if dueCount > 1 {
		body = fmt.Sprintf("Você tem %d cards para revisar hoje!", dueCount)
	}
	b, _ := json.Marshal(map[string]any{
		"title": "Agronomia Flashcards",
		"body":  body,
		"icon":  "/static/icons/icon-192.png",
		"badge": "/static/icons/icon-192.png",
		"url":   "/",
	})
	return b
}
