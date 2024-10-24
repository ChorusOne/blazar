package daemon

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"
	blazarproto "blazar/internal/pkg/proto/blazar"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/upgrades_registry"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	urproto.UnimplementedUpgradeRegistryServer
	vrproto.UnimplementedVersionResolverServer
	blazarproto.UnimplementedBlazarServer

	cfg *config.Config
	ur  *upgrades_registry.UpgradeRegistry
}

// Need to supply logger explicitly because grpc creates its own context
func NewServer(cfg *config.Config, ur *upgrades_registry.UpgradeRegistry) *Server {
	return &Server{
		cfg: cfg,
		ur:  ur,
	}
}

func (s *Server) AddUpgrade(ctx context.Context, in *urproto.AddUpgradeRequest) (*urproto.AddUpgradeResponse, error) {
	if in == nil || in.Upgrade == nil {
		return nil, status.Errorf(codes.Internal, "request is empty")
	}

	in.Upgrade.Tag = strings.TrimSpace(in.Upgrade.Tag)
	in.Upgrade.Network = s.cfg.UpgradeRegistry.Network

	err := s.ur.AddUpgrade(ctx, in.Upgrade, in.GetOverwrite())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add upgrade: %v", err)
	}

	// It is confusing for users having to wait for the upgrades list to refresh in X seconds, so we force update here
	if _, err := s.forceUpdate(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to force update: %v", err)
	}

	return &urproto.AddUpgradeResponse{}, nil
}

func (s *Server) CancelUpgrade(ctx context.Context, in *urproto.CancelUpgradeRequest) (*urproto.CancelUpgradeResponse, error) {
	if in == nil || in.Height == 0 {
		return nil, status.Errorf(codes.Internal, "request is empty")
	}

	err := s.ur.CancelUpgrade(ctx, in.Height, in.Source, s.cfg.UpgradeRegistry.Network, in.Force)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel upgrade: %v", err)
	}

	// It is confusing for users having to wait for the upgrades list to refresh in X seconds, so we force update here
	if _, err := s.forceUpdate(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to force update: %v", err)
	}

	return &urproto.CancelUpgradeResponse{}, nil
}

func (s *Server) ListUpgrades(ctx context.Context, in *urproto.ListUpgradesRequest) (*urproto.ListUpgradesResponse, error) {
	all, err := s.ur.GetAllUpgrades(ctx, !in.GetDisableCache())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list upgrades: %v", err)
	}

	stateMachine := s.ur.GetStateMachine()
	upgrades := []*urproto.Upgrade{}
	for _, upgrade := range all {
		// filter out upgrades that don't match the request
		if in.Height != nil && in.GetHeight() != upgrade.Height {
			continue
		}
		if in.Type != nil && in.GetType() != upgrade.Type {
			continue
		}
		if in.Source != nil && in.GetSource() != upgrade.Source {
			continue
		}

		upgrade.Status = stateMachine.GetStatus(upgrade.Height)
		upgrade.Step = stateMachine.GetStep(upgrade.Height)

		if len(in.Status) > 0 && !slices.Contains(in.Status, upgrade.Status) {
			continue
		}

		upgrades = append(upgrades, upgrade)
	}

	slices.SortFunc(upgrades, func(i, j *urproto.Upgrade) int {
		return cmp.Compare(j.Height, i.Height)
	})

	if in.Limit != nil && int64(len(upgrades)) > *in.Limit {
		upgrades = upgrades[:*in.Limit]
	}

	return &urproto.ListUpgradesResponse{Upgrades: upgrades}, nil
}

func (s *Server) AddVersion(ctx context.Context, in *vrproto.RegisterVersionRequest) (*vrproto.RegisterVersionResponse, error) {
	if in == nil || in.Version == nil {
		return nil, status.Errorf(codes.Internal, "request is empty")
	}

	in.Version.Network = s.cfg.UpgradeRegistry.Network
	in.Version.Tag = strings.TrimSpace(in.Version.Tag)

	err := s.ur.RegisterVersion(ctx, in.Version, in.GetOverwrite())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register version: %v", err)
	}

	// It is confusing for users having to wait for the upgrades list to refresh in X seconds, so we force update here
	if _, err := s.forceUpdate(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to force update: %v", err)
	}

	return &vrproto.RegisterVersionResponse{}, nil
}

func (s *Server) ListVersions(ctx context.Context, in *vrproto.ListVersionsRequest) (*vrproto.ListVersionsResponse, error) {
	all, err := s.ur.GetAllVersions(ctx, !in.GetDisableCache())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list versions: %v", err)
	}

	versions := []*vrproto.Version{}
	for _, version := range all {
		// filter out versions that don't match the request
		if in.Height != nil && in.GetHeight() != version.Height {
			continue
		}
		if in.Source != nil && in.GetSource() != version.Source {
			continue
		}

		versions = append(versions, version)
	}

	slices.SortFunc(versions, func(i, j *vrproto.Version) int {
		return cmp.Compare(i.Height, j.Height)
	})

	return &vrproto.ListVersionsResponse{Versions: versions}, nil
}

func (s *Server) GetVersion(ctx context.Context, in *vrproto.GetVersionRequest) (*vrproto.GetVersionResponse, error) {
	version, err := s.ur.GetVersion(ctx, !in.GetDisableCache(), in.Height)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get version: %v", err)
	}

	return &vrproto.GetVersionResponse{
		Version: version,
	}, nil
}

func (s *Server) GetLastestHeight(ctx context.Context, _ *blazarproto.GetLatestHeightRequest) (*blazarproto.GetLatestHeightResponse, error) {
	cosmosClient, err := cosmos.NewClient(s.cfg.Clients.Host, s.cfg.Clients.GrpcPort, s.cfg.Clients.CometbftPort, s.cfg.Clients.Timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cosmos client")
	}

	if err := cosmosClient.StartCometbftClient(); err != nil {
		return nil, errors.Wrapf(err, "failed to start cometbft client")
	}

	height, err := cosmosClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get latest block height")
	}

	// TODO: we should add some sanity checks here to make sure the returned height is not stale (due to the node not being synced etc)
	return &blazarproto.GetLatestHeightResponse{
		Height:  height,
		Network: s.cfg.UpgradeRegistry.Network,
	}, nil
}

func (s *Server) ForceSync(ctx context.Context, _ *urproto.ForceSyncRequest) (*urproto.ForceSyncResponse, error) {
	syncHeight, err := s.forceUpdate(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to force update: %v", err)
	}

	return &urproto.ForceSyncResponse{
		Height: syncHeight,
	}, nil
}

func (s *Server) forceUpdate(ctx context.Context) (int64, error) {
	cosmosClient, err := cosmos.NewClient(s.cfg.Clients.Host, s.cfg.Clients.GrpcPort, s.cfg.Clients.CometbftPort, s.cfg.Clients.Timeout)
	if err != nil {
		return 0, err
	}

	lastHeight, err := cosmosClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return 0, err
	}

	_, _, _, _, err = s.ur.Update(ctx, lastHeight, true)
	return lastHeight, err
}
