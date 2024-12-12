package daemon

import (
	"strconv"
)

func (d *Daemon) updateMetrics() {
	// the upgrade state may change, we don't want to persist the metric with the old status
	d.metrics.BlocksToUpgrade.Reset()

	upcomingUpgrades := d.ur.GetUpcomingUpgradesWithCache(d.currHeight)
	for _, upgrade := range upcomingUpgrades {
		upgradeHeight := strconv.FormatInt(upgrade.Height, 10)
		status := d.stateMachine.GetStatus(upgrade.Height)

		d.metrics.BlocksToUpgrade.WithLabelValues(upgradeHeight, upgrade.Name, status.String(),
			d.stateMachine.GetStep(upgrade.Height).String(), d.chainID, d.validatorAddress).Set(float64(upgrade.Height - d.currHeight))
	}
}
