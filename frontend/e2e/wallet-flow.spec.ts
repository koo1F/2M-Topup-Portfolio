import { expect, test } from "@playwright/test";
import { mkdirSync } from "node:fs";

const api = "http://127.0.0.1:18080";
const gallery = "public/screenshots";

// This uses a real API, SQL database, Redis and production worker. Only the
// external provider is a local signed-event fixture, absent in normal builds.
test("register, login, top up, settle once, enforce ownership and idempotency", async ({ page, request }) => {
  mkdirSync(gallery, { recursive: true });
  const browserJSON = (path: string) => page.evaluate(async (url) => {
    const response = await fetch(url);
    if (!response.ok) throw new Error(`Browser request ${url}: ${response.status}`);
    return response.json();
  }, path);
  const suffix = Date.now();
  const email = `portfolio-${suffix}@example.com`;
  const password = "PortfolioDemo123!";
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "2M-Topup. Money, elevated." })).toBeVisible();
  await page.screenshot({ path: `${gallery}/landing.png`, fullPage: true });
  await page.getByRole("main").getByRole("link", { name: "Create Account", exact: true }).click();
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.screenshot({ path: `${gallery}/register.png`, fullPage: true, mask: [page.locator('input[type="password"]')] });
  await page.getByRole("button", { name: "Sign Up" }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: "Sign In", exact: true }).click();
  await expect(page).toHaveURL(/\/dashboard/);
  await expect.poll(async () => (await browserJSON("/api/wallet")).balance).toBe(0);
  const cookie = (await page.context().cookies()).find((item) => item.name === "token");
  expect(cookie?.httpOnly).toBe(true);

  await page.goto("/payment");
  await page.getByPlaceholder("100.00").fill("100");
  await page.getByRole("button", { name: /Card Stripe Checkout/ }).click();
  await page.screenshot({ path: `${gallery}/topup.png`, fullPage: true });
  let payment!: { payment_id: string; status: string; redirect_url: string };
  await page.route("**/api/payment/create", async (route) => {
    const response = await route.fetch();
    payment = await response.json();
    await route.fulfill({ response });
  });
  await page.getByRole("button", { name: "Pay with Card", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Local test checkout" })).toBeVisible();
  expect(payment.status).toBe("PENDING");
  expect(payment.redirect_url).toMatch(/^http:\/\/127\.0\.0\.1:19090\/checkout\//);
  await page.getByRole("button", { name: "Complete test payment" }).click();
  await expect(page).toHaveURL(/\/payment\/success/);
  await expect.poll(async () => (await browserJSON(`/api/payment/${payment.payment_id}/status`)).status).toBe("SUCCESS");
  await expect.poll(async () => (await browserJSON("/api/wallet")).balance).toBe(100);

  const sessionID = `cs_test_${payment.payment_id}`;
  expect((await request.post(`http://127.0.0.1:19090/replay/${sessionID}`)).ok()).toBe(true);
  expect((await request.post(`http://127.0.0.1:19090/replay/${sessionID}?new_event=1`)).ok()).toBe(true);
  await expect.poll(async () => (await browserJSON("/api/wallet/transactions")).pagination.total).toBe(1);
  expect((await browserJSON("/api/wallet")).balance).toBe(100);

  // Warmed status cache must not expose the owner's payment to another user.
  const otherEmail = `other-${suffix}@example.com`;
  expect((await request.post(`${api}/api/auth/register`, { data: { email: otherEmail, password } })).status()).toBe(201);
  const otherToken = (await (await request.post(`${api}/api/auth/login`, { data: { email: otherEmail, password } })).json()).token;
  expect((await request.get(`${api}/api/payment/${payment.payment_id}/status`, { headers: { Authorization: `Bearer ${otherToken}` } })).status()).toBe(404);

  const key = `shared-${suffix}`;
  const ownerToken = cookie!.value;
  const ownerHeaders = { Authorization: `Bearer ${ownerToken}`, "Idempotency-Key": key };
  const body = { amount: 150, method: "card" };
  const first = await request.post(`${api}/api/payment/create`, { headers: ownerHeaders, data: body });
  const firstPayment = await first.json();
  expect(first.status()).toBe(201);
  const retry = await request.post(`${api}/api/payment/create`, { headers: ownerHeaders, data: body });
  expect(retry.status()).toBe(200);
  expect((await retry.json()).payment_id).toBe(firstPayment.payment_id);
  expect((await request.post(`${api}/api/payment/create`, { headers: ownerHeaders, data: { ...body, amount: 200 } })).status()).toBe(409);
  const second = await request.post(`${api}/api/payment/create`, { headers: { Authorization: `Bearer ${otherToken}`, "Idempotency-Key": key }, data: body });
  expect(second.status()).toBe(201);
  expect((await second.json()).payment_id).not.toBe(firstPayment.payment_id);
  expect((await request.post(`${api}/api/webhooks/stripe`, { data: {}, headers: { "X-Gateway-Signature": "known-mock-signature" } })).status()).toBe(401);

  await page.goto("/dashboard");
  await expect(page.getByText("100.00", { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: `${gallery}/dashboard.png`, fullPage: true });
  await page.goto("/wallet");
  await expect(page.getByRole("cell", { name: payment.payment_id })).toBeVisible();
  await page.screenshot({ path: `${gallery}/transactions.png`, fullPage: true });
  expect(errors).toEqual([]);
});
