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
)

// ShareInvitationParams contains the parameters for a share invitation email
type ShareInvitationParams struct {
	ToEmail      string
	SharerName   string
	SharerEmail  string
	SessionTitle string
	ShareURL     string
	ExpiresAt    *time.Time
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

// EmailRateLimiter tracks email sends per user per hour
type EmailRateLimiter struct {
	mu      sync.Mutex
	records map[int64]*rateLimitRecord
}

type rateLimitRecord struct {
	timestamps []time.Time
}

// NewEmailRateLimiter creates a new email rate limiter
func NewEmailRateLimiter() *EmailRateLimiter {
	return &EmailRateLimiter{
		records: make(map[int64]*rateLimitRecord),
	}
}

// Allow checks if a single email can be sent and records it if so
func (l *EmailRateLimiter) Allow(userID int64, limitPerHour int) bool {
	return l.AllowN(userID, limitPerHour, 1)
}

// AllowN checks if n emails can be sent (without recording them)
func (l *EmailRateLimiter) AllowN(userID int64, limitPerHour int, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)

	record, exists := l.records[userID]
	if !exists {
		record = &rateLimitRecord{timestamps: []time.Time{}}
		l.records[userID] = record
	}

	// Clean up old timestamps
	var valid []time.Time
	for _, ts := range record.timestamps {
		if ts.After(oneHourAgo) {
			valid = append(valid, ts)
		}
	}
	record.timestamps = valid

	// Check if we can send n more emails
	return len(record.timestamps)+n <= limitPerHour
}

// Record records that an email was sent
func (l *EmailRateLimiter) Record(userID int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	record, exists := l.records[userID]
	if !exists {
		record = &rateLimitRecord{timestamps: []time.Time{}}
		l.records[userID] = record
	}
	record.timestamps = append(record.timestamps, time.Now())
}

// ResendService implements Service using the Resend API
type ResendService struct {
	apiKey      string
	fromAddress string
	fromName    string
	httpClient  *http.Client
}

// NewResendService creates a new Resend email service
func NewResendService(apiKey, fromAddress, fromName string) *ResendService {
	return &ResendService{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
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

// SendShareInvitation sends an invitation email via Resend
func (s *ResendService) SendShareInvitation(ctx context.Context, params ShareInvitationParams) error {
	subject := fmt.Sprintf("%s shared a Confab session with you", params.SharerName)

	htmlBody, err := renderHTMLTemplate(params)
	if err != nil {
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	textBody := renderTextTemplate(params)

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
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("resend API error (status %d): %v", resp.StatusCode, errResp)
	}

	return nil
}

// renderHTMLTemplate renders the HTML email template
func renderHTMLTemplate(params ShareInvitationParams) (string, error) {
	tmpl, err := template.New("share_invitation").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	data := templateData{
		SharerName:   params.SharerName,
		SharerEmail:  params.SharerEmail,
		SessionTitle: params.SessionTitle,
		ShareURL:     params.ShareURL,
	}

	if params.ExpiresAt != nil {
		formatted := params.ExpiresAt.Format("January 2, 2006")
		data.ExpiresAt = &formatted
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// renderTextTemplate renders the plain text email template
func renderTextTemplate(params ShareInvitationParams) string {
	title := params.SessionTitle
	if title == "" {
		title = "Untitled Session"
	}

	text := fmt.Sprintf(`%s (%s) shared a Confab session with you.

Confab is a tool for saving and sharing AI conversation transcripts.

Session: %s

View it here: %s
`, params.SharerName, params.SharerEmail, title, params.ShareURL)

	if params.ExpiresAt != nil {
		text += fmt.Sprintf("\nThis link expires on %s.\n", params.ExpiresAt.Format("January 2, 2006"))
	}

	text += `
---
Confab · 548 Market St #835, San Francisco, CA 94104
Unsubscribe: https://confabulous.dev/unsubscribe
`

	return text
}

type templateData struct {
	SharerName   string
	SharerEmail  string
	SessionTitle string
	ShareURL     string
	ExpiresAt    *string
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #ffffff;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0">
        <tr>
            <td style="padding: 20px;">
                <p style="margin: 0 0 12px 0; font-size: 15px; line-height: 1.4; color: #374151;">
                    <strong>{{.SharerName}}</strong> ({{.SharerEmail}}) shared a Confab session with you.
                </p>

                <p style="margin: 0 0 12px 0; font-size: 14px; line-height: 1.4; color: #6b7280;">
                    Confab is a tool for saving and sharing AI conversation transcripts. Click below to view the shared session:
                </p>

                <div style="margin: 0 0 16px 0; padding: 12px; background-color: #f3f4f6; border-radius: 4px; border-left: 3px solid #6366f1;">
                    <span style="font-size: 15px; font-weight: 500; color: #111827;">{{if .SessionTitle}}{{.SessionTitle}}{{else}}Untitled Session{{end}}</span>
                </div>

                <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 0 16px 0;">
                    <tr>
                        <td style="border-radius: 4px; background-color: #6366f1;">
                            <a href="{{.ShareURL}}" target="_blank" style="display: inline-block; padding: 10px 20px; font-size: 14px; font-weight: 600; color: #ffffff; text-decoration: none;">View Session</a>
                        </td>
                    </tr>
                </table>

                {{if .ExpiresAt}}<p style="margin: 0 0 16px 0; font-size: 13px; color: #6b7280;">This link expires on {{.ExpiresAt}}.</p>{{end}}

                <p style="margin: 16px 0 0 0; padding-top: 12px; border-top: 1px solid #e5e7eb; font-size: 11px; line-height: 1.5; color: #9ca3af;">
                    Confab · 548 Market St #835, San Francisco, CA 94104<br>
                    <a href="https://confabulous.dev/unsubscribe" style="color: #9ca3af;">Unsubscribe</a>
                </p>
            </td>
        </tr>
    </table>
</body>
</html>`

// MockService is a mock implementation for testing
type MockService struct {
	SentEmails []ShareInvitationParams
	ShouldFail bool
	FailError  error
}

// NewMockService creates a new mock email service
func NewMockService() *MockService {
	return &MockService{
		SentEmails: []ShareInvitationParams{},
	}
}

// SendShareInvitation records the email params for testing
func (m *MockService) SendShareInvitation(ctx context.Context, params ShareInvitationParams) error {
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock email service failure")
	}
	m.SentEmails = append(m.SentEmails, params)
	return nil
}

// Reset clears all recorded emails
func (m *MockService) Reset() {
	m.SentEmails = []ShareInvitationParams{}
	m.ShouldFail = false
	m.FailError = nil
}
