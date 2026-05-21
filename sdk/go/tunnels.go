package omnitun

import (
	"context"
	"fmt"
)

type TunnelsService struct {
	client *Client
}

func (s *TunnelsService) Create(ctx context.Context, opts CreateTunnelOptions) (*Tunnel, error) {
	var result Tunnel
	if err := s.client.post(ctx, "/v1/tunnels", opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TunnelsService) Get(ctx context.Context, tunnelID string) (*Tunnel, error) {
	var result Tunnel
	path := fmt.Sprintf("/v1/tunnels/%s", tunnelID)
	if err := s.client.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TunnelsService) List(ctx context.Context, params *ListTunnelsParams) (*PaginatedList[Tunnel], error) {
	var result PaginatedList[Tunnel]
	path := "/v1/tunnels"
	if err := s.client.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TunnelsService) Update(ctx context.Context, tunnelID string, opts UpdateTunnelOptions) (*Tunnel, error) {
	var result Tunnel
	path := fmt.Sprintf("/v1/tunnels/%s", tunnelID)
	if err := s.client.patch(ctx, path, opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TunnelsService) Delete(ctx context.Context, tunnelID string) error {
	path := fmt.Sprintf("/v1/tunnels/%s", tunnelID)
	return s.client.delete(ctx, path, nil)
}

func (s *TunnelsService) Start(ctx context.Context, tunnelID string) (*Tunnel, error) {
	var result Tunnel
	path := fmt.Sprintf("/v1/tunnels/%s/start", tunnelID)
	if err := s.client.post(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TunnelsService) Stop(ctx context.Context, tunnelID string) (*Tunnel, error) {
	var result Tunnel
	path := fmt.Sprintf("/v1/tunnels/%s/stop", tunnelID)
	if err := s.client.post(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
