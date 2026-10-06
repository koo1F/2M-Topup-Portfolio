/**
 * lib/api.ts
 *
 * Single source of truth for the backend API URL used by all Next.js
 * route handlers (server-side proxy calls).
 *
 * Priority order:
 *   1. INTERNAL_API_URL  — Railway private networking URL (preferred in prod)
 *      e.g. http://backend.railway.internal:8080
 *   2. API_URL           — Public backend URL (fallback / local dev)
 *      e.g. https://backend-production-xxxx.up.railway.app
 *   3. http://localhost:8080  — Last-resort for local development only
 *
 * ⚠️  These are server-only variables (no NEXT_PUBLIC_ prefix).
 *     They are read at runtime inside Route Handlers, never baked into
 *     the client bundle.
 *
 * Railway setup:
 *   - In your frontend service → Variables, add:
 *       API_URL = https://backend-production-xxxx.up.railway.app
 *     or for private networking:
 *       INTERNAL_API_URL = http://backend.railway.internal:8080
 */
export function getApiUrl(): string {
  const url =
    process.env.INTERNAL_API_URL ||
    process.env.API_URL ||
    "http://localhost:8080";

  // Log once per cold-start so you can verify in Railway logs
  if (process.env.NODE_ENV === "production") {
    console.log(`[api] Backend URL resolved to: ${url}`);
  }

  return url;
}
