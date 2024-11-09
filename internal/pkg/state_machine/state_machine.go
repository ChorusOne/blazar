package state_machine

import (
	"context"
	"fmt"
	"slices"
	"sync"

	checksproto "blazar/internal/pkg/proto/daemon"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
)

// The rule are are as follows:
// 1. upgrades coming from the providers have one of the following statuses (`upgrade.Status`)
//   - UNKNOWN
//   - SCHEDULED
//   - ACTIVE
//   - CANCELLED
//
// The provider can update the upgrade status from eg. SCHEDULED to ACTIVE when an onchain
// governance proposal passed but statuses such as ACTIVE, EXECUTING, COMPLETED are only used by
// blazar state machine.
var (
	allowedInputStatuses = []urproto.UpgradeStatus{
		urproto.UpgradeStatus_UNKNOWN,
		urproto.UpgradeStatus_SCHEDULED,
		urproto.UpgradeStatus_ACTIVE,
		urproto.UpgradeStatus_CANCELLED,
	}

	statusManagedByStateMachine = []urproto.UpgradeStatus{
		urproto.UpgradeStatus_EXECUTING,
		urproto.UpgradeStatus_COMPLETED,
		urproto.UpgradeStatus_FAILED,
		urproto.UpgradeStatus_EXPIRED,
	}
)

func init() {
	// sanity check
	if len(urproto.UpgradeStatus_value) != len(allowedInputStatuses)+len(statusManagedByStateMachine) {
		panic(fmt.Sprintf("allowedInputStatuses and statusManagedByStateMachine do not cover all upgrade statuses. allowedInputStatuses: %d, statusManagedByStateMachine: %d, total: %d", len(allowedInputStatuses), len(statusManagedByStateMachine), len(urproto.UpgradeStatus_value)))
	}
}

type StateMachineStorage interface {
	StoreState(context.Context, *State) error
	RestoreState(context.Context) (*State, error)
}

type State struct {
	UpgradeStatus map[int64]urproto.UpgradeStatus `json:"status"`
	UpgradeStep   map[int64]urproto.UpgradeStep   `json:"steps"`

	PreCheckStatus  map[int64]map[checksproto.PreCheck]checksproto.CheckStatus  `json:"pre_check_status"`
	PostCheckStatus map[int64]map[checksproto.PostCheck]checksproto.CheckStatus `json:"post_check_status"`
}

// Simple, unsphisitcated state machine for managing upgrades
type StateMachine struct {
	lock  *sync.RWMutex
	state *State

	storage StateMachineStorage
}

func NewStateMachine(storage StateMachineStorage) *StateMachine {
	return &StateMachine{
		lock: &sync.RWMutex{},
		state: &State{
			UpgradeStatus: make(map[int64]urproto.UpgradeStatus, 0),
			UpgradeStep:   make(map[int64]urproto.UpgradeStep, 0),

			PreCheckStatus:  make(map[int64]map[checksproto.PreCheck]checksproto.CheckStatus, 0),
			PostCheckStatus: make(map[int64]map[checksproto.PostCheck]checksproto.CheckStatus, 0),
		},
		storage: storage,
	}
}

func (sm *StateMachine) UpdateStatus(currentHeight int64, upgrades map[int64]*urproto.Upgrade) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	for _, upgrade := range upgrades {
		if !slices.Contains(allowedInputStatuses, upgrade.Status) {
			panic(fmt.Sprintf("invalid upgrade status set in upgrade.Status field: %s. The list of allowed status: %s", upgrade.Status.String(), allowedInputStatuses))
		}

		// set status if it doesn't exist
		if _, ok := sm.state.UpgradeStatus[upgrade.Height]; !ok {
			sm.state.UpgradeStatus[upgrade.Height] = upgrade.Status
		}

		if _, ok := sm.state.UpgradeStep[upgrade.Height]; !ok {
			sm.state.UpgradeStep[upgrade.Height] = urproto.UpgradeStep_NONE
		}
	}

	// remove upgrade status that are not in the new list
	for height := range sm.state.UpgradeStatus {
		if _, ok := upgrades[height]; !ok {
			// TODO: consider maybe setting the status to CANCELLED instead of removing the upgrade
			delete(sm.state.UpgradeStatus, height)
			delete(sm.state.UpgradeStep, height)
		}
	}

	for _, upgrade := range upgrades {
		// if the upgrade is cancelled then there is nothing to do, we simply update the status
		// NOTE: the upgrade.status is set by provider (eg. chain provider) and the state machine state cancelled is set by a human through rpc etc
		if upgrade.Status == urproto.UpgradeStatus_CANCELLED || sm.state.UpgradeStatus[upgrade.Height] == urproto.UpgradeStatus_CANCELLED {
			sm.state.UpgradeStatus[upgrade.Height] = urproto.UpgradeStatus_CANCELLED
			continue
		}

		switch upgrade.Type {
		case urproto.UpgradeType_GOVERNANCE:
			// if the new status is coming from governance proposal then we update
			// otherwise it must have been set by a blazar instance while processing upgrade
			if !slices.Contains(statusManagedByStateMachine, sm.state.UpgradeStatus[upgrade.Height]) {
				// Some chains implement their own governance system that may be compatible with the cosmos sdk.
				// Take "Neutron", it uses smart contract for governance and it is also compatible with the cosmos sdk, such that
				// it produces the upgrade-info.json at the upgrade height.
				//
				// We want to handle the case where the GOVERNANCE upgrade is not coming from the chain itself but from anoter provider.
				// In this case, we want to mark the upgrade as ACTIVE as there is no onchain component (blazar is aware of) that manages the upgrade status.
				if upgrade.Source != urproto.ProviderType_CHAIN {
					if upgrade.Height > currentHeight {
						sm.state.UpgradeStatus[upgrade.Height] = urproto.UpgradeStatus_ACTIVE
					}
				} else {
					// the onchain status is the source of truth
					sm.state.UpgradeStatus[upgrade.Height] = upgrade.Status
				}
			}
		case urproto.UpgradeType_NON_GOVERNANCE_COORDINATED, urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED:
			// mark the upgrade as 'ready for exection' (active)
			if !slices.Contains(statusManagedByStateMachine, sm.state.UpgradeStatus[upgrade.Height]) {
				if upgrade.Height > currentHeight {
					sm.state.UpgradeStatus[upgrade.Height] = urproto.UpgradeStatus_ACTIVE
				}
			}

		default:
			panic(fmt.Sprintf("unknown upgrade type %s", upgrade.Type.String()))
		}
	}

	// sanity check
	if len(sm.state.UpgradeStatus) != len(upgrades) {
		panic(fmt.Sprintf("upgrade status map length %d does not match upgrade list length %d", len(sm.state.UpgradeStatus), len(upgrades)))
	}

	// handle other status changes
	for _, upgrade := range upgrades {
		status := sm.state.UpgradeStatus[upgrade.Height]

		// handle expired upgrades
		isPastUpgrade := upgrade.Height < currentHeight
		if isPastUpgrade && status != urproto.UpgradeStatus_CANCELLED && !slices.Contains(statusManagedByStateMachine, status) {
			sm.state.UpgradeStatus[upgrade.Height] = urproto.UpgradeStatus_EXPIRED
		}
	}
}

func (sm *StateMachine) MustSetStatus(height int64, status urproto.UpgradeStatus) {
	if err := sm.SetStatus(height, status); err != nil {
		panic(err)
	}
}

func (sm *StateMachine) SetStatus(height int64, status urproto.UpgradeStatus) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	return sm.setStatus(height, status, false)
}

func (sm *StateMachine) SetStep(height int64, step urproto.UpgradeStep) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	sm.state.UpgradeStep[height] = step
}

func (sm *StateMachine) MustSetStatusAndStep(height int64, status urproto.UpgradeStatus, step urproto.UpgradeStep) {
	if err := sm.SetStatusAndStep(height, status, step); err != nil {
		panic(err)
	}
}

func (sm *StateMachine) SetStatusAndStep(height int64, status urproto.UpgradeStatus, step urproto.UpgradeStep) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	if err := sm.setStatus(height, status, false); err != nil {
		return err
	}
	sm.state.UpgradeStep[height] = step

	return nil
}

func (sm *StateMachine) GetStatus(height int64) urproto.UpgradeStatus {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	if status, ok := sm.state.UpgradeStatus[height]; ok {
		return status
	}

	return urproto.UpgradeStatus_UNKNOWN
}

func (sm *StateMachine) GetStep(height int64) urproto.UpgradeStep {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	if step, ok := sm.state.UpgradeStep[height]; ok {
		return step
	}

	return urproto.UpgradeStep_NONE
}

func (sm *StateMachine) SetPreCheckStatus(height int64, check checksproto.PreCheck, status checksproto.CheckStatus) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	if _, ok := sm.state.PreCheckStatus[height]; !ok {
		sm.state.PreCheckStatus[height] = make(map[checksproto.PreCheck]checksproto.CheckStatus)
	}
	sm.state.PreCheckStatus[height][check] = status
}

func (sm *StateMachine) SetPostCheckStatus(height int64, check checksproto.PostCheck, status checksproto.CheckStatus) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	defer sm.persist()

	if _, ok := sm.state.PostCheckStatus[height]; !ok {
		sm.state.PostCheckStatus[height] = make(map[checksproto.PostCheck]checksproto.CheckStatus)
	}
	sm.state.PostCheckStatus[height][check] = status
}

func (sm *StateMachine) GetPreCheckStatus(height int64, check checksproto.PreCheck) checksproto.CheckStatus {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	if checkStatus, ok := sm.state.PreCheckStatus[height][check]; ok {
		return checkStatus
	}

	return checksproto.CheckStatus_PENDING
}

func (sm *StateMachine) GetPostCheckStatus(height int64, check checksproto.PostCheck) checksproto.CheckStatus {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	if checkStatus, ok := sm.state.PostCheckStatus[height][check]; ok {
		return checkStatus
	}

	return checksproto.CheckStatus_PENDING
}

func (sm *StateMachine) Restore(ctx context.Context) error {
	if sm.storage == nil {
		// if it wasn't configured then we don't need to restore the state
		return nil
	}

	state, err := sm.storage.RestoreState(ctx)
	if err != nil {
		return err
	}

	// the state was likely not initialized yet
	if state == nil {
		return nil
	}

	// initialize the state if it's not initialized
	if state.UpgradeStatus == nil {
		state.UpgradeStatus = make(map[int64]urproto.UpgradeStatus, 0)
	}

	if state.UpgradeStep == nil {
		state.UpgradeStep = make(map[int64]urproto.UpgradeStep, 0)
	}

	if state.PreCheckStatus == nil {
		state.PreCheckStatus = make(map[int64]map[checksproto.PreCheck]checksproto.CheckStatus, 0)
	}

	if state.PostCheckStatus == nil {
		state.PostCheckStatus = make(map[int64]map[checksproto.PostCheck]checksproto.CheckStatus, 0)
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.state = state

	return nil
}

func (sm *StateMachine) setStatus(height int64, status urproto.UpgradeStatus, lock bool) error {
	if lock {
		sm.lock.Lock()
		defer sm.lock.Unlock()
		defer sm.persist()
	}

	// we can't cancel the upgrade if it's already being executed, expired, failed etc
	if currentStatus, ok := sm.state.UpgradeStatus[height]; ok && status == urproto.UpgradeStatus_CANCELLED {
		if currentStep, ok := sm.state.UpgradeStep[height]; ok {
			isExecuting := currentStatus == urproto.UpgradeStatus_EXECUTING && !slices.Contains([]urproto.UpgradeStep{
				urproto.UpgradeStep_NONE,
				urproto.UpgradeStep_MONITORING,
				urproto.UpgradeStep_PRE_UPGRADE_CHECK,
			}, currentStep)

			if isExecuting || slices.Contains([]urproto.UpgradeStatus{
				urproto.UpgradeStatus_EXPIRED,
				urproto.UpgradeStatus_COMPLETED,
				urproto.UpgradeStatus_FAILED,
			}, currentStatus) {
				return fmt.Errorf("cannot cancel upgrade %d with status %s and step %s", height, currentStatus.String(), currentStep.String())
			}
		}
	}

	// handle invalid state transitions
	if currentStatus, ok := sm.state.UpgradeStatus[height]; ok {
		executingTransition := currentStatus == urproto.UpgradeStatus_EXECUTING && (status == urproto.UpgradeStatus_SCHEDULED || status == urproto.UpgradeStatus_ACTIVE)
		completedTransition := currentStatus == urproto.UpgradeStatus_COMPLETED && status != urproto.UpgradeStatus_COMPLETED
		failedTransition := currentStatus == urproto.UpgradeStatus_FAILED && status != urproto.UpgradeStatus_FAILED
		cancelledTransition := currentStatus == urproto.UpgradeStatus_CANCELLED && status != urproto.UpgradeStatus_CANCELLED

		if executingTransition || completedTransition || failedTransition || cancelledTransition {
			return fmt.Errorf("staus transition from %s to %s is not allowed", currentStatus.String(), status.String())
		}
	}

	sm.state.UpgradeStatus[height] = status
	return nil
}

func (sm *StateMachine) persist() {
	// TODO: For now we ignore writing to the storage errors because this is not a critical operation
	// NOTE: The caller must hold the lock
	if sm.storage != nil {
		_ = sm.storage.StoreState(context.TODO(), sm.state)
	}
}
