package network

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type TopologyMode int

const (
	ModeDirect TopologyMode = iota
	ModeTURN
	ModeDERP
	ModeRelay
)

func (m TopologyMode) String() string {
	switch m {
	case ModeDirect:
		return "direct"
	case ModeTURN:
		return "turn"
	case ModeDERP:
		return "derp"
	case ModeRelay:
		return "relay"
	default:
		return "unknown"
	}
}

type TopologyPlanner struct {
	stunClient     *STUNClient
	holePunchCoord *HolePunchCoordinator
	turnRelay      *TURNRelay
	logger         *slog.Logger
	mu             sync.RWMutex
	routes         map[string]*TopoRoute
}

type TopoRoute struct {
	SourceNode  string
	TargetNode  string
	Mode        TopologyMode
	Latency     time.Duration
	Established time.Time
	LastChecked time.Time
	FailCount   int
}

func NewTopologyPlanner(stunAddr, turnAddr string, logger *slog.Logger) *TopologyPlanner {
	if logger == nil {
		logger = slog.Default()
	}
	return &TopologyPlanner{
		stunClient:     NewSTUNClient(stunAddr),
		holePunchCoord: NewHolePunchCoordinator(stunAddr),
		turnRelay:      NewTURNRelay(turnAddr),
		logger:         logger,
		routes:         make(map[string]*TopoRoute),
	}
}

func routeKey(source, target string) string {
	if source < target {
		return source + "↔" + target
	}
	return target + "↔" + source
}

func (tp *TopologyPlanner) PlanRoute(ctx context.Context, sourceNAT, targetNAT NATType, sourceAddr, targetAddr string) (*TopoRoute, error) {
	key := routeKey(sourceAddr, targetAddr)

	tp.mu.RLock()
	if existing, ok := tp.routes[key]; ok {
		tp.mu.RUnlock()
		return existing, nil
	}
	tp.mu.RUnlock()

	var mode TopologyMode

	switch {
	case sourceNAT == NATNone || targetNAT == NATNone:
		mode = ModeDirect
	case sourceNAT != NATSymmetric && targetNAT != NATSymmetric && tp.holePunchCoord.TryHolePunch(ctx, sourceNAT, targetNAT, sourceAddr, targetAddr):
		mode = ModeDirect
	case tp.turnRelay.IsAvailable():
		mode = ModeTURN
	default:
		mode = ModeRelay
	}

	route := &TopoRoute{
		SourceNode:  sourceAddr,
		TargetNode:  targetAddr,
		Mode:        mode,
		Established: time.Now(),
		LastChecked: time.Now(),
	}

	tp.mu.Lock()
	tp.routes[key] = route
	tp.mu.Unlock()

	tp.logger.InfoContext(ctx, "route planned",
		"source", sourceAddr,
		"target", targetAddr,
		"source_nat", sourceNAT.String(),
		"target_nat", targetNAT.String(),
		"mode", mode.String(),
	)

	return route, nil
}

func (tp *TopologyPlanner) MonitorRoute(ctx context.Context, routeKey string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tp.mu.RLock()
			route, ok := tp.routes[routeKey]
			tp.mu.RUnlock()

			if !ok {
				return
			}

			route.LastChecked = time.Now()

			if route.Latency > 5*time.Second {
				tp.logger.WarnContext(ctx, "route latency high, degrading",
					"route", routeKey,
					"latency", route.Latency,
					"mode", route.Mode.String(),
				)
				tp.DegradeRoute(routeKey)
			}

			if route.FailCount >= 3 {
				tp.logger.WarnContext(ctx, "route failures threshold reached, degrading",
					"route", routeKey,
					"fails", route.FailCount,
				)
				tp.DegradeRoute(routeKey)
			}
		}
	}
}

func (tp *TopologyPlanner) DegradeRoute(routeKey string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	route, ok := tp.routes[routeKey]
	if !ok {
		return
	}

	switch route.Mode {
	case ModeDirect:
		if tp.turnRelay.IsAvailable() {
			route.Mode = ModeTURN
		} else {
			route.Mode = ModeRelay
		}
		route.FailCount = 0
	case ModeTURN:
		route.Mode = ModeRelay
		route.FailCount = 0
	case ModeDERP:
		route.Mode = ModeRelay
		route.FailCount = 0
	case ModeRelay:
		route.FailCount++
	}

	tp.logger.Warn("route degraded",
		"route", routeKey,
		"new_mode", route.Mode.String(),
	)
}

func (tp *TopologyPlanner) UpgradeRoute(ctx context.Context, routeKey string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	route, ok := tp.routes[routeKey]
	if !ok {
		return nil
	}

	switch route.Mode {
	case ModeRelay:
		if tp.turnRelay.IsAvailable() {
			route.Mode = ModeTURN
			route.FailCount = 0
		}
	case ModeTURN:
		route.Mode = ModeDirect
		route.FailCount = 0
	case ModeDERP:
		route.Mode = ModeTURN
		route.FailCount = 0
	case ModeDirect:
		return nil
	}

	tp.logger.InfoContext(ctx, "route upgraded",
		"route", routeKey,
		"new_mode", route.Mode.String(),
	)

	return nil
}

func (tp *TopologyPlanner) GetRoute(key string) (*TopoRoute, bool) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	route, ok := tp.routes[key]
	return route, ok
}

func (tp *TopologyPlanner) RemoveRoute(key string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	delete(tp.routes, key)
}
