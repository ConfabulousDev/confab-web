package analytics_test

import (
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/shopspring/decimal"
)

// v2CostCard builds a minimal session_card_tokens_v2 record carrying just the
// per-session total cost. 37cg migrated the cost readers (session list, org
// analytics, trends costliest-N) onto session_card_tokens_v2, so any test that
// seeds cost through UpsertCards must seed a v2 card for that cost to be read
// back. by_provider is left empty because the migrated readers only consume the
// top-level total_cost_usd scalar.
func v2CostCard(sessionID string, cost float64) *analytics.TokensV2CardRecord {
	return &analytics.TokensV2CardRecord{
		SessionID:  sessionID,
		Version:    analytics.TokensV2CardVersion,
		ComputedAt: time.Now().UTC(),
		UpToLine:   100,
		Data: analytics.TokensV2Data{
			TotalCostUSD: decimal.NewFromFloat(cost).String(),
			ByProvider:   map[string]analytics.TokensV2Provider{},
		},
	}
}
