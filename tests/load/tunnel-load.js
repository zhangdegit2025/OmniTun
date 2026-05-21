import http from 'k6/http';
import { check, sleep, group } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(95)<500'],
    'http_req_failed': ['rate<0.01'],
  },
};

const BASE_URL = 'http://localhost:8080';
let authToken = '';

export function setup() {
  const loginRes = http.post(`${BASE_URL}/v1/auth/login`, JSON.stringify({
    email: 'admin@omnitun.io',
    password: 'Admin123!',
  }), { headers: { 'Content-Type': 'application/json' } });

  const body = JSON.parse(loginRes.body);
  authToken = body.access_token;
  return { token: authToken };
}

export default function(data) {
  const headers = {
    'Authorization': `Bearer ${data.token}`,
    'Content-Type': 'application/json',
  };

  group('Dashboard Stats', () => {
    let res = http.get(`${BASE_URL}/v1/dashboard/stats`, { headers });
    check(res, { 'stats 200': (r) => r.status === 200 });
    sleep(0.5);
  });

  group('Tunnels API', () => {
    let res = http.get(`${BASE_URL}/v1/tunnels`, { headers });
    check(res, { 'tunnels 200': (r) => r.status === 200 });

    res = http.get(`${BASE_URL}/v1/tunnels/test-tunnel`, { headers });
    check(res, {
      'tunnel detail responds': (r) => r.status === 200 || r.status === 404,
    });

    sleep(0.5);
  });

  group('Auth API', () => {
    let res = http.get(`${BASE_URL}/v1/auth/me`, { headers });
    check(res, { 'me 200': (r) => r.status === 200 });
    sleep(0.5);
  });

  group('Domains API', () => {
    let res = http.get(`${BASE_URL}/v1/domains`, { headers });
    check(res, { 'domains 200': (r) => r.status === 200 });
    sleep(0.5);
  });

  group('Settings / Org API', () => {
    let res = http.get(`${BASE_URL}/v1/org/usage`, { headers });
    check(res, { 'usage 200': (r) => r.status === 200 });

    res = http.get(`${BASE_URL}/v1/api-keys`, { headers });
    check(res, { 'apikeys 200': (r) => r.status === 200 });

    sleep(0.5);
  });

  sleep(1);
}

export function teardown(data) {
  // No cleanup needed
}
