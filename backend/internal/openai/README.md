# openai

Minimal HTTP client for the OpenAI Responses API (`POST /v1/responses`).

## Files

| File | Role |
|------|------|
| `client.go` | `Client` struct, constructor with functional options, `CreateResponse` method |
| `client_test.go` | Tests using `httptest.Server`: request shape (bearer auth, explicit `store: false`, no `temperature`), output-text/refusal helpers, `incomplete` status, `*APIError` vs plain errors, context cancellation, defaults |
| `responses.go` | Request/response types, `APIError`, and the `OutputText` / `Refusal` helpers |

## Key Types

- **`Client`** -- HTTP client holding API key, base URL, and `*http.Client`. Created via `NewClient`.
- **`ResponsesRequest`** -- Request payload: `Model`, `Instructions` (system prompt), `Input` (user content), `MaxOutputTokens`, optional `Reasoning` (effort), `Store` (`*bool`, so an explicit `false` is serialized), and `Text` (structured output format).
- **`TextConfig` / `TextFormat`** -- Output format; for structured outputs use `Type: "json_schema"` with `Name`, `Strict`, and the `Schema` document (`json.RawMessage`).
- **`ResponsesResponse`** -- `ID`, `Status` (`completed`, `incomplete`, ...), `IncompleteDetails` (reason), `Output` items, and `Usage`.
- **`OutputItem` / `ContentPart`** -- Output items; only `message` items carry `output_text` (`Text`) or `refusal` (`Refusal`) parts. Other item types (e.g. `reasoning`) are ignored by the helpers.
- **`Usage`** -- Input and output token counts. Reasoning tokens are a subset of output tokens.
- **`APIError`** -- Structured error (`Detail{Message, Type, Code}` plus `StatusCode`); implements `error`.

## Key API

- **`NewClient(apiKey string, opts ...ClientOption) *Client`** -- Creates a client. Default timeout is 60 seconds, default base URL is `https://api.openai.com`.
- **`WithBaseURL(url string) ClientOption`** -- Overrides the base URL (useful for testing).
- **`(*Client).CreateResponse(ctx, *ResponsesRequest) (*ResponsesResponse, error)`** -- Sends the request with `Authorization: Bearer <key>`. Returns `*APIError` for HTTP 4xx/5xx responses whose body parses as an OpenAI error; returns a plain error (status + body) otherwise. A response with `status: "incomplete"` is **not** an error at this layer — callers decide.
- **`(*ResponsesResponse).OutputText() string`** -- Concatenates every `output_text` part of every `message` item.
- **`(*ResponsesResponse).Refusal() string`** -- Concatenates every `refusal` part of every `message` item.

## How to Extend

### Adding support for a new API parameter

1. Add the field to `ResponsesRequest` in `responses.go`. Use `omitempty` only when the API default is what you want when the field is unset; use a pointer when the zero value must still be sent (as with `Store`).
2. If the response shape changes, update `ResponsesResponse`, `OutputItem`, or `ContentPart`.

## Invariants

- **This package does no logging.** Every failure path returns a wrapped error; the caller decides what to record.
- **Leaf package.** Zero internal dependencies.
- **No vendor policy here.** Smart-recap-specific choices (reasoning effort `none`, `store: false`, strict schema, no `temperature`, treating `incomplete`/refusal/empty output as failures) live in the `openaiRecapLLM` adapter in `internal/analytics/smart_recap_llm.go`.

## Design Decisions

**Hand-rolled, not the official SDK.** Mirrors `internal/anthropic`: one endpoint, a handful of fields, no transitive dependencies.

**Responses API, not Chat Completions.** Structured outputs and reasoning controls are first-class there, and `instructions` maps directly to the smart recap system prompt.

## Testing

```bash
go test ./internal/openai/...
```

Tests use `WithBaseURL` to point the client at an `httptest.Server` that returns canned responses.

## Dependencies

**Used by:** `internal/analytics` — the `openaiRecapLLM` adapter in `smart_recap_llm.go`, selected when `SMART_RECAP_LLM_PROVIDER=openai`.
