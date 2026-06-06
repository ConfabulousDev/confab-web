package analytics

import (
	"context"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

type opencodeProvider struct{}

func init() {
	RegisterProvider(&opencodeProvider{}, models.ProviderOpencode)
}

func (p *opencodeProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	return nil, nil
}

func (p *opencodeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return &ComputeResult{}
	}
	return ComputeFromOpenCodeRollout(r)
}

func (p *opencodeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return ""
	}
	return extractOpenCodeSearchText(r)
}

func (p *opencodeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	return "", nil, nil
}

func (p *opencodeProvider) ClearMessageIDs() bool {
	return false
}

func (p *opencodeProvider) DisplayName() string {
	return "OpenCode"
}
