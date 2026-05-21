# OmniTun Load Tests

## Prerequisites

Install k6: https://k6.io/docs/get-started/installation/

```bash
# Windows (winget)
winget install k6

# Or via Chocolatey
choco install k6

# macOS
brew install k6

# Linux (Debian/Ubuntu)
sudo apt-get install k6
```

## Running

```bash
k6 run tests/load/tunnel-load.js
```

## Metrics

- **P95 latency target**: < 500ms
- **Error rate target**: < 1%
- **Concurrent users**: ramps up to 100

## Test Profile

| Stage   | Duration | Target VUs |
|---------|----------|------------|
| Ramp up | 30s      | 50         |
| Steady  | 60s      | 100        |
| Ramp down | 30s    | 0          |

## API Endpoints Tested

| Endpoint               | Method |
|------------------------|--------|
| /v1/auth/login         | POST   |
| /v1/auth/me            | GET    |
| /v1/dashboard/stats    | GET    |
| /v1/tunnels            | GET    |
| /v1/tunnels/:id        | GET    |
| /v1/domains            | GET    |
| /v1/org/usage          | GET    |
| /v1/api-keys           | GET    |
