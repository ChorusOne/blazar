package provider

import (
	"context"

	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
)

// VersionResolver is an interface for fetching versions from an external source
type VersionResolver interface {
	RegisterVersion(ctx context.Context, version *vrproto.Version, overwrite bool) error
	GetVersions(ctx context.Context) ([]*vrproto.Version, error)
	GetVersionsByHeight(ctx context.Context, height uint64) ([]*vrproto.Version, error)
}

// UpgradesProvider is an interface for fetching upgrades from an external source
type UpgradeProvider interface {
	GetUpgrades(ctx context.Context) ([]*urproto.Upgrade, error)
	GetUpgradesByType(ctx context.Context, upgradeType urproto.UpgradeType) ([]*urproto.Upgrade, error)
	GetUpgradesByHeight(ctx context.Context, height int64) ([]*urproto.Upgrade, error)
	AddUpgrade(ctx context.Context, upgrade *urproto.Upgrade, overwrite bool) error
	CancelUpgrade(ctx context.Context, height int64, network string) error
	Type() urproto.ProviderType
}

func PostProcessUpgrades(upgrades []*urproto.Upgrade, source urproto.ProviderType, priority int32) []*urproto.Upgrade {
	for n := range upgrades {
		PostProcessUpgrade(upgrades[n], source, priority)
	}

	return upgrades
}

func PostProcessUpgrade(upgrade *urproto.Upgrade, source urproto.ProviderType, priority int32) {
	if upgrade.Source != source {
		upgrade.Source = source
	}

	if upgrade.Priority == 0 {
		upgrade.Priority = priority
	}
}

func PostProcessVersions(versions []*vrproto.Version, source urproto.ProviderType, priority int32) []*vrproto.Version {
	for n := range versions {
		PostProcessVersion(versions[n], source, priority)
	}

	return versions
}

func PostProcessVersion(version *vrproto.Version, source urproto.ProviderType, priority int32) {
	if version.Source != source {
		version.Source = source
	}

	if version.Priority == 0 {
		version.Priority = priority
	}
}
