package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// ShareInvitationParams contains the parameters for a share invitation email
type ShareInvitationParams struct {
	ToEmail      string
	SharerName   string
	SharerEmail  string
	SessionTitle string
	ShareURL     string
	ExpiresAt    *time.Time
	// Provider is the canonical session type (models.ProviderClaudeCode /
	// models.ProviderCodex). Empty or unknown values fall back to neutral
	// wording in the subject/body and emit an ERROR log so the operator
	// notices unrecognised providers.
	Provider string
	// ShareID is the DB share row identifier. Included in the
	// unknown-provider error log so on-call can correlate the offending
	// row. Optional for callers that don't yet have a share row (e.g.
	// preview rendering in tests).
	ShareID string
}

// Service defines the interface for email operations
type Service interface {
	// SendShareInvitation sends an invitation email for a shared session
	SendShareInvitation(ctx context.Context, params ShareInvitationParams) error
}

// RateLimitedService wraps a Service with rate limiting
type RateLimitedService struct {
	service      Service
	limiter      *EmailRateLimiter
	limitPerHour int
}

// NewRateLimitedService creates a new rate-limited email service
func NewRateLimitedService(service Service, limitPerHour int) *RateLimitedService {
	return &RateLimitedService{
		service:      service,
		limiter:      NewEmailRateLimiter(),
		limitPerHour: limitPerHour,
	}
}

// SendShareInvitation sends an invitation email with rate limiting
func (s *RateLimitedService) SendShareInvitation(ctx context.Context, userID int64, params ShareInvitationParams) error {
	if !s.limiter.Allow(userID, s.limitPerHour) {
		return ErrRateLimitExceeded
	}
	// Record the email send attempt
	s.limiter.Record(userID)
	return s.service.SendShareInvitation(ctx, params)
}

// CheckRateLimit checks if sending n emails would exceed the rate limit
// Returns nil if allowed, ErrRateLimitExceeded if not
func (s *RateLimitedService) CheckRateLimit(userID int64, count int) error {
	if !s.limiter.AllowN(userID, s.limitPerHour, count) {
		return ErrRateLimitExceeded
	}
	return nil
}

// EmailRateLimiter tracks email sends per user per hour using a sliding window algorithm.
//
// NOTE: This is intentionally separate from internal/ratelimit.InMemoryRateLimiter.
// The generic rate limiter uses a token bucket algorithm (golang.org/x/time/rate) which
// allows bursts and provides smooth rate limiting for APIs. This email limiter uses a
// sliding window with exact timestamp tracking to enforce strict "X emails per hour"
// limits without allowing bursts - important for preventing email spam and staying
// within email provider quotas.
type EmailRateLimiter struct {
	mu      sync.Mutex
	records map[int64][]time.Time
}

// NewEmailRateLimiter creates a new email rate limiter
func NewEmailRateLimiter() *EmailRateLimiter {
	return &EmailRateLimiter{
		records: make(map[int64][]time.Time),
	}
}

// Allow checks if a single email can be sent
func (l *EmailRateLimiter) Allow(userID int64, limitPerHour int) bool {
	return l.AllowN(userID, limitPerHour, 1)
}

// AllowN checks if n emails can be sent (without recording them)
func (l *EmailRateLimiter) AllowN(userID int64, limitPerHour int, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	oneHourAgo := time.Now().Add(-time.Hour)

	var valid []time.Time
	for _, ts := range l.records[userID] {
		if ts.After(oneHourAgo) {
			valid = append(valid, ts)
		}
	}
	l.records[userID] = valid

	return len(valid)+n <= limitPerHour
}

// Record records that an email was sent
func (l *EmailRateLimiter) Record(userID int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.records[userID] = append(l.records[userID], time.Now())
}

// ResendService implements Service using the Resend API
type ResendService struct {
	apiKey      string
	fromAddress string
	fromName    string
	frontendURL string // Base URL for building links (e.g., unsubscribe)
	httpClient  *http.Client
}

// NewResendService creates a new Resend email service
func NewResendService(apiKey, fromAddress, fromName, frontendURL string) *ResendService {
	return &ResendService{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
		frontendURL: frontendURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// resendRequest is the request body for Resend API
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// humanProviderLabel returns the phrase the recipient sees identifying which
// agent's session they were sent ("Claude Code session" / "Codex session").
// Unknown or empty values fall back to a neutral phrase ("session") and emit
// an ERROR log on the supplied context so on-call notices ungrounded values.
//
// Defensive: legacy "Claude Code" (display form, pre-CF-347) is treated as
// the canonical claude-code value here so the email layer is robust even if
// a caller forgot to normalise. New callers should still call
// models.NormalizeProvider at the boundary; this helper just refuses to silently
// emit the wrong wording or spurious unknown-provider logs for known legacy
// values.
//
// Local to the email package today; if a second surface needs the same
// mapping, lift this next to models.NormalizeProvider per CLAUDE.md's "Where
// shared code lives" guidance.
func humanProviderLabel(ctx context.Context, provider, shareID, toEmail string) string {
	switch models.NormalizeProvider(provider) {
	case models.ProviderClaudeCode:
		return "Claude Code session"
	case models.ProviderCodex:
		return "Codex session"
	default:
		logger.Ctx(ctx).Error("email: unknown provider, using neutral wording",
			"provider", provider,
			"share_id", shareID,
			"to_email", toEmail,
		)
		return "session"
	}
}

// composeSubject builds the email Subject header. Extracted so tests can
// exercise the wording without spinning up a transport.
func composeSubject(ctx context.Context, params ShareInvitationParams) string {
	phrase := humanProviderLabel(ctx, params.Provider, params.ShareID, params.ToEmail)
	return fmt.Sprintf("%s shared a %s transcript with you", params.SharerName, phrase)
}

// SendShareInvitation sends an invitation email via Resend
func (s *ResendService) SendShareInvitation(ctx context.Context, params ShareInvitationParams) error {
	// Resolve the provider phrase once so the unknown-provider ERROR log
	// (if any) fires exactly once per send, not once per template render.
	phrase := humanProviderLabel(ctx, params.Provider, params.ShareID, params.ToEmail)
	subject := fmt.Sprintf("%s shared a %s transcript with you", params.SharerName, phrase)

	htmlBody, err := renderHTMLTemplateWithPhrase(params, phrase, s.frontendURL)
	if err != nil {
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	textBody := renderTextTemplateWithPhrase(params, phrase, s.frontendURL)

	reqBody := resendRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress),
		To:      []string{params.ToEmail},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("resend API error (status %d): %v", resp.StatusCode, errResp)
	}

	return nil
}

var shareInvitationTmpl = template.Must(template.New("share_invitation").Parse(htmlTemplate))

// renderHTMLTemplate renders the HTML email template. Resolves the provider
// phrase using context.Background() — callers wanting log context should use
// renderHTMLTemplateWithPhrase directly.
func renderHTMLTemplate(params ShareInvitationParams, frontendURL string) (string, error) {
	phrase := humanProviderLabel(context.Background(), params.Provider, params.ShareID, params.ToEmail)
	return renderHTMLTemplateWithPhrase(params, phrase, frontendURL)
}

// renderHTMLTemplateWithPhrase renders the HTML email template with a
// pre-resolved provider phrase. SendShareInvitation uses this directly so
// the unknown-provider ERROR log fires only once per send.
func renderHTMLTemplateWithPhrase(params ShareInvitationParams, phrase, frontendURL string) (string, error) {
	data := templateData{
		SharerName:     params.SharerName,
		SharerEmail:    params.SharerEmail,
		SessionTitle:   params.SessionTitle,
		ShareURL:       params.ShareURL,
		UnsubscribeURL: frontendURL + "/unsubscribe",
		ProviderPhrase: phrase,
	}

	if params.ExpiresAt != nil {
		data.ExpiresAt = params.ExpiresAt.Format("January 2, 2006")
	}

	var buf bytes.Buffer
	if err := shareInvitationTmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// renderTextTemplate renders the plain text email template. Resolves the
// provider phrase using context.Background() — callers wanting log context
// should use renderTextTemplateWithPhrase directly.
func renderTextTemplate(params ShareInvitationParams, frontendURL string) string {
	phrase := humanProviderLabel(context.Background(), params.Provider, params.ShareID, params.ToEmail)
	return renderTextTemplateWithPhrase(params, phrase, frontendURL)
}

// renderTextTemplateWithPhrase renders the plain text email template with a
// pre-resolved provider phrase. SendShareInvitation uses this directly so
// the unknown-provider ERROR log fires only once per send.
func renderTextTemplateWithPhrase(params ShareInvitationParams, phrase, frontendURL string) string {
	title := params.SessionTitle
	if title == "" {
		title = "Untitled Session"
	}

	text := fmt.Sprintf(`%s (%s) shared a %s transcript with you.

Session: %s

View it here: %s
`, params.SharerName, params.SharerEmail, phrase, title, params.ShareURL)

	if params.ExpiresAt != nil {
		text += fmt.Sprintf("\nThis link expires on %s.\n", params.ExpiresAt.Format("January 2, 2006"))
	}

	text += fmt.Sprintf(`
---
Unsubscribe: %s/unsubscribe
`, frontendURL)

	return text
}

type templateData struct {
	SharerName     string
	SharerEmail    string
	SessionTitle   string
	ShareURL       string
	ExpiresAt      string
	UnsubscribeURL string
	// ProviderPhrase is the resolved per-provider noun phrase used in the
	// body line "shared a {{.ProviderPhrase}} transcript with you:".
	// Examples: "Claude Code session" / "Codex session" / "session".
	ProviderPhrase string
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="x-apple-disable-message-reformatting">
    <meta name="format-detection" content="telephone=no,address=no,email=no,date=no,url=no">
    <!--[if mso]>
    <noscript>
        <xml>
            <o:OfficeDocumentSettings>
                <o:PixelsPerInch>96</o:PixelsPerInch>
            </o:OfficeDocumentSettings>
        </xml>
    </noscript>
    <![endif]-->
    <style>
        @media screen and (max-width: 600px) {
            .email-container { width: 100% !important; }
            .email-padding { padding: 16px !important; }
            .content-padding { padding: 20px 16px !important; }
        }
    </style>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #fafafa; -webkit-text-size-adjust: 100%; -ms-text-size-adjust: 100%;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="background-color: #fafafa;">
        <tr>
            <td class="email-padding" style="padding: 24px;" align="center">
                <table role="presentation" class="email-container" width="560" cellspacing="0" cellpadding="0" border="0" style="max-width: 560px; width: 100%; background-color: #ffffff; border: 1px solid #e5e5e5; border-radius: 6px;">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 16px 24px; border-bottom: 1px solid #e5e5e5;">
                            <span style="font-family: Georgia, 'Times New Roman', serif; font-style: italic; font-size: 22px; color: #1a1a1a;">Confabulous</span>
                        </td>
                    </tr>
                    <!-- Content -->
                    <tr>
                        <td class="content-padding" style="padding: 24px;">
                            <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.5; color: #1a1a1a;">
                                <strong>{{.SharerName}}</strong> <span style="color: #666666;">({{.SharerEmail}})</span> shared a {{.ProviderPhrase}} transcript with you:
                            </p>

                            <!-- Session preview block (styled like user message) -->
                            <div style="margin: 0 0 20px 0; padding: 12px 16px; background-color: #f0fff4; border-radius: 6px; border: 1px solid #efefef; border-left: 3px solid #22863a;">
                                <span style="font-size: 14px; color: #1a1a1a;">{{if .SessionTitle}}{{.SessionTitle}}{{else}}Untitled Session{{end}}</span>
                            </div>

                            <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 0 20px 0;">
                                <tr>
                                    <td style="border-radius: 4px; background-color: #0066cc;">
                                        <a href="{{.ShareURL}}" target="_blank" style="display: inline-block; padding: 10px 20px; font-size: 14px; font-weight: 600; color: #ffffff; text-decoration: none;">View Session</a>
                                    </td>
                                </tr>
                            </table>

                            {{if .ExpiresAt}}<p style="margin: 0; font-size: 13px; color: #999999;">This link expires on {{.ExpiresAt}}.</p>{{end}}
                        </td>
                    </tr>
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 16px 24px; border-top: 1px solid #e5e5e5; background-color: #fafafa;">
                            <p style="margin: 0; font-size: 12px; color: #999999;">
                                <a href="{{.UnsubscribeURL}}" style="color: #999999; text-decoration: underline;">Unsubscribe</a>
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

