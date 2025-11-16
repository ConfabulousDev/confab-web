# Confab Frontend

Minimal SvelteKit frontend for Confab with GitHub OAuth authentication.

## Features

- Public home page
- GitHub OAuth login flow
- Authenticated user display
- Minimal, clean styling

## Development

### Prerequisites

- Node.js 18+
- Backend running on `http://localhost:8080`

### Setup

```bash
# Install dependencies
npm install

# Start dev server
npm run dev
```

The frontend will run on `http://localhost:5173`

### How It Works

1. User clicks "Login with GitHub" button
2. Redirects to `/auth/github/login` (proxied to backend)
3. GitHub OAuth flow completes
4. Backend sets session cookie and redirects back to `/`
5. Frontend calls `/api/v1/me` to get user info
6. Displays authenticated state

### Backend Integration

The Vite config proxies these routes to the backend:
- `/auth/*` → `http://localhost:8080/auth/*`
- `/api/*` → `http://localhost:8080/api/*`

This avoids CORS issues during development.

## Build

```bash
npm run build
```

## Preview

```bash
npm run preview
```
