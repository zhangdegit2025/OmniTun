package gateway

import (
	"github.com/omnitun/omnitun/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	gatewayAgentsConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "omnitun",
		Subsystem: "gateway",
		Name:      "agents_connected",
		Help:      "Number of currently connected agents.",
	})

	gatewayMessagesReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "gateway",
		Name:      "messages_received_total",
		Help:      "Total number of WebSocket messages received, partitioned by message type.",
	}, []string{"type"})

	gatewayMessagesSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "gateway",
		Name:      "messages_sent_total",
		Help:      "Total number of WebSocket messages sent, partitioned by message type.",
	}, []string{"type"})

	gatewayConnectionErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "gateway",
		Name:      "connection_errors_total",
		Help:      "Total number of connection errors encountered during WebSocket upgrade or message handling.",
	})

	gatewayHeartbeatTimeouts = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "omnitun",
		Subsystem: "gateway",
		Name:      "heartbeat_timeouts_total",
		Help:      "Total number of agent connections closed due to heartbeat timeout.",
	})

	gatewayWebSocketConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "omnitun",
	Subsystem: "gateway",
		Name:      "websocket_connections_total",
		Help:      "Total number of WebSocket connections established.",
	})
)

func init() {
	metrics.Registry.MustRegister(
		gatewayAgentsConnected,
		gatewayMessagesReceived,
		gatewayMessagesSent,
		gatewayConnectionErrors,
		gatewayHeartbeatTimeouts,
		gatewayWebSocketConnections,
	)
}
