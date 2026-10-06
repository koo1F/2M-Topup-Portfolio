# Load Test Results

Tested with k6 against Railway (Hong Kong) from local machine:

| Scenario | VUs | Error Rate | p95 Latency |
|---|---|---|---|
| Sustained load | 50 | 0.00% ✅ | 2.41s* |
| Spike test | 500 | 1.95% | 3.48s* |

\*Latency includes ~1.2s network round-trip (TH → HK)
Backend response time estimated ~300-500ms at 50 VUs
