package chain

import (
	"fmt"
	"time"

	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"

	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

type ProposalStatus int

const (
	UNKNOWN        ProposalStatus = -1
	UNSPECIFIED    ProposalStatus = 0
	DEPOSIT_PERIOD ProposalStatus = 1
	VOTING_PERIOD  ProposalStatus = 2
	PASSED         ProposalStatus = 3
	REJECTED       ProposalStatus = 4
	FAILED         ProposalStatus = 5
	CANCELLED      ProposalStatus = 6
)

func (ps ProposalStatus) String() string {
	switch ps {
	case UNKNOWN:
		return "UNKNOWN"
	case UNSPECIFIED:
		return "UNSPECIFIED"
	case DEPOSIT_PERIOD:
		return "DEPOSIT_PERIOD"
	case VOTING_PERIOD:
		return "VOTING_PERIOD"
	case PASSED:
		return "PASSED"
	case REJECTED:
		return "REJECTED"
	case FAILED:
		return "FAILED"
	case CANCELLED:
		return "CANCELLED"
	default:
		return fmt.Sprintf("%d", int(ps))
	}
}

type chainUpgrade struct {
	Height     int64
	Name       string
	Status     ProposalStatus
	Network    string
	ProposalID uint64
	CreatedAt  time.Time
}

func (cu chainUpgrade) ToProto() urproto.Upgrade {
	source := urproto.ProviderType_CHAIN

	upgradeStatus := urproto.UpgradeStatus_UNKNOWN
	switch cu.Status {
	case UNKNOWN:
	case UNSPECIFIED:
		upgradeStatus = urproto.UpgradeStatus_UNKNOWN
	case DEPOSIT_PERIOD:
		upgradeStatus = urproto.UpgradeStatus_SCHEDULED
	case VOTING_PERIOD:
		upgradeStatus = urproto.UpgradeStatus_SCHEDULED
	case PASSED:
		upgradeStatus = urproto.UpgradeStatus_ACTIVE
	case REJECTED:
		upgradeStatus = urproto.UpgradeStatus_CANCELLED
	case FAILED:
		upgradeStatus = urproto.UpgradeStatus_CANCELLED
	case CANCELLED:
		upgradeStatus = urproto.UpgradeStatus_CANCELLED
	}

	// #nosec G115
	proposalID := int64(cu.ProposalID)
	return urproto.Upgrade{
		Height:     cu.Height,
		Tag:        "",
		Network:    cu.Network,
		Name:       cu.Name,
		Type:       urproto.UpgradeType_GOVERNANCE,
		Status:     upgradeStatus,
		Source:     source,
		ProposalId: &proposalID,
	}
}

func fromV1(status v1.ProposalStatus) ProposalStatus {
	switch status {
	case v1.ProposalStatus_PROPOSAL_STATUS_UNSPECIFIED:
		return UNSPECIFIED
	case v1.ProposalStatus_PROPOSAL_STATUS_DEPOSIT_PERIOD:
		return DEPOSIT_PERIOD
	case v1.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD:
		return VOTING_PERIOD
	case v1.ProposalStatus_PROPOSAL_STATUS_PASSED:
		return PASSED
	case v1.ProposalStatus_PROPOSAL_STATUS_REJECTED:
		return REJECTED
	case v1.ProposalStatus_PROPOSAL_STATUS_FAILED:
		return FAILED
	default:
		return UNKNOWN
	}
}

func fromV1beta1(status v1beta1.ProposalStatus) ProposalStatus {
	switch status {
	case v1beta1.StatusNil:
		return UNSPECIFIED
	case v1beta1.StatusDepositPeriod:
		return DEPOSIT_PERIOD
	case v1beta1.StatusVotingPeriod:
		return VOTING_PERIOD
	case v1beta1.StatusPassed:
		return PASSED
	case v1beta1.StatusRejected:
		return REJECTED
	case v1beta1.StatusFailed:
		return FAILED
	default:
		return UNKNOWN
	}
}

func trySoftwareUpgradeProposal(typeURL string, value []byte, status ProposalStatus, chain string, submitTime time.Time) (*chainUpgrade, error) {
	if typeURL == "/cosmos.upgrade.v1beta1.SoftwareUpgradeProposal" {
		// this is deprecated but still widely used on chains
		upgrade := &upgradetypes.SoftwareUpgradeProposal{}
		if err := upgrade.Unmarshal(value); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal SoftwareUpgradeProposal")
		}
		return &chainUpgrade{
			Height:    upgrade.Plan.Height,
			Name:      upgrade.Plan.Name,
			Status:    status,
			Network:   chain,
			CreatedAt: submitTime,
		}, nil
	}
	return nil, nil
}

func tryMsgSoftwareUpgrade(typeURL string, value []byte, status ProposalStatus, chain string, submitTime time.Time) (*chainUpgrade, error) {
	if typeURL == "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade" {
		upgrade := &upgradetypes.MsgSoftwareUpgrade{}
		if err := upgrade.Unmarshal(value); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal MsgSoftwareUpgrade")
		}
		return &chainUpgrade{
			Height:    upgrade.Plan.Height,
			Name:      upgrade.Plan.Name,
			Status:    status,
			Network:   chain,
			CreatedAt: submitTime,
		}, nil
	}
	return nil, nil
}
