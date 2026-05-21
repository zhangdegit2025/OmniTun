package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

type RelaySelector struct {
	repo Repository
}

func NewRelaySelector(repo Repository) *RelaySelector {
	return &RelaySelector{repo: repo}
}

func (s *RelaySelector) Select(ctx context.Context, preferredRegion string) (*RelayNode, error) {
	relays, err := s.repo.GetActiveRelays(ctx)
	if err != nil {
		return nil, fmt.Errorf("relay selector: get active relays: %w", err)
	}

	var candidates []*RelayNode
	for _, r := range relays {
		if r.ActiveTunnels < r.Capacity {
			candidates = append(candidates, r)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("relay selector: no available relay nodes")
	}

	sort.Slice(candidates, func(i, j int) bool {
		ri, rj := candidates[i], candidates[j]
		riMatch := ri.Region == preferredRegion
		rjMatch := rj.Region == preferredRegion
		if riMatch != rjMatch {
			return riMatch
		}
		riRatio := float64(ri.ActiveTunnels) / float64(ri.Capacity)
		rjRatio := float64(rj.ActiveTunnels) / float64(rj.Capacity)
		return riRatio < rjRatio
	})

	selected := candidates[0]
	slog.Info("relay selected",
		"relay_id", selected.ID,
		"region", selected.Region,
		"active_tunnels", selected.ActiveTunnels,
		"capacity", selected.Capacity,
	)
	return selected, nil
}

func (s *RelaySelector) SelectAlternative(ctx context.Context, region string, failedRelayID string) (*RelayNode, error) {
	relays, err := s.repo.GetActiveRelays(ctx)
	if err != nil {
		return nil, fmt.Errorf("relay selector: get active relays: %w", err)
	}

	var candidates []*RelayNode
	for _, r := range relays {
		if r.ID == failedRelayID {
			continue
		}
		if r.Status != "online" {
			continue
		}
		if r.Capacity == 0 {
			continue
		}
		if region != "" && r.Region != region {
			continue
		}
		if r.ActiveTunnels >= r.Capacity {
			continue
		}
		candidates = append(candidates, r)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("relay selector: no alternative relay nodes available in region %s (excluded %s)", region, failedRelayID)
	}

	sort.Slice(candidates, func(i, j int) bool {
		riRatio := float64(candidates[i].ActiveTunnels) / float64(candidates[i].Capacity)
		rjRatio := float64(candidates[j].ActiveTunnels) / float64(candidates[j].Capacity)
		return riRatio < rjRatio
	})

	selected := candidates[0]
	slog.Info("alternative relay selected",
		"relay_id", selected.ID,
		"region", selected.Region,
		"active_tunnels", selected.ActiveTunnels,
		"capacity", selected.Capacity,
		"failed_relay_id", failedRelayID,
	)
	return selected, nil
}
