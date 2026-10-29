# legal/

The legal documents for the managed service at confabulous.dev, published by Confabulous Software LLC. They render in-app at `/terms`, `/privacy`, `/cookies`, `/disclaimer` and `/acceptable-use` (via `pages/LegalDocPage.tsx`), and those routes only exist when `saasFooterEnabled` is on: self-hosted instances 404 on them.

## Files

| File | Role |
|------|------|
| `terms.md`, `privacy.md`, `cookies.md`, `disclaimer.md`, `acceptable-use.md` | The documents, as GitHub-flavoured Markdown. The page renders the title and "Last updated", so the files start at the first section (no `#` heading). One paragraph per line. |
| `docs.ts` | `LEGAL_DOCS` registry: `slug` (route), `title`, `description` (the `/policies` card) and `lastUpdated` (ISO date). Small on purpose: the router and `PoliciesPage` import it. Also exports the `LegalSlug` type. |
| `content.ts` | `LEGAL_MARKDOWN`: slug → Markdown, imported with Vite `?raw`. Only `LegalDocPage` imports it, so the policy text stays in that lazy chunk. |
| `legal.test.ts` | Content checks for every registered document (see below). |

## Editing a policy

1. Edit the `.md` file.
2. Bump that document's `lastUpdated` in `docs.ts`.
3. **Material changes need advance notice to users** before they take effect (for example, a new category of data collected, a new sub-processor, or changed liability terms). Decide how users will be told before merging.
4. Adding or changing a cookie (`backend/internal/auth/`) or a browser storage key (`localStorage` / `sessionStorage`, here or in `docs/`) means updating the tables in `cookies.md`.

## Links and anchors

- Headings get `id`s derived from their text (lowercased, punctuation dropped, spaces to hyphens; repeats get `-2`, `-3`). For example, `## 1. WHAT INFORMATION DO WE COLLECT?` becomes `#1-what-information-do-we-collect`. Renaming a heading changes its anchor, so update the links to it.
- Links to another policy use the route (`[confabulous.dev/terms](/terms)`); `LegalDocPage` routes them client-side.
- An anchor that isn't a heading uses inline HTML: `<a id="withdrawconsent"></a>` (Privacy Notice).

## Tests

`legal.test.ts` runs over every registry entry and fails if:

- a document is empty
- an in-page `#anchor` link has no matching `id` (catches renamed headings, and ids that `DOMPurify` strips)
- a document mentions the former policy host
- `lastUpdated` isn't an ISO date
- a document starts with its own `#` title

## History

These documents were ported from the hosted policies in `legal/snapshots/termly-2026-10-29/` (kata evvf). The port was verified word for word against that snapshot; the only changes were the hosted data-request link, links between policies, and conversion formatting. The Cookie Policy was rewritten from the 5z5r cookie and storage inventory. The snapshot is an archive: never edit it.
