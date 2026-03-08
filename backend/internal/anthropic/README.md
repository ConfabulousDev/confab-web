# anthropic

HTTP client for the Anthropic Messages API with OpenTelemetry tracing.

## Files

| File | Role |
|------|------|
| `client.go` | `Client` struct, constructor with functional options, `CreateMessage` method |
| `client_test.go` | Tests using `httptest.Server` for `CreateMessage`, client options, and `GetTextContent` |
| `messages.go` | Request/response types, `APIError`, and helper methods |

## Key Types

- **`Client`** -- HTTP client holding API key, base URL, and `*http.Client`. Created via `NewClient`.
- **`MessagesRequest`** -- Request payload: model, max tokens, optional temperature, system prompt, and messages.
- **`Message`** -- A single conversation turn with `Role` and `Content`.
- **`MessagesResponse`** -- Full API response including content blocks, stop reason, and token usage.
- **`Usage`** -- Token counts: input, output, cache creation, and cache read.
- **`APIError`** -- Structured error from the Anthropic API; implements the `error` interface. Contains a top-level `Type`, a nested `ErrorDetail` of type `ErrorDetails`, and a `StatusCode` (excluded from JSON).
- **`ErrorDetails`** -- Nested error detail within `APIError`, containing `Type` and `Message`.
- **`ContentBlock`** -- A typed content block in the response (currently only `text` type is used).

## Key API

- **`NewClient(apiKey string, opts ...ClientOption) *Client`** -- Creates a client. Default timeout is 60 seconds, default base URL is `https://api.anthropic.com`.
- **`WithBaseURL(url string) ClientOption`** -- Overrides the base URL (useful for testing).
- **`WithTimeout(d time.Duration) ClientOption`** -- Overrides the HTTP timeout.
- **`(*Client).CreateMessage(ctx, *MessagesRequest) (*MessagesResponse, error)`** -- Sends a message and returns the response. Returns `*APIError` for HTTP 4xx/5xx responses when the body can be parsed; returns a plain error otherwise.
- **`(*MessagesResponse).GetTextContent() string`** -- Concatenates all text content blocks into a single string.

## How to Extend

### Adding support for a new API parameter

1. Add the field to `MessagesRequest` in `messages.go` with the appropriate JSON tag.
2. If the response shape changes, update `MessagesResponse` or `ContentBlock` accordingly.
3. Add OpenTelemetry attributes in `CreateMessage` if the parameter is worth tracing.

## Invariants

- **All API calls are traced.** `CreateMessage` creates an OpenTelemetry span with model, max tokens, status code, and token usage attributes. Errors are recorded on the span.
- **API errors are structured.** HTTP 4xx/5xx responses are parsed into `*APIError` with status code, error type, and message. Callers can type-assert to get details.
- **API version is pinned.** The `anthropic-version` header is set to `2023-06-01` for all requests.

## Design Decisions

**Functional options pattern.** `ClientOption` functions allow optional configuration without a sprawling constructor signature. The defaults (60s timeout, production URL) work for most callers.

**Single `GetTextContent` helper.** Rather than exposing complex content block iteration to callers, the response has a convenience method that handles the common case of extracting concatenated text.

## Testing

```bash
go test ./internal/anthropic/...
```

Tests use `WithBaseURL` to point the client at an `httptest.Server` that returns canned responses.

## Dependencies

**Uses:** `go.opentelemetry.io/otel` (tracing)

**Used by:** `internal/analytics` (smart recap generation)
