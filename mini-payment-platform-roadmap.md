# Mini Payment Platform + Wallet System
## Roadmap ฉบับ Production-Ready

---

## Phase 0: เข้าใจ Architecture ก่อนเขียนโค้ด

```
                         User
                          |
                    Next.js Frontend
                          |
                    Go API Backend
                          |
        ------------------------------------
        |                |                 |
      Auth           Wallet Service     Payment Service
        |                |                 |
        ------------------------------------
                          |
                ----------------------
                |                    |
          PostgreSQL              Redis
          (ข้อมูลถาวร)     (cache / lock / queue)
                |                    |
                ----------------------
                          |
                    Background Worker
```

| ส่วน | หน้าที่ |
|---|---|
| Next.js | หน้าเว็บ + state management |
| Go API | Business logic หลัก |
| PostgreSQL | ข้อมูลถาวร + ACID transaction |
| Redis | Cache, Rate limit, Distributed lock, Queue |
| Worker | Process payment async |
| Docker | Environment ครบทุก service |

---

## Phase 1: Setup Project (สัปดาห์ที่ 1)

**เป้าหมาย:** ให้ระบบรันได้ก่อน แล้วค่อยเพิ่ม feature

**Output ที่ต้องเห็น:**
- `localhost:3000` → เห็นหน้าเว็บ
- `localhost:8080/api/health` → ตอบ `{"status":"ok"}`

### 1.1 โครงสร้าง Repository

```
mini-payment-platform/
├── frontend/
├── backend/
├── worker/
├── docker-compose.yml
└── README.md
```

### 1.2 docker-compose.yml เริ่มต้น

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: payment_db
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      DATABASE_URL: postgres://user:password@postgres:5432/payment_db
      REDIS_URL: redis:6379

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    depends_on:
      - backend

volumes:
  postgres_data:
```

---

## Phase 2: Backend Structure

**หลักการ:** อย่าเขียนทุกอย่างใน `main.go`

```
backend/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go          # อ่าน env vars
│   ├── database/
│   │   ├── postgres.go        # connection pool
│   │   └── redis.go           # redis client
│   ├── user/
│   │   ├── handler.go         # HTTP handler
│   │   ├── service.go         # business logic
│   │   └── repository.go      # DB queries
│   ├── wallet/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   ├── payment/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   └── middleware/
│       ├── auth.go            # JWT middleware
│       └── ratelimit.go       # Rate limit middleware
├── Dockerfile
└── go.mod
```

---

## Phase 3: Database Design (Production-Ready)

> ออกแบบให้ดีตั้งแต่ต้น แก้ทีหลังยากมาก

### 3.1 Migration Strategy

ใช้ tool เช่น `golang-migrate` หรือ `goose` อย่าเขียน SQL ตรงๆ ใน code

```
backend/
└── migrations/
    ├── 001_create_users.sql
    ├── 002_create_wallets.sql
    ├── 003_create_transactions.sql
    └── 004_create_payments.sql
```

### 3.2 ตาราง users

```sql
CREATE TABLE users (
  id           BIGSERIAL PRIMARY KEY,
  email        VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  is_active    BOOLEAN NOT NULL DEFAULT true,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_email_format CHECK (email ~* '^[^@]+@[^@]+\.[^@]+$')
);

-- Index สำหรับ login query
CREATE INDEX idx_users_email ON users(email);
```

### 3.3 ตาราง wallets

```sql
CREATE TABLE wallets (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL,
  balance    NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT wallets_user_fk FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT wallets_user_unique UNIQUE (user_id),      -- 1 user = 1 wallet
  CONSTRAINT wallets_balance_positive CHECK (balance >= 0)  -- ❗ ป้องกัน balance ติดลบ
);
```

### 3.4 ตาราง transactions

```sql
CREATE TYPE transaction_type AS ENUM ('DEPOSIT', 'WITHDRAW', 'PAYMENT', 'REFUND');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED');

CREATE TABLE transactions (
  id           BIGSERIAL PRIMARY KEY,
  wallet_id    BIGINT NOT NULL,
  type         transaction_type NOT NULL,
  amount       NUMERIC(15, 2) NOT NULL,
  status       transaction_status NOT NULL DEFAULT 'PENDING',
  reference_id VARCHAR(100) NOT NULL,   -- ❗ link กับ payment
  metadata     JSONB,                   -- ข้อมูลเพิ่มเติมแบบ flexible
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT transactions_wallet_fk FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  CONSTRAINT transactions_amount_positive CHECK (amount > 0),
  CONSTRAINT transactions_reference_unique UNIQUE (reference_id)  -- ❗ ป้องกัน duplicate
);

-- Index สำหรับ query บ่อย
CREATE INDEX idx_transactions_wallet_id ON transactions(wallet_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_reference_id ON transactions(reference_id);
```

### 3.5 ตาราง payments

```sql
CREATE TYPE payment_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED', 'EXPIRED');

CREATE TABLE payments (
  id                 BIGSERIAL PRIMARY KEY,
  user_id            BIGINT NOT NULL,
  amount             NUMERIC(15, 2) NOT NULL,
  status             payment_status NOT NULL DEFAULT 'PENDING',
  gateway_reference  VARCHAR(100),           -- reference จาก gateway
  idempotency_key    VARCHAR(100) NOT NULL,  -- ❗ ป้องกัน duplicate payment
  expires_at         TIMESTAMPTZ NOT NULL,   -- QR หมดอายุ
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  paid_at            TIMESTAMPTZ,

  CONSTRAINT payments_user_fk FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT payments_amount_positive CHECK (amount > 0),
  CONSTRAINT payments_idempotency_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_expires_at ON payments(expires_at) WHERE status = 'PENDING';
```

---

## Phase 4: Authentication

### 4.1 Register

```
POST /api/auth/register
```

Request:
```json
{
  "email": "test@test.com",
  "password": "123456"
}
```

**Backend flow:**
1. Validate email format + password length
2. Check email ซ้ำ
3. Hash password ด้วย `bcrypt` (cost factor >= 12)
4. **BEGIN TRANSACTION**
   - INSERT users
   - INSERT wallets (สร้าง wallet ให้เลย)
5. **COMMIT**
6. Return user data (ไม่ return password_hash)

### 4.2 Login

```
POST /api/auth/login
```

Response:
```json
{
  "token": "eyJhbGci...",
  "expires_at": "2026-01-02T00:00:00Z"
}
```

**ข้อควรระวัง:**
- ใช้ `bcrypt.CompareHashAndPassword` อย่า compare string ตรงๆ
- JWT ควรมี expiry (เช่น 24 ชั่วโมง)
- เก็บ `user_id` ใน JWT claims

---

## Phase 5: Wallet System

### 5.1 ดู Balance (พร้อม Redis Cache)

```
GET /api/wallet
Authorization: Bearer {token}
```

**Flow ที่ถูกต้อง:**
```
Request
  ↓
Redis GET "wallet:balance:{user_id}"
  ↓ hit               ↓ miss
Return cache       Query Postgres
                       ↓
                   Redis SET (TTL 30s)
                       ↓
                   Return data
```

Response:
```json
{
  "balance": 500.00,
  "currency": "THB"
}
```

**หมายเหตุ:** Invalidate cache ทุกครั้งที่มี transaction

### 5.2 ดู Transactions

```
GET /api/wallet/transactions?page=1&limit=20
Authorization: Bearer {token}
```

Response:
```json
{
  "data": [
    {
      "id": 1,
      "type": "DEPOSIT",
      "amount": 100.00,
      "status": "SUCCESS",
      "reference_id": "PAY001",
      "created_at": "2026-01-01T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 50
  }
}
```

---

## Phase 6: Payment System (พร้อม Idempotency)

### 6.1 Create Payment

```
POST /api/payment/create
Authorization: Bearer {token}
Idempotency-Key: {uuid}   ← ❗ Header นี้สำคัญมาก
```

Request:
```json
{
  "amount": 100.00
}
```

**Backend flow:**
1. ตรวจ `Idempotency-Key` header (required)
2. Check Redis ว่า key นี้เคย process แล้วหรือยัง
   - ถ้าเคย → return cached response ทันที
3. INSERT payments พร้อม `idempotency_key` และ `expires_at` (เช่น +15 นาที)
4. สร้าง QR code payload (mock)
5. Cache response ใน Redis (TTL = expires_at)

Response:
```json
{
  "payment_id": "PAY001",
  "amount": 100.00,
  "qr_payload": "00020101021...",
  "expires_at": "2026-01-01T10:15:00Z",
  "status": "PENDING"
}
```

### 6.2 Mock Gateway Webhook

```
POST /api/payment/webhook
X-Gateway-Signature: {hmac}  ← ❗ ต้อง verify signature เสมอ
```

Request:
```json
{
  "payment_id": "PAY001",
  "status": "SUCCESS",
  "gateway_reference": "GW-XYZ-001"
}
```

**Backend flow (Critical Section):**

```
1. Verify webhook signature (HMAC-SHA256)
2. ดึง payment จาก DB
3. ตรวจว่า status == PENDING (ถ้าไม่ใช่ → return 200 เฉยๆ)
   ↓
4. Acquire Distributed Lock ใน Redis
   Key: "payment:lock:{payment_id}"
   TTL: 30 seconds
   ↓
5. BEGIN TRANSACTION (Postgres)
   - UPDATE payments SET status='SUCCESS', paid_at=NOW()
   - INSERT transactions (type=DEPOSIT, status=SUCCESS)
   - UPDATE wallets SET balance = balance + amount
   - Invalidate Redis cache "wallet:balance:{user_id}"
   COMMIT
   ↓
6. Release Lock
   ↓
7. Return 200
```

**ทำไมต้องมี Lock?**
ป้องกัน webhook ยิงซ้ำสองครั้งพร้อมกัน (race condition) แม้จะมี `reference_id UNIQUE` แล้ว แต่ lock ทำให้ปลอดภัยกว่า

---

## Phase 7: Redis — ครบทุก Use Case

### 7.1 Cache Balance

```go
// Key pattern
"wallet:balance:{user_id}"  // TTL: 30 วินาที

// Invalidate เมื่อ
// - มี transaction ใหม่
// - withdraw / deposit สำเร็จ
```

### 7.2 Rate Limiting

```go
// Key pattern
"ratelimit:{user_id}:{endpoint}"  // เช่น "ratelimit:123:payment:create"

// Algorithm: Sliding Window
// ตัวอย่าง: payment/create → max 10 ครั้ง / นาที
```

### 7.3 Distributed Lock (สำคัญมาก)

```go
// Key pattern
"lock:payment:{payment_id}"  // TTL: 30 วินาที

// ใช้ SET NX PX (atomic operation)
// SET lock:payment:PAY001 "worker-1" NX PX 30000
```

### 7.4 Queue

```
Webhook รับแล้ว → Redis List LPUSH → Worker BRPOP → Process → Update DB
```

```go
// Queue key
"queue:payment:webhook"

// Worker อ่านแบบ blocking
BRPOP queue:payment:webhook 5
```

---

## Phase 8: Worker Service

```
worker/
├── main.go
├── processor/
│   └── payment.go    # logic การ process payment
└── Dockerfile
```

**Flow:**
```
Redis Queue
    ↓  (BRPOP - blocking pop)
Worker รับ job
    ↓
Process payment (call payment service)
    ↓
Success → อัพเดท DB + ส่ง notification
Failed  → Retry (max 3 ครั้ง) → Dead Letter Queue
```

**Retry Strategy:**
```
ครั้งที่ 1 → รอ 5 วินาที
ครั้งที่ 2 → รอ 30 วินาที
ครั้งที่ 3 → รอ 5 นาที
หลังจากนั้น → ย้ายไป Dead Letter Queue
```

---

## Phase 9: Frontend (Next.js)

### หน้าที่ต้องมี

| Route | หน้าที่ |
|---|---|
| `/` | Landing / Login redirect |
| `/register` | สมัครสมาชิก |
| `/login` | เข้าสู่ระบบ |
| `/dashboard` | ภาพรวม balance + recent transactions |
| `/wallet` | ประวัติ transactions ทั้งหมด |
| `/payment` | เติมเงิน + QR code |

### Dashboard

```
┌─────────────────────────────┐
│  ยินดีต้อนรับ, [ชื่อ]        │
├─────────────────────────────┤
│  ยอดคงเหลือ                  │
│  ฿ 500.00                   │
├─────────────────────────────┤
│  รายการล่าสุด                │
│  ✓ DEPOSIT   +100.00        │
│  ✓ PAYMENT    -50.00        │
│  ✗ WITHDRAW  [FAILED]       │
└─────────────────────────────┘
```

### Payment Page — ต้องมี Error State

```
┌─────────────────────────────┐
│  เติมเงิน                    │
│  จำนวน: [100] บาท           │
│  [สร้าง QR]                 │
├─────────────────────────────┤
│  State: PENDING             │
│  [QR IMAGE]                 │
│  หมดอายุใน: 14:32           │ ← countdown timer
├─────────────────────────────┤
│  State: SUCCESS  ✓          │
│  "ชำระเงินสำเร็จ"           │
├─────────────────────────────┤
│  State: EXPIRED  ✗          │
│  "QR หมดอายุแล้ว"           │
│  [สร้าง QR ใหม่]            │ ← error state
├─────────────────────────────┤
│  State: FAILED   ✗          │
│  "การชำระเงินล้มเหลว"        │
│  [ลองอีกครั้ง]              │ ← error state
└─────────────────────────────┘
```

**Polling Strategy:**
- Poll `GET /api/payment/{id}/status` ทุก 3 วินาที
- หยุด poll เมื่อ status เป็น SUCCESS / FAILED / EXPIRED

---

## Phase 10: Deployment

| ส่วน | Platform | หมายเหตุ |
|---|---|---|
| Frontend | Vercel | Auto-deploy จาก GitHub |
| Backend | Google Cloud Run | Scale-to-zero ประหยัดค่าใช้จ่าย |
| PostgreSQL | Supabase | มี free tier |
| Redis | Upstash | Serverless Redis, pay-per-use |

**ทางเลือกที่ง่ายกว่าสำหรับ Portfolio:**
- Backend → Railway (deploy ง่ายกว่า GCP มาก)
- ทั้ง stack → Railway ได้เลย (มี PostgreSQL + Redis ใน platform)

---

## Phase 11: Load Testing ด้วย k6

### Acceptance Criteria (ต้องตั้งก่อน test)

| Metric | เป้าหมาย |
|---|---|
| Throughput | >= 500 req/sec |
| p95 Latency | <= 500ms |
| p99 Latency | <= 1000ms |
| Error Rate | <= 1% |

### Test Script ตัวอย่าง

```javascript
// k6/load_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 100 },    // Ramp up
    { duration: '1m',  target: 1000 },   // Stay at 1000
    { duration: '30s', target: 5000 },   // Spike
    { duration: '1m',  target: 1000 },   // Back to normal
    { duration: '30s', target: 0 },      // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],  // Acceptance criteria
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  // 1. Login
  const loginRes = http.post('http://api/api/auth/login', JSON.stringify({
    email: `user${__VU}@test.com`,
    password: '123456',
  }));
  check(loginRes, { 'login success': (r) => r.status === 200 });

  const token = loginRes.json('token');
  const headers = { Authorization: `Bearer ${token}` };

  // 2. ดู wallet
  const walletRes = http.get('http://api/api/wallet', { headers });
  check(walletRes, { 'wallet ok': (r) => r.status === 200 });

  // 3. สร้าง payment
  const paymentRes = http.post('http://api/api/payment/create',
    JSON.stringify({ amount: 100 }),
    { headers: { ...headers, 'Idempotency-Key': `${__VU}-${__ITER}` } }
  );
  check(paymentRes, { 'payment created': (r) => r.status === 201 });

  sleep(1);
}
```

---

## ลำดับการทำจริง (ห้ามข้ามขั้น)

```
1. Setup repo + docker-compose
          ↓
2. Go API health check → localhost:8080/api/health
          ↓
3. PostgreSQL connection + migration tool
          ↓
4. สร้างตาราง + constraint ครบตั้งแต่ต้น
          ↓
5. User register + login (JWT)
          ↓
6. Wallet: ดู balance
          ↓
7. Payment: create + mock webhook (ยังไม่มี Redis)
          ↓
8. เพิ่ม Idempotency ใน payment webhook
          ↓
9. เพิ่ม Redis: cache → rate limit → lock → queue
          ↓
10. Worker: อ่าน queue + process payment
          ↓
11. Frontend: login → dashboard → payment flow
          ↓
12. Deploy (Railway ก่อน ถ้าต้องการ GCP ทีหลัง)
          ↓
13. Load test + ปรับ performance ตาม metric
```

---

## สรุปสิ่งที่เพิ่มจาก Version เดิม

| หัวข้อ | เพิ่มอะไร | ทำไม |
|---|---|---|
| Database | `CHECK constraints`, `UNIQUE`, Index | ป้องกัน data corruption |
| Payment | `Idempotency-Key` header | ป้องกัน double charge |
| Webhook | Distributed Lock | ป้องกัน race condition |
| Redis | Lock use case | ขาดไปจากเดิม |
| Frontend | Error states ครบ | UX ที่ดีต้องมี |
| Load Test | Acceptance criteria | วัดผลได้จริง |
| Migration | Tool-based migration | แก้ schema ง่ายขึ้น |
