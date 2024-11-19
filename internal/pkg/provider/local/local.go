package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"

	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/provider"
	sm "blazar/internal/pkg/state_machine"

	"google.golang.org/protobuf/encoding/protojson"
)

type localProviderData struct {
	Upgrades []*urproto.Upgrade `json:"upgrades"`
	Versions []*vrproto.Version `json:"versions"`
	State    *sm.State          `json:"state"`
}

var JSONMarshaller = protojson.MarshalOptions{
	Multiline: true,
	Indent:    "    ",
}

type Provider struct {
	configPath string
	network    string
	priority   int32
	lock       *sync.RWMutex
}

func NewProvider(configPath, network string, priority int32) (*Provider, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		d := localProviderData{}
		jsonData, err := json.Marshal(&d)
		if err != nil {
			return nil, errors.Wrapf(err, "could not marshal local provider data into json")
		}
		if err := os.WriteFile(configPath, jsonData, 0600); err != nil {
			return nil, errors.Wrapf(err, "could not create local provider data file")
		}
	}

	ur := &Provider{
		configPath: configPath,
		network:    network,
		priority:   priority,
		lock:       &sync.RWMutex{},
	}

	return ur, nil
}

func (lp *Provider) GetUpgrades(_ context.Context) ([]*urproto.Upgrade, error) {
	data, err := lp.readData(true)
	if err != nil {
		return nil, err
	}

	return provider.PostProcessUpgrades(data.Upgrades, urproto.ProviderType_LOCAL, lp.priority), nil
}

func (lp *Provider) GetUpgradesByHeight(ctx context.Context, height int64) ([]*urproto.Upgrade, error) {
	upgrades, err := lp.GetUpgrades(ctx)
	if err != nil {
		return []*urproto.Upgrade{}, err
	}

	filtered := make([]*urproto.Upgrade, 0, len(upgrades))
	for _, upgrade := range upgrades {
		if upgrade.Height == height {
			filtered = append(filtered, upgrade)
		}
	}

	return provider.PostProcessUpgrades(filtered, urproto.ProviderType_LOCAL, lp.priority), nil
}

func (lp *Provider) GetUpgradesByType(ctx context.Context, upgradeType urproto.UpgradeType) ([]*urproto.Upgrade, error) {
	upgrades, err := lp.GetUpgrades(ctx)
	if err != nil {
		return []*urproto.Upgrade{}, err
	}

	filtered := make([]*urproto.Upgrade, 0, len(upgrades))
	for _, upgrade := range upgrades {
		if upgrade.Type == upgradeType {
			filtered = append(filtered, upgrade)
		}
	}

	return provider.PostProcessUpgrades(filtered, urproto.ProviderType_LOCAL, lp.priority), nil
}

func (lp *Provider) AddUpgrade(_ context.Context, upgrade *urproto.Upgrade, overwrite bool) error {
	provider.PostProcessUpgrade(upgrade, urproto.ProviderType_LOCAL, lp.priority)

	lp.lock.Lock()
	defer lp.lock.Unlock()

	if upgrade.Network != lp.network {
		return fmt.Errorf("upgrade network %s does not match %s", upgrade.Network, lp.network)
	}

	data, err := lp.readData(false)
	if err != nil {
		return err
	}
	upgrades := data.Upgrades

	for n, existingUpgrade := range upgrades {
		if existingUpgrade.Height == upgrade.Height && existingUpgrade.Priority == upgrade.Priority {
			if !overwrite {
				return fmt.Errorf("upgrade for height %d and priority %d already registered", upgrade.Height, upgrade.Priority)
			}
			upgrades = slices.Delete(upgrades, n, n+1)
			break
		}
	}

	upgrades = append(upgrades, upgrade)
	data.Upgrades = upgrades

	jsonData, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	return os.WriteFile(lp.configPath, jsonData, 0600)
}

func (lp *Provider) RegisterVersion(_ context.Context, version *vrproto.Version, overwrite bool) error {
	provider.PostProcessVersion(version, urproto.ProviderType_LOCAL, lp.priority)

	lp.lock.Lock()
	defer lp.lock.Unlock()

	if version.Network != lp.network {
		return fmt.Errorf("version network %s does not match %s", version.Network, lp.network)
	}

	data, err := lp.readData(false)
	if err != nil {
		return err
	}
	versions := data.Versions

	for n, existingVersion := range versions {
		if existingVersion.Height == version.Height && existingVersion.Priority == version.Priority {
			if !overwrite {
				return fmt.Errorf("version for height=%d, priority=%d already registered", version.Height, version.Priority)
			}
			versions = slices.Delete(versions, n, n+1)
			break
		}
	}

	versions = append(versions, version)
	data.Versions = versions

	jsonData, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	return os.WriteFile(lp.configPath, jsonData, 0600)
}

func (lp *Provider) GetVersions(_ context.Context) ([]*vrproto.Version, error) {
	data, err := lp.readData(true)
	if err != nil {
		return nil, err
	}

	return provider.PostProcessVersions(data.Versions, urproto.ProviderType_LOCAL, lp.priority), nil
}

func (lp *Provider) GetVersionsByHeight(ctx context.Context, height uint64) ([]*vrproto.Version, error) {
	versions, err := lp.GetVersions(ctx)
	if err != nil {
		return nil, err
	}

	filtered := []*vrproto.Version{}
	for _, version := range versions {
		// #nosec G115
		if version.Height == int64(height) {
			filtered = append(filtered, version)
		}
	}

	return provider.PostProcessVersions(filtered, urproto.ProviderType_LOCAL, lp.priority), nil
}

func (lp *Provider) StoreState(state *sm.State) error {
	lp.lock.Lock()
	defer lp.lock.Unlock()

	data, err := lp.readData(false)
	if err != nil {
		return err
	}
	data.State = state

	jsonData, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	return os.WriteFile(lp.configPath, jsonData, 0600)
}

func (lp *Provider) checkUniqueKey(data *localProviderData) error {
	type uniqueKey struct {
		height   int64
		priority int32
	}

	heightPrioritySet := make(map[uniqueKey]struct{})

	for _, upgrade := range data.Upgrades {
		if _, ok := heightPrioritySet[uniqueKey{height: upgrade.Height, priority: upgrade.Priority}]; ok {
			return fmt.Errorf("found multiple upgrades for height=%d, priority=%d", upgrade.Height, upgrade.Priority)
		}
		heightPrioritySet[uniqueKey{height: upgrade.Height, priority: upgrade.Priority}] = struct{}{}
	}

	heightPrioritySet = make(map[uniqueKey]struct{})

	for _, version := range data.Versions {
		if _, ok := heightPrioritySet[uniqueKey{height: version.Height, priority: version.Priority}]; ok {
			return fmt.Errorf("found multiple versions for height=%d, priority=%d", version.Height, version.Priority)
		}
		heightPrioritySet[uniqueKey{height: version.Height, priority: version.Priority}] = struct{}{}
	}

	for _, version := range data.Versions {
		if version.Network != lp.network {
			return fmt.Errorf("network %s does not match configured network %s", version.Network, lp.network)
		}
	}

	for _, upgrade := range data.Upgrades {
		if upgrade.Network != lp.network {
			return fmt.Errorf("network %s does not match configured network %s", upgrade.Network, lp.network)
		}
	}
	return nil
}

func (lp *Provider) RestoreState() (*sm.State, error) {
	data, err := lp.readData(true)
	if err != nil {
		return nil, err
	}

	if err := lp.checkUniqueKey(data); err != nil {
		return nil, err
	}

	return data.State, nil
}

func (lp *Provider) CancelUpgrade(_ context.Context, height int64, network string) error {
	if network != lp.network {
		return fmt.Errorf("the network %s does not match local provider: %s", network, lp.network)
	}

	lp.lock.Lock()
	defer lp.lock.Unlock()

	data, err := lp.readData(false)
	if err != nil {
		return err
	}
	upgrades := data.Upgrades

	upgradeWithHighestPriority, pos := &urproto.Upgrade{Priority: 0}, 0
	for n, existingUpgrade := range upgrades {
		if existingUpgrade.Height == height && existingUpgrade.Priority > upgradeWithHighestPriority.Priority {
			upgradeWithHighestPriority, pos = existingUpgrade, n
		}
	}

	if upgradeWithHighestPriority.Priority == 0 {
		// if there is no upgrades registered (in local provider) blazar will create one with status CANCELLED
		cancellationUpgrade := &urproto.Upgrade{
			Height:     height,
			Network:    network,
			Priority:   lp.priority,
			Name:       "",
			Type:       urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED,
			Status:     urproto.UpgradeStatus_CANCELLED,
			Step:       urproto.UpgradeStep_NONE,
			Source:     urproto.ProviderType_DATABASE,
			ProposalId: nil,
		}

		upgrades = append(upgrades, cancellationUpgrade)
		data.Upgrades = upgrades
	} else {
		// if there is an upgrade with the same height and priority, blazar will cancel it
		upgrades[pos].Status = urproto.UpgradeStatus_CANCELLED
		data.Upgrades = upgrades
	}

	jsonData, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	return os.WriteFile(lp.configPath, jsonData, 0600)
}

func (lp *Provider) Type() urproto.ProviderType {
	return urproto.ProviderType_LOCAL
}

func (lp *Provider) readData(lock bool) (*localProviderData, error) {
	if lock {
		lp.lock.RLock()
		defer lp.lock.RUnlock()
	}

	var localData localProviderData

	fileData, err := os.ReadFile(lp.configPath)
	if os.IsNotExist(err) {
		jsonData, err := json.Marshal(&localData)
		if err != nil {
			return nil, errors.Wrapf(err, "could not marshal new upgrades file to protobuf")
		}

		if err := os.WriteFile(lp.configPath, jsonData, 0600); err != nil {
			return nil, errors.Wrapf(err, "could not create new upgrades file")
		}
		return &localData, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "could not read %s upgrades file", lp.configPath)
	}

	if err := json.Unmarshal(fileData, &localData); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal %s upgrades file", lp.configPath)
	}

	return &localData, nil
}
