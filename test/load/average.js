import http from 'k6/http';
import { check, sleep } from 'k6';
import { thresholds } from './lib/thresholds.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '1m', target: 20 },  // ramp up
    { duration: '3m', target: 20 },  // steady state
    { duration: '1m', target: 0  },  // ramp down
  ],
  thresholds,
};

export default function () {
  const headers = { 'Content-Type': 'application/json' };

  // List endpoints (read-heavy mix)
  const listHH = http.get(`${BASE_URL}/api/households?page=1&per_page=10`);
  check(listHH, { 'list households ok': (r) => r.status === 200 });

  const listPK = http.get(`${BASE_URL}/api/pickups?page=1&per_page=10`);
  check(listPK, { 'list pickups ok': (r) => r.status === 200 });

  const listPY = http.get(`${BASE_URL}/api/payments?page=1&per_page=10`);
  check(listPY, { 'list payments ok': (r) => r.status === 200 });

  // Reports
  const ws = http.get(`${BASE_URL}/api/reports/waste-summary`);
  check(ws, { 'waste summary ok': (r) => r.status === 200 });

  const ps = http.get(`${BASE_URL}/api/reports/payment-summary`);
  check(ps, { 'payment summary ok': (r) => r.status === 200 });

  // Create new household + pickup (write traffic)
  const hhRes = http.post(
    `${BASE_URL}/api/households`,
    JSON.stringify({ owner_name: `Avg-${__VU}-${__ITER}`, address: '2 Load Ave' }),
    { headers },
  );
  check(hhRes, { 'household created': (r) => r.status === 201 });

  if (hhRes.status === 201) {
    const householdId = hhRes.json('data.id');
    const pkRes = http.post(
      `${BASE_URL}/api/pickups`,
      JSON.stringify({ household_id: householdId, type: 'paper' }),
      { headers },
    );
    check(pkRes, { 'pickup created': (r) => r.status === 201 });

    // L2: exercise the multipart payment-confirm write path on every iteration.
    // Create a full lifecycle: schedule → complete → confirm (BR-06).
    if (pkRes.status === 201) {
      const pickupId = pkRes.json('data.id');
      const tomorrow = new Date(Date.now() + 86400000).toISOString();

      const schedRes = http.put(
        `${BASE_URL}/api/pickups/${pickupId}/schedule`,
        JSON.stringify({ pickup_date: tomorrow }),
        { headers },
      );
      check(schedRes, { 'pickup scheduled': (r) => r.status === 200 });

      const complRes = http.put(`${BASE_URL}/api/pickups/${pickupId}/complete`, null, { headers });
      check(complRes, { 'pickup completed': (r) => r.status === 200 });

      if (complRes.status === 200) {
        // Find the newly created pending payment for this household.
        const payListRes = http.get(`${BASE_URL}/api/payments?household_id=${householdId}&status=pending`);
        if (payListRes.status === 200) {
          const payments = payListRes.json('data');
          if (payments && payments.length > 0) {
            const paymentId = payments[0].id;
            // Minimal JPEG magic bytes as proof file (satisfies the magic-byte sniff check).
            const fakeJpeg = new Uint8Array([
              0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10,
              0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
              0xFF, 0xD9,
            ]).buffer;
            const confirmRes = http.put(
              `${BASE_URL}/api/payments/${paymentId}/confirm`,
              { proof: http.file(fakeJpeg, 'receipt.jpg', 'image/jpeg') },
            );
            check(confirmRes, { 'payment confirmed': (r) => r.status === 200 });
          }
        }
      }
    }
  }

  sleep(0.5);
}
