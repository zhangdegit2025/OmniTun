package omnitun

import (
	"context"
	"fmt"
)

type DomainsService struct {
	client *Client
}

func (s *DomainsService) Create(ctx context.Context, opts CreateDomainOptions) (*Domain, error) {
	var result Domain
	if err := s.client.post(ctx, "/v1/domains", opts, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DomainsService) Get(ctx context.Context, domainID string) (*Domain, error) {
	var result Domain
	path := fmt.Sprintf("/v1/domains/%s", domainID)
	if err := s.client.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DomainsService) List(ctx context.Context) (*PaginatedList[Domain], error) {
	var result PaginatedList[Domain]
	if err := s.client.get(ctx, "/v1/domains", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DomainsService) Delete(ctx context.Context, domainID string) error {
	path := fmt.Sprintf("/v1/domains/%s", domainID)
	return s.client.delete(ctx, path, nil)
}
