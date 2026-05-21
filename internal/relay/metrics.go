package relay

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	relayConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "connections_active",
		Help:      "Number of active relay connections.",
	})

	relayBytesForwarded = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "bytes_forwarded_total",
		Help:      "Total bytes forwarded by the relay.",
	})

	relayTunnelsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "tunnels_active",
		Help:      "Number of active tunnels on this relay.",
	})

	relayProxyErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "proxy_errors_total",
		Help:      "Total proxy errors by type.",
	}, []string{"type"})

	relayTunnelBytesIn = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "tunnel_bytes_in_total",
		Help:      "Total bytes received per tunnel.",
	}, []string{"tunnel_id"})

	relayTunnelBytesOut = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "relay",
		Name:      "tunnel_bytes_out_total",
		Help:      "Total bytes sent per tunnel.",
	}, []string{"tunnel_id"})
)

func recordBytesForwarded(n float64) {
	relayBytesForwarded.Add(n)
}

func recordProxyError(errType string) {
	relayProxyErrors.WithLabelValues(errType).Inc()
}

func setTunnelsActive(count int) {
	relayTunnelsActive.Set(float64(count))
}

func setConnectionsActive(count int) {
	relayConnectionsActive.Set(float64(count))
}
