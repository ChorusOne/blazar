package upgrades_registry

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/provider"
	"blazar/internal/pkg/provider/chain"
	"blazar/internal/pkg/provider/database"
	"blazar/internal/pkg/provider/local"
	"blazar/internal/pkg/state_machine"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

type SyncInfo struct {
	LastBlockHeight int64
	LastUpdateTime  time.Time
}

type UpgradeRegistry struct {
	// a list of provider to fetch upgrades from (e.g. chain, database, local)
	providers map[urproto.ProviderType]provider.UpgradeProvider

	// a list provider to fetch versions from (e.g. chain, database, local)
	versionProviders []urproto.ProviderType

	// a state machine containing the current status of all upgrades
	stateMachine *state_machine.StateMachine

	// lock for the registry
	lock *sync.RWMutex

	// a list of latst fetched upgrades
	upgrades map[int64]*urproto.Upgrade

	// a list of versions fetched from providers
	versions map[int64]*vrproto.Version

	// a list of upgrades that were overridden by another upgrade with the same height and higher priority
	overriddenUpgrades map[int64][]*urproto.Upgrade

	// a list of versions that were overridden by another version with the same height and higher priority
	overriddenVersions map[int64][]*vrproto.Version

	// information about the last sync
	syncInfo SyncInfo

	// network for which the registry is created
	network string
}

func NewUpgradeRegistry(providers map[urproto.ProviderType]provider.UpgradeProvider, versionProviders []urproto.ProviderType, stateMachine *state_machine.StateMachine, network string) *UpgradeRegistry {
	return &UpgradeRegistry{
		providers:          providers,
		versionProviders:   versionProviders,
		lock:               &sync.RWMutex{},
		upgrades:           make(map[int64]*urproto.Upgrade, 0),
		versions:           make(map[int64]*vrproto.Version, 0),
		overriddenUpgrades: make(map[int64][]*urproto.Upgrade),
		overriddenVersions: make(map[int64][]*vrproto.Version),
		stateMachine:       stateMachine,
		syncInfo:           SyncInfo{},
		network:            network,
	}
}

func NewUpgradesRegistryFromConfig(cfg *config.Config) (*UpgradeRegistry, error) {
	providers := make(map[urproto.ProviderType]provider.UpgradeProvider, 0)

	if cfg.UpgradeRegistry.Provider.Chain != nil && slices.Contains(
		cfg.UpgradeRegistry.SelectedProviders, urproto.ProviderType_name[int32(urproto.ProviderType_CHAIN)],
	) {
		cosmosClient, err := cosmos.NewClient(cfg.Clients.Host, cfg.Clients.GrpcPort, cfg.Clients.CometbftPort, cfg.Clients.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create cosmos client")
		}

		if err := cosmosClient.StartCometbftClient(); err != nil {
			return nil, errors.Wrapf(err, "failed to start cometbft client")
		}

		provider := chain.NewProvider(cosmosClient, cfg.UpgradeRegistry.Network, cfg.UpgradeRegistry.Provider.Chain.DefaultPriority)
		providers[provider.Type()] = provider
	}

	if cfg.UpgradeRegistry.Provider.Database != nil && slices.Contains(
		cfg.UpgradeRegistry.SelectedProviders, urproto.ProviderType_name[int32(urproto.ProviderType_DATABASE)],
	) {
		provider, err := database.NewDatabaseProvider(
			cfg.UpgradeRegistry.Provider.Database,
			cfg.UpgradeRegistry.Network,
		)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create database provider")
		}
		providers[provider.Type()] = provider
	}

	if cfg.UpgradeRegistry.Provider.Local != nil && slices.Contains(
		cfg.UpgradeRegistry.SelectedProviders, urproto.ProviderType_name[int32(urproto.ProviderType_LOCAL)],
	) {
		provider, err := local.NewProvider(
			cfg.UpgradeRegistry.Provider.Local.ConfigPath,
			cfg.UpgradeRegistry.Network,
			cfg.UpgradeRegistry.Provider.Local.DefaultPriority,
		)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create local provider")
		}
		providers[provider.Type()] = provider
	}

	versionProviders := make([]urproto.ProviderType, 0)
	if cfg.UpgradeRegistry.VersionResolvers != nil {
		for _, providerName := range cfg.UpgradeRegistry.VersionResolvers.Providers {
			providerType := urproto.ProviderType(urproto.ProviderType_value[providerName])
			if _, ok := providers[providerType].(provider.VersionResolver); !ok {
				return nil, fmt.Errorf("version resolver provider %s does not implement VersionResolver interface", providerName)
			}
			versionProviders = append(versionProviders, providerType)
		}
	}

	// handle state machine storage provider
	var stateMachine *state_machine.StateMachine
	if cfg.UpgradeRegistry.StateMachine.Provider != "" {
		if cfg.UpgradeRegistry.StateMachine.Provider != urproto.ProviderType_name[int32(urproto.ProviderType_LOCAL)] {
			return nil, fmt.Errorf("state machine storage provider %s is not supported (only 'local' is supported now)", cfg.UpgradeRegistry.StateMachine.Provider)
		}

		providerType := urproto.ProviderType(urproto.ProviderType_value[cfg.UpgradeRegistry.StateMachine.Provider])
		localProvider := providers[providerType].(*local.Provider)
		stateMachine = state_machine.NewStateMachine(localProvider)
	}

	// state machine without storage provider is okay, everything will be stored in memory
	if stateMachine == nil {
		stateMachine = state_machine.NewStateMachine(nil)
	}

	// TODO: context in constructor aint great
	err := stateMachine.Restore(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to restore state machine")
	}

	return NewUpgradeRegistry(providers, versionProviders, stateMachine, cfg.UpgradeRegistry.Network), nil
}

func (ur *UpgradeRegistry) GetStateMachine() *state_machine.StateMachine {
	return ur.stateMachine
}

func (ur *UpgradeRegistry) GetAllUpgradesWithCache() map[int64]*urproto.Upgrade {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	return copyMap(ur.upgrades)
}

func (ur *UpgradeRegistry) GetAllUpgrades(ctx context.Context, useCache bool) (map[int64]*urproto.Upgrade, error) {
	if useCache {
		return ur.GetAllUpgradesWithCache(), nil
	}

	_, _, resolvedUpgrades, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return resolvedUpgrades, nil
}

func (ur *UpgradeRegistry) GetOverriddenUpgradesWithCache() map[int64][]*urproto.Upgrade {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	return copyMapList(ur.overriddenUpgrades)
}

func (ur *UpgradeRegistry) GetOverriddenUpgrades(ctx context.Context, useCache bool) (map[int64][]*urproto.Upgrade, error) {
	if useCache {
		return ur.GetOverriddenUpgradesWithCache(), nil
	}

	_, _, _, overriddenUpgrades, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return overriddenUpgrades, nil
}

func (ur *UpgradeRegistry) GetUpcomingUpgradesWithCache(height int64, allowedStatus ...urproto.UpgradeStatus) []*urproto.Upgrade {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	upcomingUpgrades := sortAndfilterUpgradesByStatus(ur.upgrades, ur.stateMachine, height, allowedStatus...)

	return copyList(upcomingUpgrades)
}

func (ur *UpgradeRegistry) GetUpcomingUpgrades(ctx context.Context, useCache bool, height int64, allowedStatus ...urproto.UpgradeStatus) (
	[]*urproto.Upgrade,
	error,
) {
	if useCache {
		return ur.GetUpcomingUpgradesWithCache(height, allowedStatus...), nil
	}

	_, _, resolvedUpgrades, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return sortAndfilterUpgradesByStatus(resolvedUpgrades, ur.stateMachine, height, allowedStatus...), nil
}

func (ur *UpgradeRegistry) GetUpgradeWithCache(height int64) *urproto.Upgrade {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	upgrade := filterUpgradesByHeight(ur.upgrades, height)
	if upgrade != nil {
		return proto.Clone(upgrade).(*urproto.Upgrade)
	}
	return nil
}

func (ur *UpgradeRegistry) GetUpgrade(ctx context.Context, useCache bool, height int64) (*urproto.Upgrade, error) {
	if useCache {
		return ur.GetUpgradeWithCache(height), nil
	}

	_, _, resolvedUpgrades, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return filterUpgradesByHeight(resolvedUpgrades, height), nil
}

func (ur *UpgradeRegistry) Update(ctx context.Context, currentHeight int64, commit bool) (
	map[int64]*vrproto.Version,
	map[int64][]*vrproto.Version,
	map[int64]*urproto.Upgrade,
	map[int64][]*urproto.Upgrade,
	error,
) {
	resolvedVersions, overriddenVersions, err := ur.UpdateVersions(ctx, commit)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrapf(err, "failed to update versions")
	}

	resolvedUpgrades, overriddenUpgrades, err := ur.UpdateUpgrades(ctx, currentHeight, resolvedVersions, commit)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrapf(err, "failed to update upgrades")
	}

	ur.lock.Lock()
	defer ur.lock.Unlock()

	ur.syncInfo = SyncInfo{
		LastBlockHeight: currentHeight,
		LastUpdateTime:  time.Now(),
	}

	return resolvedVersions, overriddenVersions, resolvedUpgrades, overriddenUpgrades, nil
}

func (ur *UpgradeRegistry) GetAllVersionsWithCache() map[int64]*vrproto.Version {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	return copyMap(ur.versions)
}

func (ur *UpgradeRegistry) GetAllVersions(ctx context.Context, useCache bool) (map[int64]*vrproto.Version, error) {
	if useCache {
		return ur.GetAllVersionsWithCache(), nil
	}

	resolvedVersions, _, _, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return resolvedVersions, nil
}

func (ur *UpgradeRegistry) GetOverriddenVersionsWithCache() map[int64][]*vrproto.Version {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	return copyMapList(ur.overriddenVersions)
}

func (ur *UpgradeRegistry) GetOverriddenVersions(ctx context.Context, useCache bool) (map[int64][]*vrproto.Version, error) {
	if useCache {
		return ur.GetOverriddenVersionsWithCache(), nil
	}

	_, overriddenVersions, _, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return overriddenVersions, nil
}

func (ur *UpgradeRegistry) GetVersionWithCache(height int64) *vrproto.Version {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	version := filterVersionsByHeight(ur.versions, height)
	if version != nil {
		return proto.Clone(version).(*vrproto.Version)
	}
	return nil
}

func (ur *UpgradeRegistry) GetVersion(ctx context.Context, useCache bool, height int64) (*vrproto.Version, error) {
	if useCache {
		return ur.GetVersionWithCache(height), nil
	}

	resolvedVersions, _, _, _, err := ur.Update(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	return filterVersionsByHeight(resolvedVersions, height), nil
}

func (ur *UpgradeRegistry) UpdateVersions(ctx context.Context, commit bool) (map[int64]*vrproto.Version, map[int64][]*vrproto.Version, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([][]*vrproto.Version, len(ur.versionProviders))

	for i, providerName := range ur.versionProviders {
		// from go 1.22 the copy of the loop variable is not needed anymore
		// https://tip.golang.org/doc/go1.22#language

		g.Go(func() error {
			if provider, ok := ur.providers[providerName].(provider.VersionResolver); ok {
				versions, err := provider.GetVersions(ctx)
				if err != nil {
					return errors.Wrapf(err, "%s provider failed to fetch versions", providerName)
				}

				if err := checkDuplicates(versions, providerName); err != nil {
					return errors.Wrapf(err, "%s version provider returned duplicate versions", providerName)
				}

				results[i] = versions
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	allVersions := make([]*vrproto.Version, 0)
	for _, versions := range results {
		allVersions = append(allVersions, versions...)
	}

	resolvedVersions, overriddenVersions := resolvePriorities(allVersions)

	if commit {
		ur.lock.Lock()
		defer ur.lock.Unlock()

		ur.versions = resolvedVersions
		ur.overriddenVersions = overriddenVersions
	}

	return resolvedVersions, overriddenVersions, nil
}

func (ur *UpgradeRegistry) UpdateUpgrades(ctx context.Context, currentHeight int64, versions map[int64]*vrproto.Version, commit bool) (map[int64]*urproto.Upgrade, map[int64][]*urproto.Upgrade, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([][]*urproto.Upgrade, len(ur.providers))

	i := 0
	for _, provider := range ur.providers {
		// from go 1.22 the copy of the loop variable (provider) is not needed anymore
		// https://tip.golang.org/doc/go1.22#language
		//
		// but the copy of the global variable (i) is still needed
		// https://golang.org/doc/faq#closures_and_goroutines
		ii := i

		g.Go(func() error {
			upgrades, err := provider.GetUpgrades(ctx)
			if err != nil {
				return errors.Wrapf(err, "%s provider failed to fetch upgrades", provider.Type())
			}

			if err := checkDuplicates(upgrades, provider.Type()); err != nil {
				return errors.Wrapf(err, "%s provider returned duplicate upgrades", provider.Type())
			}

			results[ii] = upgrades
			return nil
		})

		i++
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	allUpgrades := make([]*urproto.Upgrade, 0)
	for _, upgrades := range results {
		allUpgrades = append(allUpgrades, upgrades...)
	}

	resolvedUpgrades, overriddenUpgrades := resolvePriorities(allUpgrades)

	// lock just in case the versions map is reference to ur.versions
	ur.lock.RLock()
	for _, upgrade := range resolvedUpgrades {
		// try to resolve version for the upgrade
		if upgrade.Tag == "" {
			if version, ok := versions[upgrade.Height]; ok {
				upgrade.Tag = version.Tag
			}
			// else {
			// TODO: try to resolve version using different methods, RPC, regexes etc
			// }
		}
	}
	ur.lock.RUnlock()

	if commit {
		ur.lock.Lock()
		defer ur.lock.Unlock()
		ur.upgrades = resolvedUpgrades
		ur.overriddenUpgrades = overriddenUpgrades

		// update statuses of all resolved upgrades
		ur.stateMachine.UpdateStatus(currentHeight, ur.upgrades)
	}

	return resolvedUpgrades, overriddenUpgrades, nil
}

func (ur *UpgradeRegistry) RegisterVersion(ctx context.Context, version *vrproto.Version, overwrite bool) error {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	switch version.Source {
	case urproto.ProviderType_CHAIN:
		return errors.New("add upgrade is not supported for chain provider")

	case urproto.ProviderType_DATABASE:
		if p, ok := ur.providers[urproto.ProviderType_DATABASE]; ok {
			return p.(provider.VersionResolver).RegisterVersion(ctx, version, overwrite)
		} else {
			return errors.New("database provider is not configured")
		}

	case urproto.ProviderType_LOCAL:
		if p, ok := ur.providers[urproto.ProviderType_LOCAL]; ok {
			return p.(provider.VersionResolver).RegisterVersion(ctx, version, overwrite)
		} else {
			return errors.New("local provider is not configured")
		}
	}

	return fmt.Errorf("unknown upgrade source %s", version.GetSource().String())
}

func (ur *UpgradeRegistry) CancelUpgrade(ctx context.Context, height int64, source urproto.ProviderType, network string, force bool) error {
	if force {
		if network != ur.network {
			return fmt.Errorf("the network %s does not match the registry network %s", network, ur.network)
		}
		// this is partially true because in this case the provider doesn't matter, but user should be aware this is only cancelled per blazar node and not globally
		if source != urproto.ProviderType_LOCAL {
			return fmt.Errorf("force cancel is only supported for local provider")
		}
		return ur.stateMachine.SetStatus(height, urproto.UpgradeStatus_CANCELLED)
	}

	switch source {
	// cancel only on this blazar instance
	case urproto.ProviderType_LOCAL:
		if p, ok := ur.providers[urproto.ProviderType_LOCAL]; ok {
			return p.CancelUpgrade(ctx, height, network)
		} else {
			return errors.New("local provider is not configured")
		}

	// cancel on all blazar instances
	case urproto.ProviderType_DATABASE:
		if p, ok := ur.providers[urproto.ProviderType_DATABASE]; ok {
			return p.CancelUpgrade(ctx, height, network)
		} else {
			return errors.New("database provider is not configured")
		}

	default:
		return fmt.Errorf("can't cancel upgrade with source %s", source.String())
	}
}

func (ur *UpgradeRegistry) AddUpgrade(ctx context.Context, upgrade *urproto.Upgrade, overwrite bool) error {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	// The use case for cancelled status is for user to create and upgrade with higher proiority to cancel the existing upgrade
	if upgrade.Status != urproto.UpgradeStatus_UNKNOWN && upgrade.Status != urproto.UpgradeStatus_CANCELLED {
		return errors.New("status is not allowed to be set manually")
	}

	if upgrade.Step != urproto.UpgradeStep_NONE {
		return errors.New("step is not allowed to be set manually")
	}

	switch upgrade.Source {
	case urproto.ProviderType_CHAIN:
		return errors.New("add upgrade is not supported for chain provider")

	case urproto.ProviderType_DATABASE:
		if provider, ok := ur.providers[urproto.ProviderType_DATABASE]; ok {
			return provider.AddUpgrade(ctx, upgrade, overwrite)
		} else {
			return errors.New("database provider is not configured")
		}

	case urproto.ProviderType_LOCAL:
		if provider, ok := ur.providers[urproto.ProviderType_LOCAL]; ok {
			return provider.AddUpgrade(ctx, upgrade, overwrite)
		} else {
			return errors.New("local provider is not configured")
		}
	}

	return fmt.Errorf("unknown upgrade source %s", upgrade.Source.String())
}

func (ur *UpgradeRegistry) SyncInfo() SyncInfo {
	ur.lock.RLock()
	defer ur.lock.RUnlock()

	return ur.syncInfo
}

func (ur *UpgradeRegistry) Network() string {
	return ur.network
}

func resolvePriorities[T interface {
	GetPriority() int32
	GetHeight() int64
}](objects []T) (map[int64]T, map[int64][]T) {
	grouppedByHeight := make(map[int64][]T)
	for _, object := range objects {
		grouppedByHeight[object.GetHeight()] = append(grouppedByHeight[object.GetHeight()], object)
	}

	resolvedObjects := make(map[int64]T, 0)
	overriddenObjects := make(map[int64][]T, 0)
	for height, objects := range grouppedByHeight {
		if len(objects) > 1 {
			sort.Slice(objects, func(i, j int) bool {
				if objects[i].GetPriority() == objects[j].GetPriority() {
					panic(fmt.Errorf("found objects with the same height=%d and priority=%d", objects[i].GetHeight(), objects[i].GetPriority()))
				}
				return objects[i].GetPriority() > objects[j].GetPriority()
			})
			overriddenObjects[height] = objects[1:]
		}

		resolvedObjects[objects[0].GetHeight()] = objects[0]
	}

	return resolvedObjects, overriddenObjects
}

// check for duplicate upgrades with the same height and priority
func checkDuplicates[T interface {
	GetPriority() int32
	GetHeight() int64
}](versions []T, providerName urproto.ProviderType) error {
	set := make(map[int64][]T, len(versions))

	for _, version := range versions {
		if _, ok := set[version.GetHeight()]; ok {
			for _, v := range set[version.GetHeight()] {
				if version.GetPriority() == v.GetPriority() {
					return fmt.Errorf("found versions with the same height (%d) and priority (%d) from the same source (%s)", version.GetHeight(), version.GetPriority(), providerName)
				}
			}
		}
		set[version.GetHeight()] = append(set[version.GetHeight()], version)
	}

	return nil
}

func sortAndfilterUpgradesByStatus(upgrades map[int64]*urproto.Upgrade, sm *state_machine.StateMachine, height int64, allowedStatus ...urproto.UpgradeStatus) []*urproto.Upgrade {
	upcomingUpgrades := make([]*urproto.Upgrade, 0)
	for _, upgrade := range upgrades {
		currentStatus := sm.GetStatus(upgrade.Height)
		if upgrade.Height >= height && (len(allowedStatus) == 0 || slices.Contains(allowedStatus, currentStatus)) {
			upcomingUpgrades = append(upcomingUpgrades, upgrade)
		}
	}

	sort.Slice(upcomingUpgrades, func(i, j int) bool {
		return upcomingUpgrades[i].Height < upcomingUpgrades[j].Height
	})

	return upcomingUpgrades
}

func filterUpgradesByHeight(upgrades map[int64]*urproto.Upgrade, height int64) *urproto.Upgrade {
	if upgrade, ok := upgrades[height]; ok {
		return upgrade
	}
	return nil
}

func filterVersionsByHeight(versions map[int64]*vrproto.Version, height int64) *vrproto.Version {
	if version, ok := versions[height]; ok {
		return version
	}
	return nil
}

func copyMap[T proto.Message](m map[int64]T) map[int64]T {
	newMap := make(map[int64]T, len(m))
	for k, v := range m {
		newMap[k] = proto.Clone(v).(T)
	}

	return newMap
}

func copyMapList[T proto.Message](m map[int64][]T) map[int64][]T {
	newMap := make(map[int64][]T, len(m))
	for k, v := range m {
		newMap[k] = make([]T, len(v))
		for n, vv := range v {
			newMap[k][n] = proto.Clone(vv).(T)
		}
	}

	return newMap
}

func copyList[T proto.Message](m []T) []T {
	newMap := make([]T, len(m))
	for n, v := range m {
		newMap[n] = proto.Clone(v).(T)
	}

	return newMap
}
