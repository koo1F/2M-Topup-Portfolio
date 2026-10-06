# 2M-Topup Frontend

Next.js 16.3 App Router frontend for the 2M-Topup wallet demo. Route handlers proxy authenticated requests to the Go API using an HTTP-only cookie.

See the [project README](../README.md) for architecture, Docker setup, Stripe configuration, and the complete demo flow.

## Local development

Start PostgreSQL, Redis, the Go API, and the worker as described in the project README. Then, from this directory:

```bash
cp .env.example .env.local
npm ci
npm run dev
```

Open http://localhost:3000 and register your own demo account.

`API_URL` defaults to `http://localhost:8080`. `INTERNAL_API_URL`, if set, takes priority. Both variables are used on the server.

## Checks

```bash
npm run lint
npx tsc --noEmit
npm run build
PORT=3000 HOSTNAME=127.0.0.1 npm start
```

The production build downloads the Inter font using `next/font/google`, so it needs access to Google Fonts. The build script prepares static assets for the standalone server. `npm start` starts it with port 3000 by default.

Browser and HTTP integration tests: see the [root README](../README.md#reproducible-integration-and-browser-tests).
