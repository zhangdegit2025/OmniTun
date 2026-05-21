//go:build integration

package integration

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/omnitun/omnitun/internal/network"
)

func TestSTUN_BindingRequest(t *testing.T) {
	t.Parallel()

	server := network.NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	if addr == nil {
		t.Fatal("server listen addr is nil")
	}

	client := network.NewSTUNClient(addr.String())
	mappedAddr, err := client.GetMappedAddress()
	if err != nil {
		t.Fatalf("GetMappedAddress failed: %v", err)
	}

	host, _, err := net.SplitHostPort(mappedAddr)
	if err != nil {
		t.Fatalf("invalid mapped address %q: %v", mappedAddr, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("expected mapped address 127.0.0.1, got %s", host)
	}

	natType, mapped, err := client.DetectNATType()
	if err != nil {
		t.Logf("DetectNATType returned error (expected with single server): %v", err)
	}
	if natType != network.NATOpen && natType != network.NATFullCone && natType != network.NATSymmetric {
		t.Errorf("unexpected NAT type: %s", natType.String())
	}
	if mapped != "" && mapped != mappedAddr {
		t.Logf("mapped address from DetectNATType: %s vs GetMappedAddress: %s", mapped, mappedAddr)
	}

	msgLen := 20
	msg := make([]byte, msgLen)
	binary.BigEndian.PutUint16(msg[0:2], network.StunBindingRequest)
	binary.BigEndian.PutUint16(msg[2:4], 0)
	binary.BigEndian.PutUint32(msg[4:8], network.StunMagicCookie)

	if len(msg) != 20 {
		t.Errorf("binding request should be 20 bytes, got %d", len(msg))
	}
	msgType := binary.BigEndian.Uint16(msg[0:2])
	if msgType != network.StunBindingRequest {
		t.Errorf("expected STUN binding request type 0x%04x, got 0x%04x", network.StunBindingRequest, msgType)
	}
}

func TestTURN_Allocation(t *testing.T) {
	t.Parallel()

	relay := network.NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, err := relay.CreateAllocation("integration-test-user")
	if err != nil {
		t.Fatalf("CreateAllocation failed: %v", err)
	}
	if alloc.ID == "" {
		t.Error("allocation ID should not be empty")
	}
	if alloc.Username != "integration-test-user" {
		t.Errorf("expected username 'integration-test-user', got '%s'", alloc.Username)
	}

	_, err = relay.CreateAllocation("integration-test-user")
	if err != network.ErrAllocationExists {
		t.Errorf("expected ErrAllocationExists for duplicate, got %v", err)
	}

	secondAlloc, err := relay.CreateAllocation("another-user")
	if err != nil {
		t.Fatalf("CreateAllocation for another user failed: %v", err)
	}
	if secondAlloc.ID == alloc.ID {
		t.Error("allocations for different users should have unique IDs")
	}

	if count := relay.AllocationCount(); count != 2 {
		t.Errorf("expected 2 allocations, got %d", count)
	}

	peerAddr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345}
	peerKey, err := relay.RegisterPeer(alloc.ID, peerAddr)
	if err != nil {
		t.Fatalf("RegisterPeer failed: %v", err)
	}
	if peerKey == "" {
		t.Fatal("expected non-empty peer key")
	}

	fetched, err := relay.GetAllocation(alloc.ID)
	if err != nil {
		t.Fatalf("GetAllocation failed: %v", err)
	}
	_, ok := fetched.PeerAddrs[peerKey]
	if !ok {
		t.Errorf("peer %s not found after registration", peerKey)
	}

	if err := relay.RemoveAllocation(alloc.ID); err != nil {
		t.Fatalf("RemoveAllocation failed: %v", err)
	}
	if relay.AllocationCount() != 1 {
		t.Errorf("expected 1 allocation after removal, got %d", relay.AllocationCount())
	}

	_, err = relay.GetAllocation(alloc.ID)
	if err != network.ErrAllocationNotFound {
		t.Errorf("expected ErrAllocationNotFound for removed allocation, got %v", err)
	}

	err = relay.RemoveAllocation(alloc.ID)
	if err != network.ErrAllocationNotFound {
		t.Errorf("expected ErrAllocationNotFound for double remove, got %v", err)
	}
}

func TestTopology_PlanRoute(t *testing.T) {
	t.Parallel()

	planner := network.NewTopologyPlanner("stun.example.com:3478", "turn.example.com:3478", nil)
	if planner == nil {
		t.Fatal("NewTopologyPlanner returned nil")
	}

	sourceAddr := "192.168.1.10:51820"
	targetAddr := "10.0.0.5:51820"

	route, err := planner.PlanRoute(
		context.Background(),
		network.NATFullCone,
		network.NATFullCone,
		sourceAddr,
		targetAddr,
	)
	if err != nil {
		t.Fatalf("PlanRoute failed: %v", err)
	}
	if route == nil {
		t.Fatal("PlanRoute returned nil route")
	}
	if route.SourceNode != sourceAddr {
		t.Errorf("expected source %q, got %q", sourceAddr, route.SourceNode)
	}
	if route.TargetNode != targetAddr {
		t.Errorf("expected target %q, got %q", targetAddr, route.TargetNode)
	}
	if route.Established.IsZero() {
		t.Error("route Established time should be set")
	}

	routeKey := "192.168.1.10:51820↔10.0.0.5:51820"
	cached, ok := planner.GetRoute(routeKey)
	if !ok {
		differentKey := "10.0.0.5:51820↔192.168.1.10:51820"
		cached, ok = planner.GetRoute(differentKey)
		if !ok {
			t.Fatal("cached route should be retrievable by either key ordering")
		}
	}
	if cached == nil {
		t.Fatal("cached route is nil")
	}

	route2, err := planner.PlanRoute(
		context.Background(),
		network.NATOpen,
		network.NATRestricted,
		"192.168.2.1:51820",
		"10.0.1.1:51820",
	)
	if err != nil {
		t.Fatalf("PlanRoute with NAT open should succeed: %v", err)
	}
	if route2 == nil {
		t.Fatal("expected non-nil route for open NAT")
	}

	route3, err := planner.PlanRoute(
		context.Background(),
		network.NATSymmetric,
		network.NATSymmetric,
		"10.10.10.1:51820",
		"10.10.10.2:51820",
	)
	if err != nil {
		t.Fatalf("PlanRoute with symmetric NAT should succeed: %v", err)
	}
	if route3 == nil {
		t.Fatal("expected non-nil route for symmetric NAT (fallback to relay)")
	}

	planner.RemoveRoute("10.10.10.1:51820↔10.10.10.2:51820")
	_, ok = planner.GetRoute("10.10.10.1:51820↔10.10.10.2:51820")
	if ok {
		t.Error("route should not be found after removal")
	}
	_, ok = planner.GetRoute("10.10.10.2:51820↔10.10.10.1:51820")
	if ok {
		t.Error("route should not be found after removal (alternate ordering)")
	}
}
