import http from 'k6/http';
import { check, sleep } from 'k6';
import { thresholds } from './lib/thresholds.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '30s', target: 0   },  // baseline
    { duration: '30s', target: 200 },  // instant spike
    { duration: '1m',  target: 200 },  // hold spike
    { duration: '30s', target: 0   },  // drop back
  ],
  thresholds: {
    // Spike test allows higher latency — we're testing stability not SLO
    http_req_duration: ['p(95)<1000'],
    http_req_failed:   ['rate<0.05'],
  },
};

export default function () {
  const health = http.get(`${BASE_URL}/health`);
  check(health, { 'alive during spike': (r) => r.status === 200 });

  const listHH = http.get(`${BASE_URL}/api/households?page=1&per_page=10`);
  check(listHH, { 'households ok during spike': (r) => r.status === 200 || r.status === 429 });

  sleep(0.1);
}
