package chain_watcher

import (
	"context"
	"time"

	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/log"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	registry "blazar/internal/pkg/upgrades_registry"
)

type UpgradeProposalsWatcher struct {
	ur       *registry.UpgradeRegistry
	interval time.Duration
	Errors   <-chan error
	cancel   chan<- struct{}
}

func NewUpgradeProposalsWatcher(ctx context.Context, cosmosClient *cosmos.Client, ur *registry.UpgradeRegistry, proposalInterval time.Duration) *UpgradeProposalsWatcher {
	ticker := time.NewTicker(proposalInterval)
	errors := make(chan error)
	cancel := make(chan struct{})

	logger := log.FromContext(ctx).With("package", "upgrade_proposals_watcher")
	lastUpdateTime := time.Now()

	go func() {
		for {
			select {
			case <-ticker.C:
				// we don't want to stop the watcher if there is an error
				// since it could be a temporary error, like a network issue
				// therefore we return the error to the channel and continue
				logger.Infof("Attempting to fetch upgrade proposals, last attempt was %f seconds ago", time.Since(lastUpdateTime).Seconds())
				lastUpdateTime = time.Now()

				lastHeight, err := cosmosClient.GetLatestBlockHeight(ctx)
				if err != nil {
					select {
					case errors <- err:
					// to prevent deadlock with errors channel
					case <-cancel:
						return
					}
					continue
				}

				logger.Infof("Fetching the upgrade proposals at height %d", lastHeight)
				_, _, upgrades, _, err := ur.Update(ctx, lastHeight, true)
				if err != nil {
					select {
					case errors <- err:
					// to prevent deadlock with errors channel
					case <-cancel:
						return
					}
					continue
				}

				upcomingUpgrades := ur.GetUpcomingUpgradesWithCache(lastHeight, urproto.UpgradeStatus_ACTIVE)
				logger.Infof(
					"Fetched %d upcoming upgrades in ACTIVE state, out of total %d resolved ones, next attempt in %f seconds",
					len(upcomingUpgrades), len(upgrades), proposalInterval.Seconds(),
				)

			// we want to cancel the watcher when the chain upgrade is under progress
			case <-cancel:
				return
			}
		}
	}()
	return &UpgradeProposalsWatcher{
		ur:       ur,
		cancel:   cancel,
		interval: proposalInterval,
		Errors:   errors,
	}
}

func (upw *UpgradeProposalsWatcher) GetInterval() time.Duration {
	return upw.interval
}

func (upw *UpgradeProposalsWatcher) Cancel() {
	upw.cancel <- struct{}{}
}
