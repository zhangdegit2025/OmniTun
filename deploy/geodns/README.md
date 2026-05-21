# OmniTun GeoDNS Configuration

For multi-region deployment, configure GeoDNS to route agent connections
to the nearest Relay node.

## Regional Relay Endpoints

| Region         | Hostname            | Default Port |
|----------------|---------------------|--------------|
| ap-southeast-1 | relay-sg.omnitun.io | 4443         |
| us-east-1      | relay-us.omnitun.io | 4444         |
| eu-central-1   | relay-eu.omnitun.io | 4445         |

## Agent Auto-Discovery

Agents resolve the unified hostname `relay.omnitun.io` via DNS.
GeoDNS returns the IP of the nearest Relay based on the agent's
geographic location.

```
Agent DNS query: relay.omnitun.io
  → GeoDNS resolution
    → Nearest region IP returned
```

## DNS Zone Example (RFC 1035)

```
relay.omnitun.io.    300 IN CNAME relay-sg.omnitun.io.
relay-sg.omnitun.io. 300 IN A     <SG_IP>
relay-us.omnitun.io. 300 IN A     <US_IP>
relay-eu.omnitun.io. 300 IN A     <EU_IP>
```

## Health Check Configuration

Each Relay exposes a health endpoint at `:9090/healthz` (metrics port).
Configure GeoDNS health checks:

- **SG Region**: `http://<SG_IP>:9194/healthz`
- **US Region**: `http://<US_IP>:9195/healthz`
- **EU Region**: `http://<EU_IP>:9196/healthz`

## Latency-Based Routing

For latency-based GeoDNS providers (AWS Route 53, Azure Traffic Manager):

| Region         | Latency Profile        |
|----------------|------------------------|
| ap-southeast-1 | < 50ms for APAC users |
| us-east-1      | < 50ms for Americas   |
| eu-central-1   | < 50ms for EMEA       |
