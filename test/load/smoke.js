import http from 'k6/http';
import { check, sleep } from 'k6';
import { thresholds } from './lib/thresholds.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  vus: 1,
  duration: '1m',
  thresholds,
};

// Minimal golden-flow smoke run: one VU exercises the full lifecycle.
export default function () {
  const headers = { 'Content-Type': 'application/json' };

  // Create household
  const hhRes = http.post(
    `${BASE_URL}/api/households`,
    JSON.stringify({ owner_name: `Smoke-${__VU}-${__ITER}`, address: '1 Test St' }),
    { headers },
  );
  check(hhRes, { 'household created': (r) => r.status === 201 });
  if (hhRes.status !== 201) return;
  const householdId = hhRes.json('data.id');

  // Create pickup
  const pkRes = http.post(
    `${BASE_URL}/api/pickups`,
    JSON.stringify({ household_id: householdId, type: 'plastic' }),
    { headers },
  );
  check(pkRes, { 'pickup created': (r) => r.status === 201 });
  if (pkRes.status !== 201) return;
  const pickupId = pkRes.json('data.id');

  // Schedule pickup
  const scheduleDate = new Date(Date.now() + 86400000).toISOString();
  const schRes = http.put(
    `${BASE_URL}/api/pickups/${pickupId}/schedule`,
    JSON.stringify({ pickup_date: scheduleDate }),
    { headers },
  );
  check(schRes, { 'pickup scheduled': (r) => r.status === 200 });

  // Complete pickup (auto-creates payment)
  const cmpRes = http.put(`${BASE_URL}/api/pickups/${pickupId}/complete`, null, { headers });
  check(cmpRes, { 'pickup completed': (r) => r.status === 200 });

  // Health check
  const hRes = http.get(`${BASE_URL}/health`);
  check(hRes, { 'health ok': (r) => r.status === 200 });

  sleep(1);
}
