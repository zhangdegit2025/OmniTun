# OmniTun Grafana Dashboards

Import dashboard.json into Grafana via:
1. Grafana → Dashboards → Import
2. Upload dashboard.json
3. Select Prometheus datasource

## Panels

| # | Panel | Type | Metric |
|---|-------|------|--------|
| 1 | Active Tunnels | stat | `omnitun_tunnels_active` |
| 2 | Active Connections | stat | `omnitun_connections_active` |
| 3 | Relay Health | stat | `omnitun_relay_heartbeat` |
| 4 | Agents Connected | stat | `omnitun_agents_connected` |
| 5 | Tunnel Creation Rate | timeseries | `rate(omnitun_tunnels_created_total[5m])` |
| 6 | Tunnel Start Duration P95 | timeseries | `histogram_quantile(0.95, rate(omnitun_tunnel_start_duration_seconds_bucket[5m]))` |
| 7 | Bytes Forwarded Rate | timeseries | `rate(omnitun_traffic_bytes_total[5m])` |
| 8 | Relay Bytes Forwarded | timeseries | `rate(omnitun_relay_bytes_forwarded_total[5m])` |
| 9 | API Request Rate | timeseries | `rate(omnitun_api_requests_total[5m])` |
| 10 | API Latency P95 | timeseries | `histogram_quantile(0.95, rate(omnitun_api_request_duration_seconds_bucket[5m]))` |
| 11 | Error Rate | timeseries | `rate(omnitun_api_errors_total[5m])` |
| 12 | P2P Success Rate | timeseries | `omnitun_p2p_success_rate` |
| 13 | Agent Connection Count | timeseries | `omnitun_agents_connected` |
| 14 | Relay Proxy Errors | timeseries | `rate(omnitun_relay_proxy_errors_total[5m])` |
| 15 | Gateway Messages Rate | timeseries | `rate(omnitun_gateway_messages_received_total[5m])` |

## Datasource

Requires a Prometheus datasource with UID `prometheus`. To change the UID:
1. Open dashboard.json
2. Replace all `"uid": "prometheus"` with your datasource UID
3. Re-import into Grafana

## Provisioning (optional)

To provision this dashboard automatically via Grafana provisioning:

```yaml
apiVersion: 1
providers:
  - name: 'OmniTun'
    folder: 'OmniTun'
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards
```

Then place `dashboard.json` in `/etc/grafana/provisioning/dashboards/`.
