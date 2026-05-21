package network

import (
	"context"
	"testing"
	"time"
)

func TestTopologyMode_String(t *testing.T) {
	tests := []struct {
		mode     TopologyMode
		expected string
	}{
		{ModeDirect, "direct"},
		{ModeTURN, "turn"},
		{ModeDERP, "derp"},
		{ModeRelay, "relay"},
		{TopologyMode(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.mode.String()
		if got != tt.expected {
			t.Errorf("TopologyMode(%d).String() = %q, want %q", tt.mode, got, tt.expected)
		}
	}
}

func TestTopologyPlanner_PlanRoute_SameLAN(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	route, err := tp.PlanRoute(ctx, NATNone, NATNone, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	if route.Mode != ModeDirect {
		t.Errorf("expected ModeDirect for same LAN, got %s", route.Mode)
	}
}

func TestTopologyPlanner_PlanRoute_SymmetricNAT(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	route, err := tp.PlanRoute(ctx, NATSymmetric, NATSymmetric, "192.168.1.1:41641", "10.0.0.1:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	if route.Mode != ModeTURN {
		t.Errorf("expected ModeTURN for symmetric NAT (with TURN available), got %s", route.Mode)
	}
}

func TestTopologyPlanner_PlanRoute_UnknownNAT_Relay(t *testing.T) {
	tp := NewTopologyPlanner("", "", nil)
	ctx := context.Background()

	route, err := tp.PlanRoute(ctx, NATUnknown, NATUnknown, "192.168.1.1:41641", "10.0.0.1:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	if route.Mode != ModeRelay {
		t.Errorf("expected ModeRelay for unknown NAT without TURN, got %s", route.Mode)
	}
}

func TestTopologyPlanner_DegradeRoute(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	route, err := tp.PlanRoute(ctx, NATNone, NATNone, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	if route.Mode != ModeDirect {
		t.Fatalf("expected ModeDirect initially, got %s", route.Mode)
	}

	key := routeKey("10.0.0.1:41641", "10.0.0.2:41642")

	tp.DegradeRoute(key)

	route, ok := tp.GetRoute(key)
	if !ok {
		t.Fatal("route not found after degrade")
	}

	if route.Mode != ModeTURN {
		t.Errorf("expected ModeTURN after degrade from Direct (TURN available), got %s", route.Mode)
	}

	tp.DegradeRoute(key)

	route, ok = tp.GetRoute(key)
	if !ok {
		t.Fatal("route not found after second degrade")
	}

	if route.Mode != ModeRelay {
		t.Errorf("expected ModeRelay after second degrade, got %s", route.Mode)
	}
}

func TestTopologyPlanner_UpgradeRoute(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	route, err := tp.PlanRoute(ctx, NATUnknown, NATUnknown, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	key := routeKey("10.0.0.1:41641", "10.0.0.2:41642")

	if route.Mode == ModeDirect {
		t.Logf("initial mode: %s (TURN available, so ModeTURN or ModeRelay expected)", route.Mode)
	}

	err = tp.UpgradeRoute(ctx, key)
	if err != nil {
		t.Fatalf("UpgradeRoute failed: %v", err)
	}

	route, ok := tp.GetRoute(key)
	if !ok {
		t.Fatal("route not found after upgrade")
	}
}

func TestTopologyPlanner_UpgradeRoute_WithTURN(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	tp.mu.Lock()
	key := routeKey("10.0.0.1:41641", "10.0.0.2:41642")
	tp.routes[key] = &TopoRoute{
		SourceNode:  "10.0.0.1:41641",
		TargetNode:  "10.0.0.2:41642",
		Mode:        ModeRelay,
		Established: time.Time{},
	}
	tp.mu.Unlock()

	err := tp.UpgradeRoute(ctx, key)
	if err != nil {
		t.Fatalf("UpgradeRoute failed: %v", err)
	}

	route, ok := tp.GetRoute(key)
	if !ok {
		t.Fatal("route not found after upgrade")
	}

	if route.Mode != ModeTURN {
		t.Errorf("expected ModeTURN after upgrade from Relay (TURN available), got %s", route.Mode)
	}

	err = tp.UpgradeRoute(ctx, key)
	if err != nil {
		t.Fatalf("second UpgradeRoute failed: %v", err)
	}

	route, ok = tp.GetRoute(key)
	if !ok {
		t.Fatal("route not found after second upgrade")
	}

	if route.Mode != ModeDirect {
		t.Errorf("expected ModeDirect after second upgrade, got %s", route.Mode)
	}
}

func TestTopologyPlanner_PlanRoute_Existing(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	ctx := context.Background()

	route1, err := tp.PlanRoute(ctx, NATNone, NATNone, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("first PlanRoute failed: %v", err)
	}

	route2, err := tp.PlanRoute(ctx, NATSymmetric, NATSymmetric, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("second PlanRoute failed: %v", err)
	}

	if route1 != route2 {
		t.Error("expected same route for same pair")
	}

	if route2.Mode != ModeDirect {
		t.Errorf("expected ModeDirect from cached route, got %s", route2.Mode)
	}
}

func TestTopologyPlanner_RemoveRoute(t *testing.T) {
	tp := NewTopologyPlanner("stun.example.com:3478", "", nil)
	ctx := context.Background()

	_, err := tp.PlanRoute(ctx, NATUnknown, NATUnknown, "10.0.0.1:41641", "10.0.0.2:41642")
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}

	key := routeKey("10.0.0.1:41641", "10.0.0.2:41642")

	_, ok := tp.GetRoute(key)
	if !ok {
		t.Fatal("route should exist before removal")
	}

	tp.RemoveRoute(key)

	_, ok = tp.GetRoute(key)
	if ok {
		t.Error("route should not exist after removal")
	}
}

func TestRouteKey_Symmetric(t *testing.T) {
	k1 := routeKey("A", "B")
	k2 := routeKey("B", "A")

	if k1 != k2 {
		t.Errorf("routeKey should be symmetric: %q != %q", k1, k2)
	}
}
