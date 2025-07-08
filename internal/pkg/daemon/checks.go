package daemon

import (
	"context"
	"slices"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/daemon/checks"
	"blazar/internal/pkg/docker"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"
	"blazar/internal/pkg/log/notification"
	checksproto "blazar/internal/pkg/proto/daemon"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"blazar/internal/pkg/state_machine"
)

func (d *Daemon) preUpgradeChecks(
	ctx context.Context,
	currHeight int64,
	sm *state_machine.StateMachine,
	dcc *docker.ComposeClient,
	composeConfig *config.ComposeCli,
	cfg *config.PreUpgrade,
	serviceName string,
	upgrade *urproto.Upgrade,
	networkName string,
) (int64, error) {
	ctx = notification.WithUpgradeHeight(ctx, upgrade.Height)
	logger := log.FromContext(ctx)

	currStep := sm.GetStep(upgrade.Height)
	if !(currStep == urproto.UpgradeStep_MONITORING || currStep == urproto.UpgradeStep_PRE_UPGRADE_CHECK) {
		return 0, nil
	}

	// notify once about the upcoming upgrade
	if currStep == urproto.UpgradeStep_MONITORING {
		logger.Infof(
			"Detected upcoming upgrade (type: %s, tag: %s, chain: %s) Current height: %d, upgrade height: %d",
			upgrade.Type, upgrade.Tag, networkName, currHeight, upgrade.Height,
		).Notify(ctx)

		if len(cfg.Enabled) == 0 {
			logger.Info("No pre upgrade checks configured, skipping").Notify(ctx)
		} else {
			logger.Infof("Running pre upgrade checks: %v", cfg.Enabled).Notify(ctx)
		}
	}

	if currStep != urproto.UpgradeStep_PRE_UPGRADE_CHECK {
		// NOTE: a failed pre-check doesn't mean the upgrade is not possible in a future. Current strategy for blazar is to:
		// 1. Notify user on pre-check failure
		// 2. Keep the upgrade status ACTIVE
		// 3. When the upgrade hits, blazar will attempt to upgrade the service
		//
		// At the time of writing this comment, the only retriable pre-check is the docker image fetch,
		// which is retried during the upgrade phase.
		// Not setting FAILED status here is fine.
		//
		// Example: Pre-check failed because network operator didn't register an image tag yet. He'll do it in next 100 blocks,
		// but if the status is FAILED the operator can't do anyting.
		d.MustSetStatusAndStep(upgrade.Height, urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStep_PRE_UPGRADE_CHECK)
	}

	// no need to run checks if none are enabled
	if len(cfg.Enabled) == 0 {
		return 0, nil
	}

	if slices.Contains(cfg.Enabled, checksproto.PreCheck_PULL_DOCKER_IMAGE.String()) {
		status := sm.GetPreCheckStatus(upgrade.Height, checksproto.PreCheck_PULL_DOCKER_IMAGE)
		if status != checksproto.CheckStatus_FINISHED {
			d.SetPreCheckStatus(upgrade.Height, checksproto.PreCheck_PULL_DOCKER_IMAGE, checksproto.CheckStatus_RUNNING)

			logger.Infof(
				"Pre upgrade check: %s Checking if upgrade tag %s is available",
				checksproto.PreCheck_PULL_DOCKER_IMAGE.String(), upgrade.Tag,
			).Notify(ctx)

			_, newImage, err := checks.PullDockerImage(ctx, d.dcc, serviceName, upgrade.Tag, upgrade.Height,
				cfg.PullDockerImage.MaxRetries, cfg.PullDockerImage.InitialBackoff)
			d.reportPreUpgradeRoutine(ctx, upgrade, newImage, err)

			d.SetPreCheckStatus(upgrade.Height, checksproto.PreCheck_PULL_DOCKER_IMAGE, checksproto.CheckStatus_FINISHED)
		}
	}

	if slices.Contains(cfg.Enabled, checksproto.PreCheck_SET_HALT_HEIGHT.String()) {
		status := sm.GetPreCheckStatus(upgrade.Height, checksproto.PreCheck_SET_HALT_HEIGHT)
		shouldRun := upgrade.Height <= currHeight+(cfg.Blocks-cfg.SetHaltHeight.DelayBlocks)

		if shouldRun && status != checksproto.CheckStatus_FINISHED {
			if upgrade.Type == urproto.UpgradeType_NON_GOVERNANCE_COORDINATED {
				d.SetPreCheckStatus(upgrade.Height, checksproto.PreCheck_SET_HALT_HEIGHT, checksproto.CheckStatus_RUNNING)

				logger.Infof(
					"Pre upgrade step: %s restarting daemon with halt-height %d",
					checksproto.PreCheck_SET_HALT_HEIGHT.String(), upgrade.Height,
				).Notify(ctx)

				err := dcc.RestartServiceWithHaltHeight(ctx, composeConfig, serviceName, upgrade.Height)
				d.reportPreUpgradeHaltHeight(ctx, upgrade, err)
			} else {
				logger.Infof(
					"Pre upgrade step: %s restarting daemon with halt-height skipped, as the upgrade is not %s",
					checksproto.PreCheck_SET_HALT_HEIGHT.String(), urproto.UpgradeType_NON_GOVERNANCE_COORDINATED.String(),
				).Notify(ctx)
			}

			d.SetPreCheckStatus(upgrade.Height, checksproto.PreCheck_SET_HALT_HEIGHT, checksproto.CheckStatus_FINISHED)
		}

		// When the halt height env was set the node will stop itself at the upgrade height
		// The trick is that blazar won't receive the block at the upgrade height, because the node will shutdown (depends on the cosmos-sdk version)
		// Instead we are waiting for the block prior to the upgrade height and then try to assert if the node is still running
		if upgrade.Type == urproto.UpgradeType_NON_GOVERNANCE_COORDINATED && status == checksproto.CheckStatus_FINISHED && currHeight == upgrade.Height-1 {
			ticker := time.NewTicker(time.Second)
			start := time.Now()
			countSameUpgradeHeights, countSameUpgradePlusHeights := 0, 0

			logger.Infof("Got block %d, waiting for the service to stop itself due to active halt-height setting", currHeight).Notify(ctx)

			for range ticker.C {
				logger.Info("Checking if the service has stopped itself")

				isRunning, err := dcc.IsServiceRunning(ctx, serviceName, 5*time.Second)
				if err != nil {
					return 0, err
				}

				if isRunning {
					logger.Infof("Service is still running, waiting for the service to stop itself")
					if lastHeight, err := d.cosmosClient.GetLatestBlockHeight(ctx); err == nil {
						// some cosmos-sdk versions will HALT_HEIGHT at a specified height, but in fact the next block is going to be committed
						// this has been fixed but we support this behavior for backward compatibility
						if lastHeight == upgrade.Height {
							countSameUpgradeHeights++
						} else if lastHeight == upgrade.Height+1 {
							countSameUpgradePlusHeights++
						}
					}

					// depending on the cosmos-sdk version the HALT_HEIGHT will either exit the node or throw panic and wait
					// if we can get the upgrade block 3 times from the endpoint then we are likely in that condition
					//
					// why 5 times? Most of the cosmos sdk chains won't have higher block times than 5 seconds
					//
					// TODO: In future versions there is and endpoint `/cosmos/base/node/v1beta1/config` which returns the `halt-height`
					if countSameUpgradeHeights > 5 || countSameUpgradePlusHeights > 5 {
						logger.Warn("HALT_HEIGHT likely worked but didn't shut down the node, continuing").Notify(ctx)
						return upgrade.Height, nil
					}

					if time.Since(start) > 2*time.Minute {
						err := errors.New("The service didn't stop itself after 2 minutes")
						logger.Err(err).Error("Pre upgrade step: SET_HALT_HEIGHT failed").Notify(ctx)
						return 0, err
					}
					continue
				}

				logger.Info("The service has stopped itself, continuing with the upgrade")
				return upgrade.Height, nil
			}
		}
	}

	return 0, nil
}

func (d *Daemon) reportPreUpgradeHaltHeight(ctx context.Context, upgrade *urproto.Upgrade, err error) {
	ctx = notification.WithUpgradeHeight(ctx, upgrade.Height)
	logger := log.FromContext(ctx)

	if err != nil {
		logger.Err(err).Warnf("Error setting halt height. Node will not stop itself at %d, requiring manual action", upgrade.Height).Notify(ctx)
	} else {
		logger.Infof("Halt-height has been set to %d, node will stop itself when it is time to upgrade", upgrade.Height).Notify(ctx)
	}
}

func (d *Daemon) reportPreUpgradeRoutine(ctx context.Context, upgrade *urproto.Upgrade, newImage string, err error) {
	ctx = notification.WithUpgradeHeight(ctx, upgrade.Height)
	logger := log.FromContext(ctx)

	if err != nil {
		msg := "Error performing pre upgrade check. I'll not be able to perform the upgrade, please "
		if upgrade.Tag == "" {
			msg += "register the image tag"
		} else {
			msg += "check why the image is not available on the host"
		}
		logger.Err(err).Warn(msg).Notify(ctx)
	} else {
		logger.Infof("Upgrade image: %s\nI'll attempt to upgrade when upgrade height is hit", newImage).Notify(ctx)
	}
}

func (d *Daemon) postUpgradeChecks(ctx context.Context, sm *state_machine.StateMachine, cfg *config.PostUpgrade, upgradeHeight int64) (err error) {
	defer func() {
		// ensure we update the status to failed if any error was encountered
		if err != nil {
			d.MustSetStatus(upgradeHeight, urproto.UpgradeStatus_FAILED)
		}
	}()
	ctx = notification.WithUpgradeHeight(ctx, upgradeHeight)
	logger := log.FromContext(ctx)

	currStep := sm.GetStep(upgradeHeight)
	if !(currStep == urproto.UpgradeStep_COMPOSE_FILE_UPGRADE || currStep == urproto.UpgradeStep_POST_UPGRADE_CHECK) {
		return nil
	}

	// notify once about the post checks
	if currStep == urproto.UpgradeStep_COMPOSE_FILE_UPGRADE {
		if len(cfg.Enabled) == 0 {
			logger.Info("No post upgrade checks configured, skipping").Notify(ctx)
		} else {
			logger.Infof("Running post upgrade checks: %v", cfg.Enabled).Notify(ctx)
		}
	}

	if currStep != urproto.UpgradeStep_POST_UPGRADE_CHECK {
		d.SetStep(upgradeHeight, urproto.UpgradeStep_POST_UPGRADE_CHECK)
	}

	// no need to run checks if none are enabled
	if len(cfg.Enabled) == 0 {
		return nil
	}

	if slices.Contains(cfg.Enabled, checksproto.PostCheck_GRPC_RESPONSIVE.String()) {
		status := sm.GetPostCheckStatus(upgradeHeight, checksproto.PostCheck_GRPC_RESPONSIVE)
		if status != checksproto.CheckStatus_FINISHED {
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_GRPC_RESPONSIVE, checksproto.CheckStatus_RUNNING)

			logger.Infof("Post upgrade check: %s Waiting for the grpc and cometbft services to be responsive", checksproto.PostCheck_GRPC_RESPONSIVE.String()).Notify(ctx)

			_, err = checks.GrpcResponsive(ctx, d.cosmosClient, cfg.GrpcResponsive)
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_GRPC_RESPONSIVE, checksproto.CheckStatus_FINISHED)

			if err != nil {
				return errors.Wrapf(err, "post upgrade grpc-endpoint-response check failed")
			}
		}
	}

	if slices.Contains(cfg.Enabled, checksproto.PostCheck_FIRST_BLOCK_VOTED.String()) {
		status := sm.GetPostCheckStatus(upgradeHeight, checksproto.PostCheck_FIRST_BLOCK_VOTED)
		if status != checksproto.CheckStatus_FINISHED {
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_FIRST_BLOCK_VOTED, checksproto.CheckStatus_RUNNING)

			logger.Infof("Post upgrade check: %s Waiting for the on-chain block at upgrade height=%d to be signed by us", checksproto.PostCheck_FIRST_BLOCK_VOTED.String(), upgradeHeight).Notify(ctx)

			err = checks.NextBlockSignedPostCheck(ctx, d.cosmosClient, cfg.FirstBlockVoted, upgradeHeight)
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_FIRST_BLOCK_VOTED, checksproto.CheckStatus_FINISHED)

			if err != nil {
				return errors.Wrapf(err, "post upgrade upgrade-block-signed check failed")
			}
		}
	}

	if slices.Contains(cfg.Enabled, checksproto.PostCheck_CHAIN_HEIGHT_INCREASED.String()) {
		status := sm.GetPostCheckStatus(upgradeHeight, checksproto.PostCheck_CHAIN_HEIGHT_INCREASED)
		if status != checksproto.CheckStatus_FINISHED {
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_CHAIN_HEIGHT_INCREASED, checksproto.CheckStatus_RUNNING)

			logger.Infof(
				"Post upgrade check: %s Waiting for the on-chain latest block height to be > upgrade height=%d",
				checksproto.PostCheck_CHAIN_HEIGHT_INCREASED.String(), upgradeHeight,
			).Notify(ctx)

			err = checks.ChainHeightIncreased(ctx, d.cosmosClient, cfg.ChainHeightIncreased, upgradeHeight)
			d.SetPostCheckStatus(upgradeHeight, checksproto.PostCheck_CHAIN_HEIGHT_INCREASED, checksproto.CheckStatus_FINISHED)

			if err != nil {
				return errors.Wrapf(err, "post upgrade next-block-height check failed")
			}
		}
	}
	return nil
}
