import http from 'k6/http';
import { check, sleep } from 'k6';
import { thresholds } from './lib/thresholds.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '2m',  target: 20  },  // ramp up to nominal
    { duration: '2m',  target: 50  },  // stress
    { duration: '2m',  target: 100 },  // peak stress
    { duration: '2m',  target: 50  },  // scale back
    { duration: '2m',  target: 0   },  // ramp down
  ],
  thresholds,
};

export default function () {
  const headers = { 'Content-Type': 'application/json' };

  const listHH = http.get(`${BASE_URL}/api/households?page=1&per_page=20`);
  check(listHH, { 'list households': (r) => r.status === 200 });

  const ws = http.get(`${BASE_URL}/api/reports/waste-summary`);
  check(ws, { 'waste summary': (r) => r.status === 200 });

  const hhRes = http.post(
    `${BASE_URL}/api/households`,
    JSON.stringify({ owner_name: `Stress-${__VU}-${__ITER}`, address: '3 Stress Blvd' }),
    { headers },
  );
  check(hhRes, { 'household created': (r) => r.status === 201 });

  sleep(0.2);
}
