package local

import (
	"path"
	"sync"
	"testing"

	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoading(t *testing.T) {
	lp := Provider{
		configPath: "../../../../testdata/provider/local/test.json",
		network:    "test",
		priority:   1,
		lock:       &sync.RWMutex{},
	}

	upgrades, err := lp.readData(true)
	require.NoError(t, err)
	shouldBeUpgrades := []*urproto.Upgrade{
		{
			Height:   10,
			Tag:      "v1.0.0",
			Network:  "test",
			Name:     "invalid_upcoming_upgrade_due_to_passed_height",
			Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status:   urproto.UpgradeStatus_UNKNOWN,
			Source:   urproto.ProviderType_LOCAL,
			Priority: 1,
		},
		{
			Height:   100,
			Tag:      "v1.0.0",
			Network:  "test",
			Name:     "valid_upcoming_upgrade",
			Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status:   urproto.UpgradeStatus_UNKNOWN,
			Source:   urproto.ProviderType_LOCAL,
			Priority: 1,
		},
		{
			Height:   101,
			Tag:      "",
			Network:  "test",
			Name:     "valid_upgrade_without_tag",
			Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status:   urproto.UpgradeStatus_UNKNOWN,
			Source:   urproto.ProviderType_LOCAL,
			Priority: 1,
		},
		{
			Height:   102,
			Tag:      "v1.0.0",
			Network:  "test",
			Name:     "invalid_upcoming_upgrade_due_to_cancelled_status",
			Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status:   urproto.UpgradeStatus_CANCELLED,
			Source:   urproto.ProviderType_LOCAL,
			Priority: 1,
		},
	}
	shouldBeVersions := []*vrproto.Version{
		{
			Height:   10,
			Tag:      "a-tag",
			Network:  "test",
			Priority: 1,
			Source:   0,
		},
	}
	assert.Equal(t, len(shouldBeUpgrades), len(upgrades.Upgrades))
	for i := range shouldBeUpgrades {
		assert.EqualExportedValues(t, shouldBeUpgrades[i], upgrades.Upgrades[i])
	}
	assert.Equal(t, len(shouldBeVersions), len(upgrades.Versions))
	for i := range shouldBeVersions {
		assert.EqualExportedValues(t, shouldBeVersions[i], upgrades.Versions[i])
	}
}

func TestNonExistentFile(t *testing.T) {
	dir := t.TempDir()

	lp, err := NewProvider(path.Join(dir, "non-existing.json"), "test", 1)
	require.NoError(t, err)
	data, err := lp.readData(true)
	require.NoError(t, err)
	assert.Empty(t, data.Upgrades)
	assert.Empty(t, data.Versions)
	assert.Nil(t, data.State)
}

func TestLoadFailing(t *testing.T) {
	tests := []struct {
		name string
		file string
		err  string
	}{
		{
			name: "TestDuplicateUpgrades",
			file: "../../../../testdata/provider/local/duplicate-upgrade.json",
			err:  "found multiple upgrades for height=10, priority=1",
		},
		{
			name: "TestDuplicateVersion",
			file: "../../../../testdata/provider/local/duplicate-version.json",
			err:  "found multiple versions for height=10, priority=1",
		},
		{
			name: "TestDifferentNetworkUpgrade",
			file: "../../../../testdata/provider/local/different-upgrade-network.json",
			err:  "network not-test does not match configured network test",
		},
		{
			name: "TestDifferentNetworkVersion",
			file: "../../../../testdata/provider/local/different-version-network.json",
			err:  "network not-test does not match configured network test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := Provider{
				configPath: tt.file,
				network:    "test",
				priority:   1,
				lock:       &sync.RWMutex{},
			}

			_, err := lp.RestoreState()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}
