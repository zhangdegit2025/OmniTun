package tunnel

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TunnelsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "omnitun_tunnels_active",
		Help: "Current number of active tunnels",
	})

	TunnelsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "omnitun_tunnels_created_total",
		Help: "Total number of tunnels created",
	})

	TunnelStartDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "omnitun_tunnel_start_duration_seconds",
		Help:    "Tunnel start duration in seconds",
		Buckets: prometheus.DefBuckets,
	})
)
