package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type BillingPlan struct {
	ID              string
	Name            string
	PriceMonthlyUSD int
	MaxTunnels      int
	MaxBandwidthGB  int
	Features        []string
}

var Plans = map[string]BillingPlan{
	"free":     {ID: "free", Name: "Free", PriceMonthlyUSD: 0, MaxTunnels: 1, MaxBandwidthGB: 1},
	"pro":      {ID: "pro", Name: "Pro", PriceMonthlyUSD: 1200, MaxTunnels: 10, MaxBandwidthGB: 100},
	"team":     {ID: "team", Name: "Team", PriceMonthlyUSD: 4900, MaxTunnels: 50, MaxBandwidthGB: 500},
	"business": {ID: "business", Name: "Business", PriceMonthlyUSD: 19900, MaxTunnels: 999999, MaxBandwidthGB: 5120},
}

type StripeService struct {
	secretKey     string
	webhookSecret string
	logger        *slog.Logger
	mockMode      bool
	repo          StripeRepository
}

type StripeRepository interface {
	UpdateSubscription(ctx context.Context, orgID string, planID string, status string) error
	CreateCustomer(ctx context.Context, orgID string, customerID string) error
}

func NewStripeService(secretKey, webhookSecret string, logger *slog.Logger, repo StripeRepository) *StripeService {
	mockMode := secretKey == "" || secretKey == "sk_test_mock"
	return &StripeService{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		logger:        logger,
		mockMode:      mockMode,
		repo:          repo,
	}
}

func (s *StripeService) CreateCheckoutSession(ctx context.Context, orgID, planID, successURL, cancelURL string) (string, error) {
	if s.mockMode {
		sessionID := "cs_mock_" + uuidStr()
		s.logger.Info("mock checkout session created", "org_id", orgID, "plan", planID, "session_id", sessionID)
		s.repo.UpdateSubscription(ctx, orgID, planID, "active")
		return successURL, nil
	}
	return "", fmt.Errorf("stripe not configured")
}

func (s *StripeService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.mockMode {
		s.logger.Info("mock webhook received", "payload", string(payload))
		return nil
	}
	return nil
}

func PlanFromString(planID string) (BillingPlan, bool) {
	p, ok := Plans[planID]
	return p, ok
}

func uuidStr() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i * 4))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func webhookEventType(payload []byte) string {
	var evt struct {
		Type string `json:"type"`
	}
	json.Unmarshal(payload, &evt)
	return evt.Type
}

var _ = webhookEventType

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}
