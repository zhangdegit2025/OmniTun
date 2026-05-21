package omnitun

import (
	"context"
	"fmt"
)

type NetworksService struct {
	client *Client
}

func (s *NetworksService) Create(ctx context.Context, opts CreateNetworkOptions) (*MeshNetwork, error) {
	var result MeshNetwork
	if err := s.client.post(ctx, "/v1/networks", opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *NetworksService) Get(ctx context.Context, networkID string) (*MeshNetwork, error) {
	var result MeshNetwork
	path := fmt.Sprintf("/v1/networks/%s", networkID)
	if err := s.client.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *NetworksService) List(ctx context.Context) (*PaginatedList[MeshNetwork], error) {
	var result PaginatedList[MeshNetwork]
	if err := s.client.get(ctx, "/v1/networks", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *NetworksService) Delete(ctx context.Context, networkID string) error {
	path := fmt.Sprintf("/v1/networks/%s", networkID)
	return s.client.delete(ctx, path, nil)
}

func (s *NetworksService) Join(ctx context.Context, opts JoinNetworkOptions) (*MeshNetwork, error) {
	var result MeshNetwork
	if err := s.client.post(ctx, "/v1/networks/join", opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *NetworksService) Leave(ctx context.Context, networkID string) error {
	path := fmt.Sprintf("/v1/networks/%s/leave", networkID)
	return s.client.post(ctx, path, nil, nil)
}
