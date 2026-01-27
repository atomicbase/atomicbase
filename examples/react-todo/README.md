# React Todo Example

A Next.js todo application demonstrating Atomicbase with:
- Google OAuth authentication (Lucia pattern with Arctic + @oslojs)
- Database-per-user architecture
- shadcn/ui components

## Prerequisites

1. Atomicbase API server running
2. Google OAuth credentials from [Google Cloud Console](https://console.cloud.google.com/apis/credentials)

## Setup

1. Copy the environment variables:
   ```bash
   cp .env.local.example .env.local
   ```

2. Configure your `.env.local`:
   ```
   ATOMICBASE_URL=http://localhost:8080
   ATOMICBASE_API_KEY=your-api-key
   GOOGLE_CLIENT_ID=your-client-id
   GOOGLE_CLIENT_SECRET=your-client-secret
   NEXT_PUBLIC_APP_URL=http://localhost:3000
   ```

3. Push the schemas to Atomicbase:
   ```bash
   pnpm exec atomicbase push
   ```

4. Create the "primary" tenant for auth data:
   ```bash
   curl -X POST http://localhost:8080/platform/tenants \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-api-key" \
     -d '{"name": "primary", "template": "primary"}'
   ```

5. Start the development server:
   ```bash
   pnpm dev
   ```

6. Open http://localhost:3000

## Architecture

### Database Model

- **Primary database** (`primary` tenant): Stores users and sessions for authentication
- **User tenant databases** (`user-{googleId}`): Each user gets their own database for todos

### Schema Templates

**Primary** (`schemas/primary.ts`):
- `users` - User accounts linked to Google OAuth
- `sessions` - Session tokens with expiration

**Tenant** (`schemas/tenant.ts`):
- `todos` - User's todo items

### Auth Flow

1. User clicks "Sign in with Google"
2. Redirected to Google OAuth consent screen
3. On callback, user is created (if new) along with their tenant database
4. Session token is hashed (SHA-256) and stored, cookie set
5. User redirected to dashboard

## Tech Stack

- Next.js 16 (App Router)
- Tailwind CSS 4
- shadcn/ui components
- Atomicbase SDK
- Arctic (OAuth)
- @oslojs/crypto (session hashing)
