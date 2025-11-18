# Spam Account Prevention Plan

## Current State

- GitHub OAuth required (provides baseline protection)
- Email allowlist (`ALLOWED_EMAILS`) for manual approval
- No automated spam prevention

## Threat Model

| Threat | Likelihood | Impact |
|--------|------------|--------|
| Automated bot signups | Low (GitHub OAuth blocks most) | High |
| Throwaway account abuse | Medium | Medium |
| Resource exhaustion (storage) | Medium | High |
| Shared link spam | Low | Low |

## Prevention Strategies

### Tier 1: Low Friction (Recommended First)

#### 1.1 GitHub Account Age Check
**Cost**: Free
**Friction**: None
**Effectiveness**: ★★★★☆

```go
// During OAuth callback
func checkAccountAge(githubUser *GitHubUser) error {
    accountAge := time.Since(githubUser.CreatedAt)
    if accountAge < 7 * 24 * time.Hour {
        return errors.New("account too new")
    }
    return nil
}
```

Reject GitHub accounts created within last 7 days.

#### 1.2 Signup Rate Limiting
**Cost**: Free
**Friction**: None
**Effectiveness**: ★★★☆☆

```go
// Rate limit: 5 signups per IP per hour
var signupLimiter = ratelimit.New(ratelimit.Config{
    Requests: 5,
    Window:   time.Hour,
    KeyFunc:  getClientIP,
})
```

#### 1.3 GitHub Activity Score
**Cost**: Free
**Friction**: None
**Effectiveness**: ★★★★☆

```go
type GitHubUser struct {
    CreatedAt   time.Time `json:"created_at"`
    PublicRepos int       `json:"public_repos"`
    Followers   int       `json:"followers"`
}

func calculateTrustScore(user *GitHubUser) int {
    score := 0

    // Account age (max 40 points)
    ageMonths := int(time.Since(user.CreatedAt).Hours() / 24 / 30)
    score += min(ageMonths * 4, 40)

    // Public repos (max 30 points)
    score += min(user.PublicRepos * 3, 30)

    // Followers (max 30 points)
    score += min(user.Followers * 2, 30)

    return score
}

// Require score >= 20 for signup
```

### Tier 2: Medium Friction

#### 2.1 Cloudflare Turnstile
**Cost**: Free (unlimited)
**Friction**: Low (invisible most of the time)
**Effectiveness**: ★★★★☆

```html
<!-- Frontend: Add to signup/login page -->
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
<div class="cf-turnstile" data-sitekey="YOUR_SITE_KEY"></div>
```

```go
// Backend: Verify token
func verifyTurnstile(token string) (bool, error) {
    resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify",
        url.Values{
            "secret":   {os.Getenv("TURNSTILE_SECRET")},
            "response": {token},
        })
    // ...
}
```

**Setup**: https://dash.cloudflare.com/?to=/:account/turnstile

#### 2.2 Disposable Email Blocking
**Cost**: Free (open source list)
**Friction**: Low
**Effectiveness**: ★★★☆☆

Use open source list: https://github.com/disposable-email-domains/disposable-email-domains

```go
var disposableDomains = loadDisposableDomains()

func isDisposableEmail(email string) bool {
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return false
    }
    domain := strings.ToLower(parts[1])
    return disposableDomains[domain]
}
```

### Tier 3: High Friction (If Problems Persist)

#### 3.1 Manual Approval Queue
**Cost**: Free (your time)
**Friction**: High
**Effectiveness**: ★★★★★

```sql
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'pending';
-- Values: pending, approved, rejected, suspended
```

Admin UI to review and approve signups.

#### 3.2 Phone Verification
**Cost**: ~$0.05/SMS
**Friction**: High
**Effectiveness**: ★★★★★

**Vendors**:
- Twilio Verify: $0.05/verification
- AWS SNS: $0.00645/SMS (US)
- Vonage: $0.0053/SMS

#### 3.3 Invite-Only System
**Cost**: Free
**Friction**: High
**Effectiveness**: ★★★★★

```sql
CREATE TABLE invites (
    id SERIAL PRIMARY KEY,
    code VARCHAR(32) UNIQUE NOT NULL,
    created_by_user_id INTEGER REFERENCES users(id),
    used_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    used_at TIMESTAMP
);
```

Each user gets N invite codes to share.

## Vendor Comparison

### CAPTCHA Services

| Service | Free Tier | Pros | Cons |
|---------|-----------|------|------|
| **Cloudflare Turnstile** | Unlimited | Invisible, privacy-focused | Newer |
| **hCaptcha** | Unlimited | Pays you, good privacy | More visible challenges |
| **reCAPTCHA v3** | 1M/month | Widely recognized | Privacy concerns, Google |

**Recommendation**: Cloudflare Turnstile

### Email Validation

| Service | Free Tier | Features |
|---------|-----------|----------|
| **Kickbox** | 100/day | Disposable + deliverability |
| **Abstract API** | 100/month | Simple API |
| **Open source list** | Unlimited | Disposable only |

**Recommendation**: Start with open source list, upgrade if needed

### IP Reputation

| Service | Free Tier | Features |
|---------|-----------|----------|
| **IPQualityScore** | 5K/month | VPN/proxy/tor detection |
| **AbuseIPDB** | 1K/day | Community reports |
| **Cloudflare** | Included | Bot score, threat score |

## Implementation Plan

### Phase 1: Quick Wins (1-2 hours)

1. Add GitHub account age check (reject < 7 days)
2. Add signup rate limiting (5/hour per IP)

```go
// In OAuth callback
if time.Since(githubUser.CreatedAt) < 7*24*time.Hour {
    http.Error(w, "Account too new. Please try again in a few days.", http.StatusForbidden)
    return
}
```

### Phase 2: Trust Scoring (2-4 hours)

1. Implement GitHub activity score
2. Store score with user record
3. Use score for feature gating (e.g., sharing requires score >= 30)

### Phase 3: CAPTCHA (2-3 hours)

1. Sign up for Cloudflare Turnstile
2. Add widget to login page
3. Verify token on backend

### Phase 4: Admin Tools (4-8 hours)

1. User management UI
2. Approval queue for low-trust signups
3. Ability to suspend/ban users

## Monitoring & Metrics

Track these to detect spam:
- Signups per day/week
- Signup to active user conversion
- Accounts with no sessions after 7 days
- Storage usage per user
- Share creation rate per user

## Configuration

```bash
# .env additions
GITHUB_MIN_ACCOUNT_AGE_DAYS=7
GITHUB_MIN_TRUST_SCORE=20
TURNSTILE_SITE_KEY=xxx
TURNSTILE_SECRET=xxx
SIGNUP_RATE_LIMIT=5
SIGNUP_RATE_WINDOW=3600
```

## Open Questions

1. **Remove ALLOWED_EMAILS?**
   - Keep as override for trusted users?
   - Deprecate once trust scoring works?

2. **Trust score thresholds**
   - What score for signup?
   - What score for sharing?
   - What score for unlimited storage?

3. **Appeal process**
   - How do legitimate new users get access?
   - Manual override request form?

4. **Existing users**
   - Grandfather all current users?
   - Backfill trust scores?

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| TBD | Start with Tier 1 | Low effort, no friction |
| TBD | Cloudflare Turnstile over reCAPTCHA | Privacy, free unlimited |
