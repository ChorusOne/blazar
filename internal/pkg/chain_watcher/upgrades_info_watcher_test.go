package chain_watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/testutils"

	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUpgradeInfoFile(t *testing.T) {
	cases := []struct {
		filename      string
		expectUpgrade upgradetypes.Plan
		expectErr     bool
	}{
		{
			filename:      "f1-good.json",
			expectUpgrade: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123},
			expectErr:     false,
		},
		{
			filename:      "f2-normalized-name.json",
			expectUpgrade: upgradetypes.Plan{Name: "Upgrade2", Info: "some info", Height: 125},
			expectErr:     false,
		},
		{
			filename:      "f2-bad-type.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f2-bad-type-2.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f3-empty.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f4-empty-obj.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f5-partial-obj-1.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f5-partial-obj-2.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "unknown.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.filename, func(t *testing.T) {
			ui, err := parseUpgradeInfoFile(filepath.Join(testutils.TestdataDirPath, "upgrade-files", tc.filename))
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectUpgrade, ui)
			}
		})
	}
}

func TestMonitorUpgrade(t *testing.T) {
	t.Run("NoExistingFile", func(t *testing.T) {
		chainHome := t.TempDir()
		err := os.MkdirAll(filepath.Join(chainHome, "data"), 0755)
		require.NoError(t, err)

		cfg := &config.Config{
			ChainHome: chainHome,
			Watchers: config.Watchers{
				UIInterval: 5 * time.Millisecond,
			},
		}
		uiw, err := NewUpgradeInfoWatcher(cfg.UpgradeInfoFilePath(), cfg.Watchers.UIInterval)
		require.NoError(t, err)

		go func() {
			time.Sleep(10 * time.Millisecond)
			testutils.MustCopy(t, "upgrade-files/f1-good.json", cfg.UpgradeInfoFilePath())
		}()

		upgrade := <-uiw.Upgrades
		require.NoError(t, upgrade.Error)
		assert.Equal(t, upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123}, upgrade.Plan)
	})

	t.Run("UpgradesFileExists", func(t *testing.T) {
		chainHome := t.TempDir()
		err := os.MkdirAll(filepath.Join(chainHome, "data"), 0755)
		require.NoError(t, err)

		cfg := &config.Config{
			ChainHome: chainHome,
			Watchers: config.Watchers{
				UIInterval: 5 * time.Millisecond,
			},
		}
		// copy an upgrade file before creating the watcher
		testutils.MustCopy(t, "upgrade-files/f1-good.json", cfg.UpgradeInfoFilePath())

		uiw, err := NewUpgradeInfoWatcher(cfg.UpgradeInfoFilePath(), cfg.Watchers.UIInterval)
		require.NoError(t, err)

		// the old upgrade-info.json file should be loaded
		assert.Equal(t, upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123}, uiw.lastInfo)

		go func() {
			time.Sleep(10 * time.Millisecond)
			testutils.MustCopy(t, "upgrade-files/f2-normalized-name.json", cfg.UpgradeInfoFilePath())
		}()

		upgrade := <-uiw.Upgrades
		require.NoError(t, upgrade.Error)
		assert.Equal(t, upgradetypes.Plan{Name: "Upgrade2", Info: "some info", Height: 125}, upgrade.Plan)
	})
}
