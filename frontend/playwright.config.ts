import { defineConfig, devices } from "@playwright/test";

const databaseURL = process.env.E2E_DATABASE_URL ?? "postgres://user:password@127.0.0.1:55432/payment_test?sslmode=disable";
const redisURL = process.env.E2E_REDIS_URL ?? "127.0.0.1:56379";

export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:13000",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "on",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } } },
    { name: "firefox", use: { ...devices["Desktop Firefox"], viewport: { width: 1440, height: 1000 } } },
  ],
  webServer: [
    {
      command: "go run -tags=e2e ./cmd/server",
      cwd: "../backend",
      url: "http://127.0.0.1:18080/api/ready",
      timeout: 120_000,
      reuseExistingServer: !process.env.CI,
      env: {
        DATABASE_URL: databaseURL, REDIS_URL: redisURL, PORT: "18080",
        E2E_FIXTURE: "1", JWT_SECRET: "e2e-only-jwt-secret",
        STRIPE_SECRET_KEY: "sk_test_local_fixture", STRIPE_WEBHOOK_SECRET: "whsec_e2e_only",
        STRIPE_SUCCESS_URL: "http://127.0.0.1:13000/payment/success",
        STRIPE_CANCEL_URL: "http://127.0.0.1:13000/payment/cancel",
      },
    },
    {
      command: "npm run build && npm run start",
      url: "http://127.0.0.1:13000",
      timeout: 120_000,
      reuseExistingServer: !process.env.CI,
      env: { PORT: "13000", HOSTNAME: "127.0.0.1", API_URL: "http://127.0.0.1:18080", INTERNAL_API_URL: "", NEXT_TELEMETRY_DISABLED: "1" },
    },
  ],
});
