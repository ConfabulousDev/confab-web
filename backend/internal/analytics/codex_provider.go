package analytics

import (
	"context"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

type codexProvider struct{}

func init() {
	RegisterProvider(&codexProvider{}, models.ProviderCodex)
}

func (p *codexProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	rollout, err := LoadCodexRollout(ctx, input.DB, input.Store, input.SessionID, input.UserID, input.Provider, input.ExternalID)
	if err != nil {
		return nil, err
	}
	if rollout == nil {
		return nil, nil
	}
	return rollout, nil
}

func (p *codexProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	return ComputeFromCodexRollout(rollout.(*codex.ParsedRollout))
}

func (p *codexProvider) SearchText(ctx context.Context, rollout Rollout) string {
	return ExtractCodexUserMessagesText(rollout.(*codex.ParsedRollout))
}

func (p *codexProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	transcript, idMap := PrepareCodexTranscript(rollout.(*codex.ParsedRollout))
	return transcript, idMap, nil
}

func (p *codexProvider) ClearMessageIDs() bool {
	return true
}
