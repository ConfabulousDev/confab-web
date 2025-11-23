# Confab Frontend (React)

This is the React-based frontend for Confab, migrated from Svelte 5 to achieve better virtual scrolling performance and ecosystem compatibility.

## Tech Stack

- **React 18.3** - Modern React with hooks
- **TypeScript 5.6** - Type safety
- **Vite 7.2** - Fast build tool and dev server
- **React Router v6** - Client-side routing
- **TanStack Virtual** - High-performance virtual scrolling for large transcript lists
- **TanStack Query** - Server state management
- **Prism.js** - Syntax highlighting for code blocks
- **CSS Modules** - Scoped component styling

## Why React?

The migration from Svelte 5 to React was motivated by:

1. **Virtual Scrolling Performance**: TanStack Virtual provides battle-tested virtual scrolling for large transcript lists (1000+ messages)
2. **Ecosystem Maturity**: Better tooling, libraries, and Claude Code compatibility
3. **Svelte 5 Limitations**: Limited virtual scrolling libraries with compatibility issues

## Project Structure

```
frontend-new/
├── src/
│   ├── components/          # Reusable components
│   │   ├── RunCard.tsx
│   │   └── transcript/      # Transcript viewer components
│   │       ├── TranscriptViewer.tsx
│   │       ├── MessageList.tsx      # Virtual scrolling implementation
│   │       ├── Message.tsx
│   │       ├── ContentBlock.tsx
│   │       ├── CodeBlock.tsx
│   │       ├── BashOutput.tsx
│   │       └── AgentPanel.tsx       # Recursive agent tree
│   ├── pages/               # Route pages
│   │   ├── HomePage.tsx
│   │   ├── SessionsPage.tsx
│   │   ├── SessionDetailPage.tsx
│   │   ├── SharedSessionPage.tsx
│   │   └── APIKeysPage.tsx
│   ├── services/            # API and business logic
│   │   ├── api.ts
│   │   ├── csrf.ts
│   │   ├── transcriptService.ts
│   │   └── agentTreeBuilder.ts
│   ├── types/               # TypeScript type definitions
│   │   ├── index.ts
│   │   └── transcript.ts
│   ├── utils/               # Utility functions
│   │   └── index.ts
│   ├── App.tsx              # Root component with providers
│   ├── router.tsx           # Route configuration
│   └── main.tsx             # Entry point
├── public/                  # Static assets
├── vite.config.ts          # Vite configuration
├── tsconfig.json           # TypeScript configuration
└── package.json            # Dependencies
```

## Key Features

### Virtual Scrolling (Primary Goal)

The `MessageList` component uses TanStack Virtual to efficiently render large transcript lists:

```typescript
const virtualizer = useVirtualizer({
  count: virtualItems.length,
  getScrollElement: () => parentRef.current,
  estimateSize: () => 150,
  overscan: 5,
});
```

This enables smooth scrolling with thousands of messages without performance degradation.

### Recursive Agent Trees

The `AgentPanel` component recursively renders agent trees with:
- Depth-based indentation (20px per level, max 100px)
- Color-coded borders (6 colors cycling)
- Auto-expansion of first 2 levels
- Agent metadata (duration, tokens, tool use count)

### Code Highlighting

`CodeBlock` component uses Prism.js for syntax highlighting with:
- 20+ language support
- Line truncation with expand toggle
- Copy to clipboard
- Custom scrollbars

### Bash Output Rendering

`BashOutput` component provides terminal-style output:
- Dark theme styling
- ANSI escape sequence stripping
- Command prompt display
- Exit code display for errors

## Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Setup

```bash
cd frontend-new
npm install
npm run dev
```

The dev server will start on http://localhost:5173 (or another port if 5173 is in use).

### Build

```bash
npm run build
```

Outputs to `dist/` directory with optimized bundles:
- Minified JS/CSS
- Code splitting
- Tree shaking
- Gzip compression

### Preview Production Build

```bash
npm run preview
```

## Path Aliases

The project uses TypeScript path aliases for clean imports:

- `@/` → `src/`
- `@/components` → `src/components`
- `@/types` → `src/types`
- `@/services` → `src/services`
- `@/utils` → `src/utils`

Example:
```typescript
import { SessionDetail } from '@/types';
import { formatDate } from '@/utils';
import RunCard from '@/components/RunCard';
```

## API Integration

The frontend proxies API requests to the backend:

```typescript
// vite.config.ts
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

All API calls to `/api/*` are automatically proxied to the Go backend at `localhost:8080`.

## CSRF Protection

The app uses CSRF tokens for authenticated requests:

```typescript
import { fetchWithCSRF } from '@/services/csrf';

// Automatically includes CSRF token
const response = await fetchWithCSRF('/api/v1/sessions', {
  method: 'POST',
  body: JSON.stringify(data),
});
```

## Styling

Components use CSS Modules for scoped styling:

```typescript
// Component.tsx
import styles from './Component.module.css';

function Component() {
  return <div className={styles.container}>...</div>;
}
```

```css
/* Component.module.css */
.container {
  padding: 1rem;
}
```

## Migration Notes

### Svelte → React Pattern Conversions

| Svelte | React |
|--------|-------|
| `$:` reactive declarations | `useMemo`, `useEffect` |
| Svelte stores | `useState` |
| `onMount` | `useEffect` |
| `{#if}...{:else}` | Ternary operators, `&&` |
| `on:click` | `onClick` |
| `bind:value` | Controlled components |
| `$lib` imports | `@/` path aliases |
| Built-in scoped styles | CSS Modules |

### Type Definitions

All TypeScript types were ported verbatim from the Svelte version as they had no framework dependencies.

### Service Layer

Services (`api.ts`, `transcriptService.ts`, `agentTreeBuilder.ts`, `csrf.ts`) were ported with minimal changes - only import path updates from `$lib` to `@/`.

## Performance Optimizations

1. **Virtual Scrolling**: Only renders visible messages
2. **Code Splitting**: Automatic route-based splitting
3. **Lazy Loading**: Components loaded on demand
4. **Memoization**: `useMemo` for expensive computations
5. **CSS Modules**: Scoped styles, no global pollution

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

## Production Deployment

The production build is optimized and ready for deployment:

```bash
npm run build
# Outputs to dist/
# Serve dist/ with any static file server
```

For the Confab backend integration, the `dist/` directory should be served by the Go backend's static file handler.

## Future Enhancements

Potential improvements:

- [ ] Add React.lazy() for route-level code splitting
- [ ] Implement service worker for offline support
- [ ] Add E2E tests with Playwright
- [ ] Optimize bundle size with tree-shaking analysis
- [ ] Add dark mode support
- [ ] Implement keyboard shortcuts for navigation

## Contributing

When adding new components:

1. Use functional components with hooks
2. Add TypeScript types for all props
3. Use CSS Modules for styling
4. Follow existing naming conventions (camelCase for files/components)
5. Update this README if adding major features

## License

Same as Confab backend - see repository root.
