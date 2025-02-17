package chain_watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/file_watcher"
	"blazar/internal/pkg/log"

	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

type UpgradesInfoWatcher struct {
	// full path to a watched file
	filename string
	lastInfo upgradetypes.Plan

	Upgrades <-chan NewUpgradeInfo
}

type NewUpgradeInfo struct {
	Plan  upgradetypes.Plan
	Error error
}

func NewUpgradeInfoWatcher(ctx context.Context, upgradeInfoFilePath string, interval time.Duration) (*UpgradesInfoWatcher, error) {
	logger := log.FromContext(ctx)
	exists, fw, err := file_watcher.NewFileWatcher(logger, upgradeInfoFilePath, interval)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating file watcher for %s", upgradeInfoFilePath)
	}

	// Default to empty plan(height = 0) if file doesn't exist
	// Any upgrade will have height > 0
	var info upgradetypes.Plan
	if exists {
		info, err = parseUpgradeInfoFile(upgradeInfoFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse upgrade-info.json file")
		}
	}

	upgrades := make(chan NewUpgradeInfo)

	uiw := &UpgradesInfoWatcher{
		filename: upgradeInfoFilePath,
		lastInfo: info,
		Upgrades: upgrades,
	}

	go func() {
		for {
			newEvent := <-fw.ChangeEvents
			if newEvent.Error != nil {
				panic(errors.Wrapf(newEvent.Error, "upgrade info watcher's file watcher observed an error"))
			}
			logger.Debugf("Upgrade info watcher observed a file event: %v", newEvent.Event)

			if e := newEvent.Event; e == file_watcher.FileCreated || e == file_watcher.FileModified {
				upgrade, err := uiw.checkIfUpdateIsNeeded(logger)

				// we don't want to stop the watcher if there is an error here
				// since it could be a temporary error
				// eg: file created but not written to yet
				// send the error to the channel for logging and continue
				// we can export these errors as metrics later
				var newUpgradeInfo NewUpgradeInfo
				if err != nil {
					logger.Debugf("Upgrade info watcher observed an error: %v", err)
					newUpgradeInfo.Error = err
					upgrades <- newUpgradeInfo
				} else if upgrade != nil {
					logger.Debugf("Upgrade info watcher observed an upgrade: %+v", *upgrade)
					newUpgradeInfo.Plan = *upgrade
					upgrades <- newUpgradeInfo
					fw.Cancel()
					return
				}
			}
		}
	}()

	return uiw, nil
}

// checkIfUpdateIsNeeded reads update plan from upgrade-info.json
// and returns the plan, if a new upgrade height has been hit
func (uiw *UpgradesInfoWatcher) checkIfUpdateIsNeeded(logger *log.MultiLogger) (*upgradetypes.Plan, error) {
	info, err := parseUpgradeInfoFile(uiw.filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse upgrade-info.json file")
	}

	logger.Debugf("Upgrade info watcher parsed upgrade-info.json: %+v", info)

	// Lets check if the upgrade plan height
	// is not equal to that what we last knew
	//
	// This breaks down in one edge case:
	// Lets say the chain hits an upgrade at height 1000
	// and the upgrade-info.json file is created
	// Whether the upgrade was successful or not that doesn't matter.
	// But for some reason the chain is restored to some height
	// lower than 1000, say 900, without touching the upgrade-info.json
	// file. Now when the upgrade block
	// height is hit again, the upgrade will not be detected.
	//
	// The modTime of upgrade-info.json may change due to various
	// reasons, hence it can't be used as a reliable check for
	// an upgrade height being hit. Unfortunately, there is no
	// way to detect this edge case, we can at most add a
	// warning in the README.
	if info.Height != uiw.lastInfo.Height {
		uiw.lastInfo = info
		return &info, nil
	}

	return nil, nil
}

func parseUpgradeInfoFile(filename string) (upgradetypes.Plan, error) {
	var ui upgradetypes.Plan

	f, err := os.Open(filename)
	if err != nil {
		return upgradetypes.Plan{}, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	if err := d.Decode(&ui); err != nil {
		return upgradetypes.Plan{}, err
	}

	// required values must be set
	if ui.Height <= 0 || ui.Name == "" {
		return upgradetypes.Plan{}, fmt.Errorf("invalid upgrade-info.json content; name and height must be not empty; got: %v", ui)
	}

	return ui, err
}
