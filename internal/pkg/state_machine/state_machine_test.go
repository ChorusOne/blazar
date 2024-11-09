package state_machine

import (
	"testing"

	urproto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/stretchr/testify/assert"
)

// Asserts that the state machine panics when it receives an upgrade with an initial status that is not managed by the state machine
func TestStateMachineInitialUpgradeStates(t *testing.T) {
	for upgradeStatus, shouldFail := range map[urproto.UpgradeStatus]bool{
		// states managed by the provider or manually
		urproto.UpgradeStatus_UNKNOWN:   false,
		urproto.UpgradeStatus_SCHEDULED: false,
		urproto.UpgradeStatus_ACTIVE:    false,
		urproto.UpgradeStatus_CANCELLED: false,

		// states managed by the state machine
		urproto.UpgradeStatus_EXECUTING: true,
		urproto.UpgradeStatus_COMPLETED: true,
		urproto.UpgradeStatus_FAILED:    true,
		urproto.UpgradeStatus_EXPIRED:   true,
	} {
		currentHeight := int64(100)
		upgrades := []*urproto.Upgrade{
			{
				Height: 200,
				Tag:    "v1.0.0",
				Name:   "test upgrade",
				Type:   urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
				Status: upgradeStatus,
				Source: urproto.ProviderType_CHAIN,
			},
		}

		upgradesMap := make(map[int64]*urproto.Upgrade)
		for _, upgrade := range upgrades {
			upgradesMap[upgrade.Height] = upgrade
		}

		stateMachine := NewStateMachine(nil)
		if shouldFail {
			assert.Panics(t, func() {
				stateMachine.UpdateStatus(currentHeight, upgradesMap)
			})
		} else {
			assert.NotPanics(t, func() {
				stateMachine.UpdateStatus(currentHeight, upgradesMap)
			})
		}
	}
}

// Asserts that GOVERNANCE proposals from various chain sources are set to correct initial state
func TestStateMachineInitialNonChainGov(t *testing.T) {
	for provider, expectedStatus := range map[urproto.ProviderType]urproto.UpgradeStatus{
		urproto.ProviderType_CHAIN:    urproto.UpgradeStatus_UNKNOWN,
		urproto.ProviderType_LOCAL:    urproto.UpgradeStatus_ACTIVE,
		urproto.ProviderType_DATABASE: urproto.UpgradeStatus_ACTIVE,
	} {
		upgrades := []*urproto.Upgrade{
			{
				Height: 200,
				Tag:    "v1.0.0",
				Name:   "test upgrade",
				Type:   urproto.UpgradeType_GOVERNANCE,
				Status: urproto.UpgradeStatus_UNKNOWN,
				Source: provider,
			},
		}

		upgradesMap := make(map[int64]*urproto.Upgrade)
		for _, upgrade := range upgrades {
			upgradesMap[upgrade.Height] = upgrade
		}

		stateMachine := NewStateMachine(nil)
		stateMachine.UpdateStatus(100, upgradesMap)
		assert.Equal(t, expectedStatus.String(), stateMachine.GetStatus(200).String())
	}
}

// Asserts that the state machine panics when it receives an upgrade with an initial status that is not managed by the state machine
func TestStateMachineUpgradesAreDeleted(t *testing.T) {
	currentHeight := int64(100)
	upgrades := []*urproto.Upgrade{
		{
			Height: 200,
			Tag:    "v1.0.0",
			Name:   "test upgrade",
			Type:   urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status: urproto.UpgradeStatus_ACTIVE,
			Source: urproto.ProviderType_CHAIN,
		},
		{
			Height: 400,
			Tag:    "v1.0.0",
			Name:   "test upgrade",
			Type:   urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
			Status: urproto.UpgradeStatus_ACTIVE,
			Source: urproto.ProviderType_CHAIN,
		},
	}

	upgradesMap := make(map[int64]*urproto.Upgrade)
	for _, upgrade := range upgrades {
		upgradesMap[upgrade.Height] = upgrade
	}

	stateMachine := NewStateMachine(nil)
	stateMachine.UpdateStatus(currentHeight, upgradesMap)

	assert.Equal(t, urproto.UpgradeStatus_ACTIVE, stateMachine.GetStatus(200))

	// remove the upgrade with height 200
	delete(upgradesMap, 200)
	stateMachine.UpdateStatus(currentHeight, upgradesMap)

	assert.Equal(t, urproto.UpgradeStatus_UNKNOWN, stateMachine.GetStatus(200))
	assert.Equal(t, urproto.UpgradeStatus_ACTIVE, stateMachine.GetStatus(400))
}

// Asserts that the state machine sets the expiry status correctly
func TestStateMachineExpiry(t *testing.T) {
	type testType struct {
		initialStatus  urproto.UpgradeStatus
		expectedStatus urproto.UpgradeStatus
		step           urproto.UpgradeStep
	}

	tests := []testType{
		{urproto.UpgradeStatus_UNKNOWN, urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_SCHEDULED, urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},

		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_FAILED, urproto.UpgradeStep_NONE},

		// check if the active step doesn't mess up with the states
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStep_COMPOSE_FILE_UPGRADE},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStep_POST_UPGRADE_CHECK},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_FAILED, urproto.UpgradeStep_COMPOSE_FILE_UPGRADE},

		// if blazar missed the upgrade, this is the upgrade block is passed and it didn't transition into executing
		// the status is set to expire. While it is possible such option can happen, handling the past upgrades that "should be executed"
		// makes things more complicated
		{urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStep_MONITORING},
	}

	for upgradeType, tests := range map[urproto.UpgradeType][]testType{
		urproto.UpgradeType_GOVERNANCE:                   tests,
		urproto.UpgradeType_NON_GOVERNANCE_COORDINATED:   tests,
		urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED: tests,
	} {
		for _, test := range tests {
			currentHeight := int64(100)
			upgrades := []*urproto.Upgrade{
				{
					Height: 50, // this upgrade time has passed
					Tag:    "v1.0.0",
					Name:   "test upgrade",
					Type:   upgradeType,
					Status: urproto.UpgradeStatus_UNKNOWN,
				},
			}

			upgradesMap := make(map[int64]*urproto.Upgrade)
			for _, upgrade := range upgrades {
				upgradesMap[upgrade.Height] = upgrade
			}

			stateMachine := NewStateMachine(nil)
			_ = stateMachine.SetStatus(upgrades[0].Height, test.initialStatus)
			stateMachine.SetStep(upgrades[0].Height, test.step)
			assert.Equal(t, test.initialStatus, stateMachine.GetStatus(upgrades[0].Height))

			stateMachine.UpdateStatus(currentHeight, upgradesMap)
			assert.Equal(t, test.expectedStatus, stateMachine.GetStatus(upgrades[0].Height))
		}
	}
}

// Asserts the ability to cancel an upgrade
func TestStateMachineCancellation(t *testing.T) {
	type testType struct {
		initialStatus  urproto.UpgradeStatus
		expectedStatus urproto.UpgradeStatus
		step           urproto.UpgradeStep
	}

	// The rule of thumb is that a user can cancel upgrade only if:
	// * the upgrade is not being executed yet (status != EXECUTING or anything past that, like COMPLETED or FAILED)
	// * if the upgrade started and current step is NONE or MONITORING or in PRE CHECK (anything past that is not cancellable)
	allCancellableTests := []testType{
		{urproto.UpgradeStatus_UNKNOWN, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_SCHEDULED, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},

		// we allow to cancel if the upgrade didn't start yet (as in, no step has been taken yet)
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_NONE},

		// if current step is MONITORING, we can still cancel the upgrade
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_MONITORING},

		// if current step is PRE_UPGRADE_CHECK, we can still cancel the upgrade
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStep_PRE_UPGRADE_CHECK},

		// if the upgrade is already being executed, we can't cancel it
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStep_COMPOSE_FILE_UPGRADE},

		// if the upgrade is already being executed, we can't cancel it (because it already happened, right?)
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStep_POST_UPGRADE_CHECK},

		// if upgrade is expired, completed, failed and cancelled then we can't cancel it
		{urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStatus_EXPIRED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStep_NONE},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_FAILED, urproto.UpgradeStep_NONE},
	}

	for upgradeType, tests := range map[urproto.UpgradeType][]testType{
		urproto.UpgradeType_GOVERNANCE:                   allCancellableTests,
		urproto.UpgradeType_NON_GOVERNANCE_COORDINATED:   allCancellableTests,
		urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED: allCancellableTests,
	} {
		for _, test := range tests {
			currentHeight := int64(100)
			upgrades := []*urproto.Upgrade{
				{
					Height: 150,
					Tag:    "v1.0.0",
					Name:   "test upgrade",
					Type:   upgradeType,
					Status: urproto.UpgradeStatus_UNKNOWN,
				},
			}

			upgradesMap := make(map[int64]*urproto.Upgrade)
			for _, upgrade := range upgrades {
				upgradesMap[upgrade.Height] = upgrade
			}

			stateMachine := NewStateMachine(nil)

			// set initial status and step
			_ = stateMachine.SetStatus(upgrades[0].Height, test.initialStatus)
			stateMachine.SetStep(upgrades[0].Height, test.step)
			stateMachine.UpdateStatus(currentHeight, upgradesMap)

			// simulate the cancellation
			_ = stateMachine.SetStatus(upgrades[0].Height, urproto.UpgradeStatus_CANCELLED)
			stateMachine.SetStep(upgrades[0].Height, test.step)
			stateMachine.UpdateStatus(currentHeight, upgradesMap)

			assert.Equal(t, test.expectedStatus, stateMachine.GetStatus(upgrades[0].Height))
		}
	}
}

// Asserts clearly invalid upgrade state transitions are not allowed
func TestStateMachineInvalidStateTransitions(t *testing.T) {
	type testType struct {
		initialStatus urproto.UpgradeStatus
		newStatus     urproto.UpgradeStatus
		expectError   bool
	}

	allTests := []testType{
		// if the upgrade was executing then regressing to prior states is likely a bug
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_SCHEDULED, true},
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_ACTIVE, true},

		// TODO: I am not sure if we should allow this transition
		{urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStatus_EXPIRED, false},

		// upgrade in completed state is final and can't be changed
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_UNKNOWN, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_SCHEDULED, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_ACTIVE, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_EXECUTING, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_COMPLETED, false},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_FAILED, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_CANCELLED, true},
		{urproto.UpgradeStatus_COMPLETED, urproto.UpgradeStatus_EXPIRED, true},

		// upgrade in failed state is final and can't be changed
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_UNKNOWN, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_SCHEDULED, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_ACTIVE, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_EXECUTING, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_COMPLETED, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_FAILED, false},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_CANCELLED, true},
		{urproto.UpgradeStatus_FAILED, urproto.UpgradeStatus_EXPIRED, true},

		// upgrade in cancelled state is final and can't be changed
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_UNKNOWN, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_SCHEDULED, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_ACTIVE, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_EXECUTING, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_COMPLETED, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_FAILED, true},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_CANCELLED, false},
		{urproto.UpgradeStatus_CANCELLED, urproto.UpgradeStatus_EXPIRED, true},
	}

	for upgradeType, tests := range map[urproto.UpgradeType][]testType{
		urproto.UpgradeType_GOVERNANCE:                   allTests,
		urproto.UpgradeType_NON_GOVERNANCE_COORDINATED:   allTests,
		urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED: allTests,
	} {
		for _, test := range tests {
			currentHeight := int64(100)
			upgrades := []*urproto.Upgrade{
				{
					Height: 150,
					Tag:    "v1.0.0",
					Name:   "test upgrade",
					Type:   upgradeType,
					Status: urproto.UpgradeStatus_UNKNOWN,
				},
			}

			upgradesMap := make(map[int64]*urproto.Upgrade)
			for _, upgrade := range upgrades {
				upgradesMap[upgrade.Height] = upgrade
			}

			stateMachine := NewStateMachine(nil)

			// set initial status and step
			_ = stateMachine.SetStatus(upgrades[0].Height, test.initialStatus)
			stateMachine.UpdateStatus(currentHeight, upgradesMap)

			// simulate the state change attempt
			err := stateMachine.SetStatus(upgrades[0].Height, test.newStatus)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	}
}
