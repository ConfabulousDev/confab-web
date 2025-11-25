# Frontend Design Refresh Plan

**Goal:** Dive watch aesthetic - functional, precise, smaller fonts, no decoration, but slick and well-built.

## Phase 1: Global Foundation

### 1.1 Typography Scale (smaller, tighter)
```css
--font-xs: 0.7rem     (11px) - metadata, timestamps
--font-sm: 0.8rem     (13px) - secondary text, table cells
--font-base: 0.875rem (14px) - body text (down from 16px)
--font-lg: 1rem       (16px) - emphasis
--font-xl: 1.125rem   (18px) - section headers
--font-2xl: 1.25rem   (20px) - page titles
```

### 1.2 Color Palette (muted, professional)
```css
/* Neutral grays - the backbone */
--color-bg: #fafafa
--color-bg-card: #ffffff
--color-bg-hover: #f5f5f5
--color-border: #e5e5e5
--color-border-light: #efefef

/* Text hierarchy */
--color-text: #1a1a1a
--color-text-secondary: #666666
--color-text-muted: #999999

/* Accent - single accent color, used sparingly */
--color-accent: #0066cc
--color-accent-hover: #0052a3

/* Status - muted versions */
--color-success: #22863a
--color-error: #cb2431
--color-warning: #b08800
```

### 1.3 Spacing (tighter)
```css
--spacing-xs: 4px
--spacing-sm: 8px
--spacing-md: 12px
--spacing-lg: 16px
--spacing-xl: 24px
--spacing-2xl: 32px
```

### 1.4 Shadows (subtle)
```css
--shadow-sm: 0 1px 2px rgba(0,0,0,0.04)
--shadow-md: 0 2px 8px rgba(0,0,0,0.08)
--shadow-lg: 0 4px 16px rgba(0,0,0,0.12)
```

---

## Phase 2: Layout Components

### 2.1 App Shell / Navigation Header
- Fixed top header, 48px height
- Logo left, nav links center, user avatar right
- Subtle bottom border, no shadow
- Responsive: hamburger menu on mobile

```
┌─────────────────────────────────────────────────┐
│ ◉ Confab    Sessions  Shares  Keys    [avatar] │
└─────────────────────────────────────────────────┘
```

### 2.2 Page Container
- Max-width: 1200px for lists, 1400px for detail views
- Horizontal padding: 24px (desktop), 16px (mobile)
- Centered with `margin: 0 auto`

### 2.3 Card Component
- White background
- 1px border (not shadow by default)
- Border-radius: 6px
- Padding: 16px

---

## Phase 3: Page-Specific Fixes

### 3.1 Home Page
**Current:** Big empty hero, "You're authenticated!" feels placeholder-y
**Fix:**
- Remove hero layout, use compact header
- When logged in: go straight to dashboard-style layout
- Quick stats: X sessions, last activity Y
- Action buttons as subtle links, not big colored buttons
- Login button: simple, not GitHub-branded

### 3.2 Sessions List
**Current:** Plain HTML table, dashes for empty, "Untitled Session" repeated
**Fix:**
- Proper table with subtle row hover
- Empty values: just empty, no dashes
- Untitled sessions: show first line of transcript or session ID
- Compact rows (36px height)
- Monospace session IDs
- Relative timestamps ("2h ago" not "11/22/2025, 5:35:44 PM")
- Remove "Include shared sessions" to settings or filter dropdown

```
┌────────────────────────────────────────────────────────────────┐
│ Sessions                                          [Filter ▾]  │
├────────────────────────────────────────────────────────────────┤
│ Title                        Repo              Branch    Time │
│ ─────────────────────────────────────────────────────────────│
│ Svelte 5 transcript viewer   santaclaude/confab  main    2h   │
│ Web Worker transcript...     santaclaude/confab  main    5h   │
│ 9ecf095f                                                 3d   │
└────────────────────────────────────────────────────────────────┘
```

### 3.3 Session Detail
**Current:** Dense info dump, file paths overwhelming, buttons feel disconnected
**Fix:**
- Compact header: Session ID (mono), actions aligned right
- Metadata in subtle row below header (not a big box)
- Version selector: tabs or dropdown, not big card
- Files list: collapsible, show just filename not full path
- Transcript: full width, no container box around it

```
┌────────────────────────────────────────────────────────────────┐
│ a3186365-7820-4c01-afe8-a3471e21172a    [Share] [Delete] [←]  │
│ santaclaude/confab • main • 2h ago                             │
├────────────────────────────────────────────────────────────────┤
│ Version: [v1 ▾]  Files: 2  End: backfill                      │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  [Transcript content - full width]                            │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### 3.4 Transcript Viewer
**Current:** Good but buttons feel chunky
**Fix:**
- Smaller toggle buttons, icon-only with tooltips
- Tighter message spacing
- Code blocks: slightly smaller font

---

## Phase 4: Component Refinements

### 4.1 Buttons
- Default: ghost style (no background, just text + optional border)
- Primary: subtle fill, not bright
- Danger: red text, not red background
- Smaller padding: 6px 12px
- Font size: 13px

### 4.2 Tables
- No outer border
- Header: uppercase, smaller, muted color
- Rows: subtle bottom border only
- Hover: very subtle background change

### 4.3 Forms
- Input height: 32px
- Border: 1px solid #e5e5e5
- Focus: blue border, no shadow glow
- Labels: above input, small, muted

### 4.4 Alerts/Banners
- Thin left border for color indication
- Muted background
- Compact padding

---

## Implementation Order

1. **variables.css** - Update design tokens
2. **index.css** - Global resets, base styles
3. **Layout** - Create AppShell with nav header
4. **HomePage** - Simplify, add nav
5. **SessionsPage** - Table refinements
6. **SessionDetailPage** - Layout restructure
7. **Remaining pages** - APIKeys, Shares
8. **Components** - Button, Alert, etc.
9. **Responsive** - Verify all breakpoints

---

## Reference Aesthetics

- Linear.app - clean, dense, monochrome
- Vercel dashboard - minimal, lots of whitespace but purposeful
- GitHub - utilitarian tables, small text, functional
- Raycast - dark but the density/typography
