# Portfolio demo

The project demonstrates a browser → Next.js proxy → Go API → PostgreSQL/Redis → payment worker flow. Describe it as a test payment application; it is not ready to handle real money.

## Automated gallery and recording

Run the browser test from the root README. It captures:

1. `frontend/public/screenshots/landing.png`
2. `frontend/public/screenshots/register.png` (password masked)
3. `frontend/public/screenshots/topup.png`
4. `frontend/public/screenshots/dashboard.png`
5. `frontend/public/screenshots/transactions.png`

Playwright also records `video.webm` under the wallet-flow test's `frontend/test-results/` directory. The local provider checkout is clearly marked as a fixture and makes no real payment. The video's automated interactions can be fast; use the outline below for a narrated recording.

In GitHub Actions, open the CI run and download the `browser-evidence` artifact. The artifact includes screenshots, videos, and a report. The reviewed gallery is included in this branch. Do not commit traces or authentication state.

## One-minute narration outline

| Time | Show | Explain |
| --- | --- | --- |
| 0–10s | Landing and architecture diagram | Next.js frontend, Go API, SQL database and Redis worker |
| 10–20s | Registration and login | bcrypt and JWT, with an HTTP-only frontend cookie |
| 20–35s | Create a 100 THB card top-up | Stripe test checkout or the clearly labelled local fixture |
| 35–50s | Dashboard and transaction history | Signed webhook enters Redis; worker credits the wallet in a SQL transaction |
| 50–60s | Tests/CI | Duplicate events cannot double-credit; another user cannot read the payment |

Use your own narration and explain the parts you implemented. Do not present historical k6 numbers as proof of production capacity.

## Public portfolio repository

This repository starts with fresh Git history and includes only the reviewed source and screenshots. Real environment files, generated binaries, authentication traces, and private history are excluded. Configure local settings from `.env.example` and keep credentials out of Git.
