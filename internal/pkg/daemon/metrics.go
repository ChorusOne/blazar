package daemon

import (
	checksproto "blazar/internal/pkg/proto/daemon"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"strconv"
)

func (d *Daemon) MustSetStatus(height int64, status urproto.UpgradeStatus) {
	d.stateMachine.MustSetStatus(height, status)
	d.updateMetrics()
}

func (d *Daemon) SetStep(height int64, step urproto.UpgradeStep) {
	d.stateMachine.SetStep(height, step)
	d.updateMetrics()
}

func (d *Daemon) MustSetStatusAndStep(height int64, status urproto.UpgradeStatus, step urproto.UpgradeStep) {
	d.stateMachine.MustSetStatusAndStep(height, status, step)
	d.updateMetrics()
}

func (d *Daemon) SetPreCheckStatus(height int64, check checksproto.PreCheck, status checksproto.CheckStatus) {
	d.stateMachine.SetPreCheckStatus(height, check, status)
	d.updateMetrics()
}

func (d *Daemon) SetPostCheckStatus(height int64, check checksproto.PostCheck, status checksproto.CheckStatus) {
	d.stateMachine.SetPostCheckStatus(height, check, status)
	d.updateMetrics()
}

func (d *Daemon) updateMetrics() {
	// the upgrade state may change, we don't want to persist the metric with the old status
	d.metrics.BlocksToUpgrade.Reset()

	upcomingUpgrades := d.ur.GetUpcomingUpgradesWithCache(d.currHeight)
	for _, upgrade := range upcomingUpgrades {
		upgradeHeight := strconv.FormatInt(upgrade.Height, 10)
		status := d.stateMachine.GetStatus(upgrade.Height)

		preChecksStatus := make([]string, 0, len(checksproto.PreCheck_value))
		for _, v := range checksproto.PreCheck_value {
			preChecksStatus = append(preChecksStatus, d.stateMachine.GetPreCheckStatus(upgrade.Height, checksproto.PreCheck(v)).String())
		}

		postChecksStatus := make([]string, 0, len(checksproto.PreCheck_value))
		for _, v := range checksproto.PostCheck_value {
			postChecksStatus = append(postChecksStatus, d.stateMachine.GetPostCheckStatus(upgrade.Height, checksproto.PostCheck(v)).String())
		}

		// Merge all label values into a single slice
		labelValues := append([]string{
			upgradeHeight, upgrade.Name, status.String(),
			d.stateMachine.GetStep(upgrade.Height).String(), d.validatorAddress, upgrade.Tag,
		}, preChecksStatus...)
		labelValues = append(labelValues, postChecksStatus...)

		d.metrics.BlocksToUpgrade.WithLabelValues(labelValues...).Set(float64(upgrade.Height - d.currHeight))
	}
}
