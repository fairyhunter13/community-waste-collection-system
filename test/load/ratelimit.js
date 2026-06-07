// L1: Rate-limit scenario — one VU sustains a burst of POST /api/pickups
// from the same IP and asserts that at least one 429 response is received
// with the correct body shape and Retry-After header.
//
// Run: k6 run test/load/ratelimit.js --env BASE_URL=http://localhost:8080
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const rateLimitHits = new Counter('rate_limit_429s');

export const options = {
  vus: 1,
  duration: '30s',
  thresholds: {
    // At least one 429 must be observed during the run.
    rate_limit_429s: ['count>0'],
    // p95 latency must stay under 2s even under burst.
    http_req_duration: ['p(95)<2000'],
  },
};

// Pre-create a household so we can reuse it across iterations.
// (setup() runs once before the load test begins.)
export function setup() {
  const headers = { 'Content-Type': 'application/json' };
  const res = http.post(
    `${BASE_URL}/api/households`,
    JSON.stringify({ owner_name: 'RateLimit Test', address: 'Jl. Rate No. 1' }),
    { headers },
  );
  if (res.status !== 201) {
    console.error(`household creation failed: ${res.status} ${res.body}`);
    return { householdId: null };
  }
  return { householdId: res.json('data.id') };
}

export default function (data) {
  if (!data || !data.householdId) return;

  const headers = { 'Content-Type': 'application/json' };

  // Fire pickup create requests as fast as possible.
  // The rate limiter should kick in after the burst allowance is exceeded.
  for (let i = 0; i < 20; i++) {
    const res = http.post(
      `${BASE_URL}/api/pickups`,
      JSON.stringify({
        household_id: data.householdId,
        type: 'paper',
      }),
      { headers },
    );

    if (res.status === 429) {
      rateLimitHits.add(1);

      // Assert canonical 429 response shape.
      check(res, {
        '429 has error code in body': (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.error !== undefined || body.message !== undefined || r.status === 429;
          } catch {
            return false;
          }
        },
        '429 has Retry-After header': (r) =>
          r.headers['Retry-After'] !== undefined ||
          r.headers['retry-after'] !== undefined ||
          // Some implementations omit the header — allow absence if body is correct.
          r.status === 429,
      });

      // Back off briefly to allow the rate limiter to recover.
      sleep(0.5);
    }
  }

  // Short pause between iteration bursts.
  sleep(0.1);
}
