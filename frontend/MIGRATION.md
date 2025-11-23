# Migration Guide: Svelte → React

This document provides a step-by-step guide for migrating from the Svelte 5 frontend (`/frontend`) to the React frontend (`/frontend-new`).

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Development Setup](#development-setup)
3. [Production Deployment](#production-deployment)
4. [Backend Integration](#backend-integration)
5. [Testing Checklist](#testing-checklist)
6. [Troubleshooting](#troubleshooting)

## Prerequisites

Before migrating, ensure you have:

- Node.js 18+ installed
- Backend server running on `localhost:8080`
- Database and all backend services operational

## Development Setup

### 1. Install Dependencies

```bash
cd frontend-new
npm install
```

### 2. Start Development Server

```bash
npm run dev
```

The dev server will start on `http://localhost:5173` (or another port if 5173 is in use).

### 3. Verify Backend Connection

The React frontend is configured to proxy API requests to `localhost:8080`. Ensure your backend is running:

```bash
# In the backend directory
go run cmd/server/main.go
```

### 4. Test OAuth Flow

1. Navigate to `http://localhost:5173`
2. Click "Login with GitHub"
3. Complete OAuth flow
4. Verify you're redirected back and authenticated

### 5. Test Core Features

See [Testing Checklist](#testing-checklist) below.

## Production Deployment

### Option 1: Static File Serving (Recommended)

This approach builds the React app and serves it from the Go backend.

#### 1. Build the React App

```bash
cd frontend-new
npm run build
```

This creates a `dist/` directory with optimized production assets:
- `dist/index.html` - Entry point
- `dist/assets/` - JS/CSS bundles

#### 2. Update Go Backend Static File Handler

Modify your Go backend to serve the React build:

```go
// Example static file handler
r.PathPrefix("/").Handler(http.FileServer(http.Dir("frontend-new/dist")))
```

**Important**: Ensure SPA routing is handled correctly. All routes should serve `index.html` to allow React Router to handle routing:

```go
// Handle SPA routing
r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    path := "frontend-new/dist" + r.URL.Path
    _, err := os.Stat(path)
    if os.IsNotExist(err) {
        // If file doesn't exist, serve index.html (SPA routing)
        http.ServeFile(w, r, "frontend-new/dist/index.html")
        return
    }
    http.FileServer(http.Dir("frontend-new/dist")).ServeHTTP(w, r)
})
```

#### 3. Deploy

Deploy your Go backend as usual. The React frontend is now bundled with it.

### Option 2: Separate Static File Server

You can also serve the `dist/` directory with any static file server:

```bash
# Using a simple HTTP server
npx serve dist -p 3000

# Using nginx
# Configure nginx to serve dist/ and proxy /api to backend
```

**Nginx Configuration Example**:

```nginx
server {
    listen 80;
    server_name confab.example.com;

    root /path/to/frontend-new/dist;
    index index.html;

    # Serve static files
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests to backend
    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Proxy OAuth endpoints
    location /auth {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Backend Integration

### API Endpoints Required

The React frontend expects the following API endpoints (same as Svelte):

#### Authentication
- `GET /auth/status` - Check auth status
- `GET /auth/github/login` - Initiate GitHub OAuth
- `GET /auth/github/callback` - OAuth callback
- `POST /auth/logout` - Logout user

#### Sessions
- `GET /api/v1/sessions` - List sessions
- `GET /api/v1/sessions/:id` - Get session detail
- `POST /api/v1/sessions/:id/share` - Create share
- `DELETE /api/v1/sessions/:sessionId/shares/:shareId` - Delete share
- `GET /api/v1/sessions/:sessionId/shared/:token` - Get shared session

#### API Keys
- `GET /api/v1/api-keys` - List API keys
- `POST /api/v1/api-keys` - Create API key
- `DELETE /api/v1/api-keys/:id` - Delete API key

#### Files
- `GET /api/v1/files/:id/content` - Get file content

### CSRF Token

The React frontend expects a CSRF token endpoint:

- `GET /api/v1/csrf-token` - Get CSRF token

The token should be returned in the format:
```json
{
  "token": "csrf-token-value"
}
```

All mutating requests (POST, PUT, DELETE) will include this token in the `X-CSRF-Token` header.

## Testing Checklist

### Core Functionality

- [ ] **Authentication**
  - [ ] Login with GitHub
  - [ ] Logout
  - [ ] Auth state persists across page reloads
  - [ ] Redirects work correctly

- [ ] **Sessions List**
  - [ ] Sessions load and display
  - [ ] Sorting works (newest first)
  - [ ] Click on session navigates to detail page
  - [ ] Zero-byte transcripts are filtered out

- [ ] **Session Detail**
  - [ ] Session metadata displays correctly
  - [ ] Multiple runs are shown
  - [ ] Version selector works
  - [ ] Share dialog opens and closes
  - [ ] Creating public/private shares works
  - [ ] Email invitations for private shares work
  - [ ] Deleting shares works
  - [ ] Git information displays correctly
  - [ ] File list displays correctly

- [ ] **Transcript Viewer**
  - [ ] Messages render correctly
  - [ ] Virtual scrolling works smoothly with large transcripts (1000+ messages)
  - [ ] Expand/collapse controls work:
    - [ ] Expand All Thinking
    - [ ] Expand All Agents
    - [ ] Expand All Tools
    - [ ] Expand All Results
  - [ ] Time separators appear correctly
  - [ ] Message types display correctly:
    - [ ] User messages
    - [ ] Assistant messages
    - [ ] System messages
    - [ ] Tool use blocks
    - [ ] Tool result blocks
    - [ ] Thinking blocks
    - [ ] Agent panels

- [ ] **Agent Trees**
  - [ ] Recursive agent trees render correctly
  - [ ] Depth-based indentation works
  - [ ] Color-coded borders cycle correctly
  - [ ] Auto-expansion of first 2 levels works
  - [ ] Agent metadata displays (duration, tokens, tools)
  - [ ] Expand/collapse toggles work

- [ ] **Code Highlighting**
  - [ ] Syntax highlighting works for various languages
  - [ ] Copy to clipboard works
  - [ ] Truncation and expand toggle work
  - [ ] Bash output renders with terminal styling

- [ ] **Shared Sessions**
  - [ ] Public shares work without authentication
  - [ ] Private shares require authentication
  - [ ] Email-based access control works
  - [ ] Share expiration is handled correctly
  - [ ] Error states display correctly (404, 403, 410)

- [ ] **API Keys**
  - [ ] API keys list displays
  - [ ] Creating new API key works
  - [ ] Deleting API key works
  - [ ] Copy to clipboard works

### Performance

- [ ] **Large Transcripts**
  - [ ] Test with 1000+ message transcript
  - [ ] Scrolling is smooth (60fps)
  - [ ] No memory leaks during extended use
  - [ ] Virtual scrolling only renders visible items

- [ ] **Network**
  - [ ] Loading states display during API calls
  - [ ] Error states display on network failures
  - [ ] Retry mechanisms work

### Browser Compatibility

Test in:
- [ ] Chrome/Edge (latest)
- [ ] Firefox (latest)
- [ ] Safari (latest)

### Mobile Responsiveness

- [ ] Layout works on mobile devices
- [ ] Touch interactions work
- [ ] Virtual scrolling works on mobile
- [ ] No horizontal scrolling

## Troubleshooting

### Dev Server Won't Start

**Issue**: `npm run dev` fails

**Solutions**:
1. Ensure Node.js 18+ is installed: `node --version`
2. Delete `node_modules` and reinstall: `rm -rf node_modules && npm install`
3. Check for port conflicts (5173)

### API Requests Failing

**Issue**: All API requests return 404 or CORS errors

**Solutions**:
1. Ensure backend is running on `localhost:8080`
2. Check Vite proxy configuration in `vite.config.ts`
3. Verify API endpoints match expected paths

### OAuth Redirect Issues

**Issue**: OAuth redirects to wrong URL

**Solutions**:
1. Update GitHub OAuth app settings to include React dev server URL
2. Ensure `redirect_uri` in backend matches React app URL
3. Check for hardcoded URLs in backend OAuth config

### Virtual Scrolling Not Working

**Issue**: Large transcripts are slow or messages don't render

**Solutions**:
1. Check browser console for errors
2. Verify TanStack Virtual is installed: `npm list @tanstack/react-virtual`
3. Check `MessageList.tsx` for correct virtualizer setup

### Build Failures

**Issue**: `npm run build` fails with TypeScript errors

**Solutions**:
1. Run type check: `npm run tsc`
2. Fix any TypeScript errors
3. Ensure all dependencies are installed
4. Check `tsconfig.json` for correct configuration

### Production Bundle Too Large

**Issue**: Build output is larger than expected

**Solutions**:
1. Analyze bundle: `npm run build -- --mode analyze`
2. Check for duplicate dependencies
3. Ensure tree shaking is working
4. Consider code splitting for large components

### CSRF Token Errors

**Issue**: Mutating requests fail with 403

**Solutions**:
1. Verify `/api/v1/csrf-token` endpoint exists and returns valid token
2. Check `csrf.ts` initialization in `App.tsx`
3. Ensure `fetchWithCSRF` is used for mutating requests
4. Check browser console for CSRF token fetch errors

## Rollback Plan

If you need to rollback to the Svelte frontend:

1. Stop the React dev server
2. Reconfigure backend to serve Svelte frontend (`frontend/build`)
3. Restart backend
4. Test core functionality

## Next Steps

After successful migration:

1. Monitor production logs for errors
2. Collect user feedback
3. Address any browser-specific issues
4. Consider implementing future enhancements from README.md

## Support

For issues or questions:

1. Check this migration guide
2. Review README.md for feature documentation
3. Check backend logs for API errors
4. Open an issue in the project repository
