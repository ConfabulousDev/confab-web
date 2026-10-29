# Termly policy snapshot (2026-10-29)

**Archival only. Do not edit. Nothing renders from this directory.**

This is a verbatim copy of the five policies Termly hosted for confabulous.dev, taken before we cancelled Termly (kata t5v2 / ps18). The live copies that the app serves live in `frontend/src/legal/` (ported in evvf). If you need to change a policy, change it there, not here.

## Contents

For each document (`<slug>` is one of `tos`, `privacy`, `cookie`, `disclaimer`, `aup`):

| File | What it is |
|---|---|
| `<slug>.content.json` | Raw response of `GET /api/v1/consumer/policies/<uuid>/content`, byte-for-byte: `{content: <html>, created_locales, locale}`. |
| `<slug>.meta.json` | Raw response of `GET /api/v1/consumer/policies/<uuid>`, byte-for-byte. `data` holds the questionnaire answers that generated the policy. |
| `<slug>.html` | The `content` field written out as `<!doctype html><meta charset=utf-8>` + content, so it opens in a browser and can be diffed. Derived; the JSON is the record. |

## Documents

Fetched 2026-10-29 from Termly's public, unauthenticated consumer API (the same one `app.termly.io/policy-viewer` uses).

| Slug | Document (as titled) | policyUUID | "Last updated" (in text) | meta `updated_at` | `document_version` | `total_published_count` |
|---|---|---|---|---|---|---|
| `tos` | Terms of Service | `69001385-5934-4a9f-9ade-ca93873b3e6c` | January 09, 2026 | 2026-01-09T20:44:08-05:00 | 1 | 1 |
| `privacy` | Privacy Notice | `7366762a-c58a-4a7a-9cf0-f39620707a60` | January 09, 2026 | 2026-01-09T20:44:08-05:00 | 2 | 2 |
| `cookie` | Cookie Policy | `fec4df5c-7eb9-4687-9356-218047726cae` | January 09, 2026 | 2026-10-15T20:38:59-04:00 | 3 | 2 |
| `disclaimer` | Disclaimer | `7a3b8843-fe1c-43e3-8d02-88cb537a8874` | January 09, 2026 | 2026-01-09T23:53:53-05:00 | 1 | 1 |
| `aup` | Acceptable Use Policy | `2fb5c0b3-3458-48c5-8fcd-4803d17af25d` | January 09, 2026 | 2026-01-09T23:57:52-05:00 | 1 | 1 |

The Cookie Policy was republished on 2026-10-15, but its visible "Last updated" text still says January 09, 2026.

## Known issues in the source text

- Every document links `https://app.termly.io/dsar/<its-uuid>`, Termly's hosted data-request form, which stops working once the subscription ends. The live copies replace it with `mailto:contact@confabulous.dev`.
- The Cookie Policy's only listed cookie, `s7` / "Adobe Analytics", is a scanner false positive (see 5z5r). The scan entry (`cookie.meta.json` → `data.cookies.analytics`) shows Termly found it on `https://confabulous.dev/api/v1/csrf-token` in January 2026, when the backend still used gorilla/csrf (replaced by the cookieless `filippo.io/csrf` in c8d959f1). Confabulous has never used Adobe Analytics.
- There is no separate consent-banner or cookie-scan export: the public API exposes only the policies. The scan results Termly used are the `data.cookies` field of `cookie.meta.json`. The banner is being removed (5z5r/jks2), so its configuration is not preserved.

## Re-fetching

To check for drift right before cancelling (t2pd), re-run these commands into a scratch directory and `cmp` the results against this directory:

```sh
for pair in tos:69001385-5934-4a9f-9ade-ca93873b3e6c \
            privacy:7366762a-c58a-4a7a-9cf0-f39620707a60 \
            cookie:fec4df5c-7eb9-4687-9356-218047726cae \
            disclaimer:7a3b8843-fe1c-43e3-8d02-88cb537a8874 \
            aup:2fb5c0b3-3458-48c5-8fcd-4803d17af25d; do
  slug=${pair%%:*}; uuid=${pair#*:}
  curl -sS -o "$slug.content.json" "https://app.termly.io/api/v1/consumer/policies/$uuid/content"
  curl -sS -o "$slug.meta.json"    "https://app.termly.io/api/v1/consumer/policies/$uuid"
done
```

`impressions_count` (and similar counters) in the meta files change over time, so a diff there alone is not drift.

## Licence

Termly's Terms of Use (§2, version of July 1, 2025) grant the right to "reproduce, modify, and publicly display the Forms" for our own business's compliance. Nothing in them requires removal after cancellation. The full analysis, with quotes, is on kata t5v2.
