package upgrades_registry

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"testing"

	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/provider"
	"blazar/internal/pkg/provider/database"
	"blazar/internal/pkg/provider/local"
	"blazar/internal/pkg/testutils"

	sm "blazar/internal/pkg/state_machine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func addDummyLocalProvider(t *testing.T, ur *UpgradeRegistry) {
	_, blazarDir := testutils.NewChainHomeDir(t)
	var err error
	ur.providers[urproto.ProviderType_LOCAL], err = local.NewProvider(
		blazarDir+"/local.db.json",
		"test",
		1,
	)
	if err != nil {
		t.Fatalf("failed to create local provider: %v", err)
	}
}

func prepareMockDatabaseProvider() (*database.Provider, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect database")
	}
	err = db.AutoMigrate(&urproto.Upgrade{})
	if err != nil {
		return nil, errors.Wrapf(err, "database migration failed for upgrades table")
	}

	err = db.AutoMigrate(&vrproto.Version{})
	if err != nil {
		return nil, errors.Wrapf(err, "database migration failed for versions table")
	}
	return database.NewDatabaseProviderWithDB(db, "test", 1), nil
}

func addDummyDatabaseProvider(t *testing.T, ur *UpgradeRegistry) {
	var err error
	ur.providers[urproto.ProviderType_DATABASE], err = prepareMockDatabaseProvider()
	if err != nil {
		t.Fatalf("failed to create database provider: %v", err)
	}
}

func resetProviders(t *testing.T, ur *UpgradeRegistry) {
	if _, ok := ur.providers[urproto.ProviderType_LOCAL]; ok {
		addDummyLocalProvider(t, ur)
	}
	if _, ok := ur.providers[urproto.ProviderType_DATABASE]; ok {
		addDummyDatabaseProvider(t, ur)
	}
}

func testProviders(t *testing.T, ur *UpgradeRegistry, source urproto.ProviderType) {
	tests := []struct {
		name     string
		upgrades []*urproto.Upgrade
		testFn   func(*testing.T, *UpgradeRegistry)
	}{
		{
			name: "assert the GetUpcomingUpgrades returns the correct upgrades",
			upgrades: []*urproto.Upgrade{
				{
					Height:  100,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "valid_upcoming_upgrade",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				},
				{
					Height:  101,
					Tag:     "",
					Network: "test",
					// NOTE: the upgrade without a tag should be ignored or fail
					// however this decision is left to the caller. Lack of version tag doesn't
					// mean the upgrade doesn't exist, it just means it's not ready to be applied
					Name:   "valid_upgrade_without_tag",
					Type:   urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status: urproto.UpgradeStatus_UNKNOWN,
					Source: source,
				},
				{
					Height:  102,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "invalid_upcoming_upgrade_due_to_cancelled_status",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_CANCELLED,
					Source:  source,
				},
				{
					Height:  10,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "invalid_upcoming_upgrade_due_to_passed_height",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				},
			},
			testFn: func(t *testing.T, ur *UpgradeRegistry) {
				currentHeight := int64(50)
				upgrades, err := ur.GetUpcomingUpgrades(context.Background(), false, currentHeight, urproto.UpgradeStatus_ACTIVE)
				require.NoError(t, err)

				slices.SortFunc(upgrades, func(i, j *urproto.Upgrade) int {
					return cmp.Compare(i.Height, j.Height)
				})

				assert.Len(t, upgrades, 2)
				assert.Equal(t, int64(100), upgrades[0].Height)
				assert.Equal(t, int64(101), upgrades[1].Height)

				// return all 3 three upcoming upgrades regardless of status
				upgradesNoFilter, err := ur.GetUpcomingUpgrades(context.Background(), false, currentHeight)
				require.NoError(t, err)
				assert.Len(t, upgradesNoFilter, 3)
			},
		},
		{
			name: "adding duplicate upgrade should fail",
			upgrades: []*urproto.Upgrade{
				{
					Height:  100,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "valid_upcoming_upgrade",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				},
			},
			testFn: func(t *testing.T, ur *UpgradeRegistry) {
				err := ur.AddUpgrade(context.Background(), &urproto.Upgrade{
					Height:  100,
					Tag:     "different-tag",
					Network: "test",
					Name:    "valid_upcoming_upgrade",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				}, false)
				assert.Error(t, err)
			},
		},
		{
			name: "cancel upgrade check",
			upgrades: []*urproto.Upgrade{
				{
					Height:  100,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "valid_upcoming_upgrade",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				},
			},
			testFn: func(t *testing.T, ur *UpgradeRegistry) {
				err := ur.CancelUpgrade(context.Background(), 100, source, "test", false)
				require.NoError(t, err)
				upgrade, err := ur.GetUpgrade(context.Background(), false, 100)
				require.NoError(t, err)
				assert.Equal(t, urproto.UpgradeStatus_CANCELLED, upgrade.Status)
				// non existent upgrade should also not fail
				err = ur.CancelUpgrade(context.Background(), 1000000, source, "test", false)
				require.NoError(t, err)
				upgrade, err = ur.GetUpgrade(context.Background(), false, 1000000)
				require.NoError(t, err)
				assert.Equal(t, urproto.UpgradeStatus_CANCELLED, upgrade.Status)
			},
		},
		{
			name: "override upgrade check",
			upgrades: []*urproto.Upgrade{
				{
					Height:  100,
					Tag:     "v1.0.0",
					Network: "test",
					Name:    "valid_upcoming_upgrade",
					Type:    urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:  urproto.UpgradeStatus_UNKNOWN,
					Source:  source,
				},
			},
			testFn: func(t *testing.T, ur *UpgradeRegistry) {
				err := ur.AddUpgrade(context.Background(), &urproto.Upgrade{
					Height:   100,
					Tag:      "different-tag",
					Network:  "test",
					Name:     "valid_upcoming_upgrade",
					Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:   urproto.UpgradeStatus_UNKNOWN,
					Source:   source,
					Priority: 99,
				}, false)
				require.NoError(t, err)
				upgrade, err := ur.GetUpgrade(context.Background(), false, 100)
				require.NoError(t, err)
				assert.Equal(t, "different-tag", upgrade.Tag)
				err = ur.AddUpgrade(context.Background(), &urproto.Upgrade{
					Height:   100,
					Tag:      "another-tag",
					Network:  "test",
					Name:     "valid_upcoming_upgrade",
					Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:   urproto.UpgradeStatus_UNKNOWN,
					Source:   source,
					Priority: 98,
				}, false)
				require.NoError(t, err)
				upgrade, err = ur.GetUpgrade(context.Background(), false, 100)
				require.NoError(t, err)
				assert.Equal(t, "different-tag", upgrade.Tag)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetProviders(t, ur)
			for _, upgrade := range tt.upgrades {
				err := ur.AddUpgrade(context.Background(), upgrade, false)
				if err != nil {
					t.Fatalf("failed to add upgrade: %v", err)
				}
			}

			_, _, _, _, err := ur.Update(context.Background(), 50, true)
			require.NoError(t, err)

			tt.testFn(t, ur)
		})
	}
}

func TestProviders(t *testing.T) {
	ur1 := &UpgradeRegistry{
		providers:    make(map[urproto.ProviderType]provider.UpgradeProvider),
		upgrades:     make(map[int64]*urproto.Upgrade),
		network:      "test",
		lock:         &sync.RWMutex{},
		stateMachine: sm.NewStateMachine(nil),
	}
	addDummyLocalProvider(t, ur1)
	t.Run("TestLocal", func(t *testing.T) {
		testProviders(t, ur1, urproto.ProviderType_LOCAL)
	})

	ur2 := &UpgradeRegistry{
		providers:    make(map[urproto.ProviderType]provider.UpgradeProvider),
		upgrades:     make(map[int64]*urproto.Upgrade),
		network:      "test",
		lock:         &sync.RWMutex{},
		stateMachine: sm.NewStateMachine(nil),
	}
	addDummyDatabaseProvider(t, ur2)
	t.Run("TestDatabase", func(t *testing.T) {
		testProviders(t, ur2, urproto.ProviderType_DATABASE)
	})
}

func TestSimultaneousProviders(t *testing.T) {
	ur := &UpgradeRegistry{
		providers:    make(map[urproto.ProviderType]provider.UpgradeProvider),
		upgrades:     make(map[int64]*urproto.Upgrade),
		network:      "test",
		lock:         &sync.RWMutex{},
		stateMachine: sm.NewStateMachine(nil),
	}
	addDummyLocalProvider(t, ur)
	addDummyDatabaseProvider(t, ur)
	tests := []struct {
		name     string
		upgrades []*urproto.Upgrade
		testFn   func(*testing.T, *UpgradeRegistry)
	}{
		{
			name: "test same priority height different provider",
			upgrades: []*urproto.Upgrade{
				{
					Height:   100,
					Tag:      "v1.0.0",
					Network:  "test",
					Name:     "valid_upcoming_upgrade",
					Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:   urproto.UpgradeStatus_UNKNOWN,
					Source:   urproto.ProviderType_DATABASE,
					Priority: 1,
				},
			},
			testFn: func(t *testing.T, ur *UpgradeRegistry) {
				err := ur.AddUpgrade(context.Background(), &urproto.Upgrade{
					Height:   100,
					Tag:      "different-tag",
					Network:  "test",
					Name:     "different_name",
					Type:     urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					Status:   urproto.UpgradeStatus_UNKNOWN,
					Source:   urproto.ProviderType_LOCAL,
					Priority: 1,
				}, false)
				// TODO: this should ideally error
				require.NoError(t, err)
				assert.PanicsWithError(t, "found objects with the same height=100 and priority=1", func() {
					_, _ = ur.GetAllUpgrades(context.Background(), false)
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetProviders(t, ur)
			for _, upgrade := range tt.upgrades {
				err := ur.AddUpgrade(context.Background(), upgrade, false)
				if err != nil {
					t.Fatalf("failed to add upgrade: %v", err)
				}
			}

			_, _, _, _, err := ur.Update(context.Background(), 50, true)
			require.NoError(t, err)

			tt.testFn(t, ur)
		})
	}
}
