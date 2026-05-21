package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultMaxTunnelsPerWorkspace = 25
	defaultListLimit              = 20
	maxListLimit                  = 100
	slugLength                    = 8
)

var (
	errTunnelNotFound   = status.Error(codes.NotFound, "tunnel not found")
	errInvalidStatus    = status.Error(codes.FailedPrecondition, "invalid tunnel status for this operation")
	errQuotaExceeded    = status.Error(codes.ResourceExhausted, "tunnel quota exceeded for workspace")
	errNoRelayAvailable = status.Error(codes.Unavailable, "no relay nodes available")
)

type Service struct {
	omnitunv1.UnimplementedTunnelServiceServer
	repo         Repository
	relaySel     *RelaySelector
	eventBus     EventPublisher
	connCounter  ConnectionCounter
	quotaChecker QuotaChecker
}

func NewService(repo Repository, relaySel *RelaySelector, eventBus EventPublisher) *Service {
	return &Service{
		repo:     repo,
		relaySel: relaySel,
		eventBus: eventBus,
	}
}

func (s *Service) WithConnectionCounter(cc ConnectionCounter) *Service {
	s.connCounter = cc
	return s
}

func (s *Service) WithQuotaChecker(qc QuotaChecker) *Service {
	s.quotaChecker = qc
	return s
}

func (s *Service) CreateTunnel(ctx context.Context, req *omnitunv1.CreateTunnelRequest) (*omnitunv1.CreateTunnelResponse, error) {
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Protocol == "" {
		return nil, status.Error(codes.InvalidArgument, "protocol is required")
	}

	if err := s.checkQuota(ctx, req.WorkspaceId); err != nil {
		return nil, err
	}

	slug, err := generateSlug()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate slug: %v", err)
	}

	relay, err := s.relaySel.Select(ctx, "")
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "select relay: %v", err)
	}

	now := time.Now().UTC()
	t := &Tunnel{
		ID:           "", // populated by PG (UUID)
		Name:         req.Name,
		Slug:         slug,
		WorkspaceID:  req.WorkspaceId,
		Protocol:     req.Protocol,
		LocalPort:    int(req.LocalPort),
		LocalHost:    req.LocalHost,
		CustomDomain: req.CustomDomain,
		TLSMode:      req.TlsMode,
		AuthMode:     req.AuthMode,
		Status:       StatusStopped,
		RelayID:      relay.ID,
		Region:       relay.Region,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if t.LocalHost == "" {
		t.LocalHost = "127.0.0.1"
	}
	if t.TLSMode == "" {
		t.TLSMode = "edge"
	}
	if t.AuthMode == "" {
		t.AuthMode = "none"
	}

	if err := s.repo.CreateTunnel(ctx, t); err != nil {
		return nil, status.Errorf(codes.Internal, "create tunnel: %v", err)
	}

	TunnelsCreatedTotal.Inc()

	if err := s.eventBus.PublishTunnelEvent(ctx, "tunnel.created", t); err != nil {
		slog.Warn("failed to publish tunnel.created event", "error", err)
	}

	slog.Info("tunnel created",
		"tunnel_id", t.ID,
		"name", t.Name,
		"slug", t.Slug,
		"workspace_id", t.WorkspaceID,
	)

	return &omnitunv1.CreateTunnelResponse{
		Tunnel: toProtoTunnel(t),
	}, nil
}

func (s *Service) GetTunnel(ctx context.Context, req *omnitunv1.GetTunnelRequest) (*omnitunv1.GetTunnelResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		slog.Error("get tunnel failed", "tunnel_id", req.TunnelId, "error", err)
		return nil, errTunnelNotFound
	}

	return &omnitunv1.GetTunnelResponse{
		Tunnel: toProtoTunnel(t),
	}, nil
}

func (s *Service) ListTunnels(ctx context.Context, req *omnitunv1.ListTunnelsRequest) (*omnitunv1.ListTunnelsResponse, error) {
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	tunnels, nextCursor, err := s.repo.ListTunnels(ctx, req.WorkspaceId, limit, req.Cursor)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tunnels: %v", err)
	}

	protoTunnels := make([]*omnitunv1.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		protoTunnels = append(protoTunnels, toProtoTunnel(t))
	}

	hasMore := nextCursor != ""

	return &omnitunv1.ListTunnelsResponse{
		Tunnels: protoTunnels,
		Pagination: &omnitunv1.PaginationResponse{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *Service) UpdateTunnel(ctx context.Context, req *omnitunv1.UpdateTunnelRequest) (*omnitunv1.UpdateTunnelResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		return nil, errTunnelNotFound
	}

	updated := false
	if req.Name != nil {
		t.Name = *req.Name
		updated = true
	}
	if req.AuthMode != nil {
		t.AuthMode = *req.AuthMode
		updated = true
	}
	if req.MaxConnections != nil {
		t.MaxConnections = int(*req.MaxConnections)
		updated = true
	}

	if !updated {
		return &omnitunv1.UpdateTunnelResponse{Tunnel: toProtoTunnel(t)}, nil
	}

	t.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateTunnel(ctx, t); err != nil {
		return nil, status.Errorf(codes.Internal, "update tunnel: %v", err)
	}

	if err := s.eventBus.PublishTunnelEvent(ctx, "tunnel.updated", t); err != nil {
		slog.Warn("failed to publish tunnel.updated event", "error", err)
	}

	slog.Info("tunnel updated", "tunnel_id", t.ID, "name", t.Name)

	return &omnitunv1.UpdateTunnelResponse{
		Tunnel: toProtoTunnel(t),
	}, nil
}

func (s *Service) DeleteTunnel(ctx context.Context, req *omnitunv1.DeleteTunnelRequest) (*omnitunv1.DeleteTunnelResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		return nil, errTunnelNotFound
	}

	if err := s.repo.DeleteTunnel(ctx, req.TunnelId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete tunnel: %v", err)
	}

	if err := s.eventBus.PublishTunnelEvent(ctx, "tunnel.deleted", t); err != nil {
		slog.Warn("failed to publish tunnel.deleted event", "error", err)
	}

	slog.Info("tunnel deleted", "tunnel_id", req.TunnelId)

	return &omnitunv1.DeleteTunnelResponse{
		Message: "tunnel deleted",
	}, nil
}

func (s *Service) StartTunnel(ctx context.Context, req *omnitunv1.StartTunnelRequest) (*omnitunv1.StartTunnelResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	startTime := time.Now()

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		return nil, errTunnelNotFound
	}

	if t.Status != StatusStopped {
		return nil, status.Errorf(codes.FailedPrecondition,
			"tunnel status is %s, must be %s to start", t.Status, StatusStopped)
	}

	if err := s.repo.UpdateTunnelStatus(ctx, req.TunnelId, StatusStarting); err != nil {
		return nil, status.Errorf(codes.Internal, "update tunnel status: %v", err)
	}

	if err := s.eventBus.PublishTunnelEvent(ctx, "agent.command.start", t); err != nil {
		slog.Warn("failed to publish agent.command.start event", "error", err)
	}

	relayAddress := ""
	if t.RelayID != "" {
		relay, err := s.repo.GetRelayNode(ctx, t.RelayID)
		if err != nil {
			slog.Warn("failed to resolve relay address", "relay_id", t.RelayID, "error", err)
		} else {
			relayAddress = fmt.Sprintf("%s:%d", relay.Hostname, relay.Port)
		}
	}

	TunnelStartDuration.Observe(time.Since(startTime).Seconds())

	slog.Info("tunnel start initiated", "tunnel_id", req.TunnelId, "relay_address", relayAddress)

	return &omnitunv1.StartTunnelResponse{
		Message:      "tunnel start initiated",
		RelayAddress: relayAddress,
	}, nil
}

func (s *Service) StopTunnel(ctx context.Context, req *omnitunv1.StopTunnelRequest) (*omnitunv1.StopTunnelResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		return nil, errTunnelNotFound
	}

	if t.Status != StatusActive {
		return nil, status.Errorf(codes.FailedPrecondition,
			"tunnel status is %s, must be %s to stop", t.Status, StatusActive)
	}

	if err := s.repo.UpdateTunnelStatus(ctx, req.TunnelId, StatusStopped); err != nil {
		return nil, status.Errorf(codes.Internal, "update tunnel status: %v", err)
	}

	if err := s.eventBus.PublishTunnelEvent(ctx, "agent.command.stop", t); err != nil {
		slog.Warn("failed to publish agent.command.stop event", "error", err)
	}

	slog.Info("tunnel stop initiated", "tunnel_id", req.TunnelId)

	return &omnitunv1.StopTunnelResponse{
		Message: "tunnel stop initiated",
	}, nil
}

func (s *Service) GetTunnelStats(ctx context.Context, req *omnitunv1.GetTunnelStatsRequest) (*omnitunv1.GetTunnelStatsResponse, error) {
	if req.TunnelId == "" {
		return nil, status.Error(codes.InvalidArgument, "tunnel_id is required")
	}

	t, err := s.repo.GetTunnel(ctx, req.TunnelId)
	if err != nil {
		return nil, errTunnelNotFound
	}

	var activeConns int32
	if s.connCounter != nil {
		activeConns, _ = s.connCounter.GetActiveConnections(ctx, req.TunnelId)
	}

	return &omnitunv1.GetTunnelStatsResponse{
		Stats: &omnitunv1.TunnelStats{
			BytesIn:           t.BytesInTotal,
			BytesOut:          t.BytesOutTotal,
			ActiveConnections: activeConns,
		},
	}, nil
}

func (s *Service) checkQuota(ctx context.Context, workspaceID string) error {
	count, err := s.repo.CountTunnelsByWorkspace(ctx, workspaceID)
	if err != nil {
		return status.Errorf(codes.Internal, "count tunnels: %v", err)
	}

	maxTunnels := defaultMaxTunnelsPerWorkspace
	if s.quotaChecker != nil {
		quotaMax, err := s.quotaChecker.GetMaxTunnels(ctx, workspaceID)
		if err != nil {
			slog.Warn("failed to get quota limit, using default", "error", err, "workspace_id", workspaceID)
		} else if quotaMax > 0 {
			maxTunnels = quotaMax
		}
	}

	if count >= maxTunnels {
		slog.Warn("tunnel quota exceeded",
			"workspace_id", workspaceID,
			"current", count,
			"max", maxTunnels,
		)
		return status.Errorf(codes.ResourceExhausted,
			"workspace has %d tunnels (limit: %d)", count, maxTunnels)
	}

	return nil
}

func toProtoTunnel(t *Tunnel) *omnitunv1.Tunnel {
	pt := &omnitunv1.Tunnel{
		Id:             t.ID,
		OrganizationId: t.OrganizationID,
		WorkspaceId:    t.WorkspaceID,
		Name:           t.Name,
		Slug:           t.Slug,
		Protocol:       t.Protocol,
		LocalPort:      int32(t.LocalPort),
		LocalHost:      t.LocalHost,
		CustomDomain:   t.CustomDomain,
		TlsMode:        t.TLSMode,
		AuthMode:       t.AuthMode,
		Status:         string(t.Status),
		BytesInTotal:   t.BytesInTotal,
		BytesOutTotal:  t.BytesOutTotal,
	}

	if !t.CreatedAt.IsZero() {
		pt.CreatedAt = timestamppb.New(t.CreatedAt)
	}
	if !t.UpdatedAt.IsZero() {
		pt.UpdatedAt = timestamppb.New(t.UpdatedAt)
	}

	return pt
}

func generateSlug() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, slugLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("generate slug: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func cursorEncode(t time.Time) string {
	return base64.URLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano)))
}

func cursorDecode(cursor string) (time.Time, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, fmt.Errorf("decode cursor: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cursor time: %w", err)
	}
	return t, nil
}
