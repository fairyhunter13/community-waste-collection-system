// Shared SLO thresholds for all load test scenarios.
export const thresholds = {
  http_req_duration: ['p(95)<300', 'p(99)<800'],
  http_req_failed:   ['rate<0.01'],
  http_reqs:         [],
};
