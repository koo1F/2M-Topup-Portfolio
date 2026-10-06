import http from 'k6/http';
import { sleep } from 'k6';
import { check } from 'k6';
import exec from 'k6/execution';

// Test scenarios matching requirements:
// - Ramp up: 0 → 100 users in 30s
// - Stay: 100 users for 1m
// - Spike: 100 → 500 users in 30s
// - Back: 500 → 100 users in 1m
// - Ramp down: 100 → 0 in 30s
export const options = {
  setupTimeout: '5m',
  stages: [
    { duration: '30s', target: 100 },  // Ramp up to 100
    { duration: '1m', target: 100 },   // Stay at 100
    { duration: '30s', target: 500 },  // Spike to 500
    { duration: '1m', target: 100 },   // Back to 100
    { duration: '30s', target: 0 },    // Ramp down to 0
  ],
  // Acceptance criteria (thresholds):
  // - p95 latency < 500ms
  // - p99 latency < 1000ms
  // - error rate < 1%
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
  },
};

export function setup() {
  const targetUrl = 'https://backend-production-7e53.up.railway.app';
  const totalSeededUsers = 100;
  const requests = [];

  for (let i = 1; i <= totalSeededUsers; i++) {
    const email = `user${i}@loadtest.com`;
    const password = 'Password123!';
    requests.push([
      'POST',
      `${targetUrl}/api/auth/login`,
      JSON.stringify({ email: email, password: password }),
      { headers: { 'Content-Type': 'application/json' } }
    ]);
  }

  console.log(`=== Pre-authenticating ${totalSeededUsers} users in parallel in setup() ===`);
  const responses = http.batch(requests);

  const tokens = [];
  let successCount = 0;

  for (let i = 0; i < totalSeededUsers; i++) {
    const res = responses[i];
    const email = `user${i + 1}@loadtest.com`;
    if (res.status === 200) {
      tokens.push(res.json('token'));
      successCount++;
    } else {
      console.warn(`Failed to pre-authenticate ${email}: status=${res.status} body=${res.body}`);
      tokens.push(null);
    }
  }

  console.log(`=== Pre-authentication completed: got ${successCount} tokens ===`);
  return { tokens: tokens };
}

export default function (data) {
  const targetUrl = 'https://backend-production-7e53.up.railway.app';
  
  // Get current VU and Iteration numbers
  const vuId = exec.vu.idInInstance;
  const iterId = exec.vu.iterationInInstance;
  
  // Map VUs to the 100 seeded users (0-99 index mapping)
  const totalSeededUsers = 100;
  const userIndex = ((vuId - 1) % totalSeededUsers);
  
  const token = data.tokens[userIndex];
  if (!token) {
    sleep(1);
    return;
  }

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };
  
  // 1. GET /api/wallet
  const walletRes = http.get(`${targetUrl}/api/wallet`, { headers: headers });
  check(walletRes, {
    'wallet status is 200': (r) => r.status === 200,
  });
  
  // 2. POST /api/payment/create
  const idempotencyKey = `idemp-vu-${vuId}-iter-${iterId}-${Date.now()}-${Math.floor(Math.random() * 1000000)}`;
  const paymentRes = http.post(
    `${targetUrl}/api/payment/create`,
    JSON.stringify({ amount: 100 }),
    {
      headers: Object.assign({}, headers, {
        'Idempotency-Key': idempotencyKey,
      }),
    }
  );
  
  check(paymentRes, {
    'payment create status is 201': (r) => r.status === 201,
  });
  
  sleep(3);
}
