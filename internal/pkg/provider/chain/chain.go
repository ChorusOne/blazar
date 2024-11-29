package chain

import (
	"context"
	"sort"

	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"blazar/internal/pkg/provider"

	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

type CosmosProposalsProvider interface {
	GetProposalsV1(ctx context.Context) (v1.Proposals, error)
	GetProposalsV1beta1(ctx context.Context) (v1beta1.Proposals, error)
}

type Provider struct {
	cosmosClient CosmosProposalsProvider
	chain        string
	priority     int32
}

func NewProvider(cosmosClient CosmosProposalsProvider, chain string, priority int32) *Provider {
	return &Provider{
		cosmosClient: cosmosClient,
		chain:        chain,
		priority:     priority,
	}
}

func (p *Provider) GetUpgrades(ctx context.Context) ([]*urproto.Upgrade, error) {
	upgrades, err := p.fetchAllUpgrades(ctx)
	if err != nil {
		return []*urproto.Upgrade{}, err
	}

	// Blazar expects one upgrade per height, but the governance allows to create multiple proposals for the same height
	// In the end only one upgrade will be expecuted at given height, no matter how many software upgrades proposals are registered onchain
	// The most common case for having more than one proposal is when someone create a new proposal and asks everyone to vote-no on the previous one
	// due to invalid data etc.
	//
	// To handle this case we pick the last proposal for each height with some conditions:
	// 1. if there is a proposal in PASSED state, we pick it
	// 2. if there are two equal proposals say in VOTING_PERIOD state, we pick the one with the highest proposal id

	// sort upgrades in descending order by proposal id
	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].ProposalID > upgrades[j].ProposalID
	})

	upgradesByHeight := make(map[int64][]chainUpgrade)
	for _, upgrade := range upgrades {
		if _, ok := upgradesByHeight[upgrade.Height]; !ok {
			upgradesByHeight[upgrade.Height] = make([]chainUpgrade, 0)
		}
		upgradesByHeight[upgrade.Height] = append(upgradesByHeight[upgrade.Height], upgrade)
	}

	filtered := make([]chainUpgrade, 0, len(upgrades))
	for _, upgradesForHeight := range upgradesByHeight {
		// if there is only one upgrade for the height, we don't need to do anything
		if len(upgradesForHeight) == 1 {
			filtered = append(filtered, upgradesForHeight[0])
			continue
		}

		// if there is a passed upgrade, we pick it
		foundPassed := false
		for _, upgrade := range upgradesForHeight {
			// the upgrades are sorted by proposal id in descending order
			// so the first upgrade in the list is the one with the highest
			// proposal id (in case there are two PASSED proposals for the same height)
			if upgrade.Status == PASSED {
				foundPassed = true
				filtered = append(filtered, upgrade)
				break
			}
		}

		// if there is no passed upgrade, we pick the one with the highest proposal id
		if !foundPassed {
			filtered = append(filtered, upgradesForHeight[0])
		}
	}

	// If multiple passed upgrade proposals are in the "passed" state,
	// the cosmos upgrade handler only treats the one with the highest proposal ID
	// as "passed" and all other passed proposals as "cancelled".
	// This is not to be confused with the code above, which handles the
	// case where multiple upgrade proposals exist for the same upgrade height
	// https://github.com/cosmos/cosmos-sdk/blob/f007a4ea0711da2bac20afc6283885c1b2496ae5/x/upgrade/keeper/keeper.go#L189-L193
	latestPassedProposal := uint64(0)
	isAnyProposalPassed := false
	for _, upgrade := range filtered {
		if upgrade.Status == PASSED {
			latestPassedProposal = max(latestPassedProposal, upgrade.ProposalID)
			isAnyProposalPassed = true
		}
	}

	if isAnyProposalPassed {
		for i := range filtered {
			if filtered[i].Status == PASSED && filtered[i].ProposalID < latestPassedProposal {
				filtered[i].Status = CANCELLED
			}
		}
	}

	// sort upgrades in descending order by proposal id because iterating over map doesn't guarantee order
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ProposalID > filtered[j].ProposalID
	})

	return toProto(filtered, p.priority), nil
}

func (p *Provider) GetUpgradesByType(ctx context.Context, upgradeType urproto.UpgradeType) ([]*urproto.Upgrade, error) {
	upgrades, err := p.GetUpgrades(ctx)
	if err != nil {
		return []*urproto.Upgrade{}, err
	}

	filtered := make([]*urproto.Upgrade, 0, len(upgrades))
	for _, upgrade := range upgrades {
		if upgrade.Type == upgradeType {
			filtered = append(filtered, upgrade)
		}
	}

	return filtered, nil
}

func (p *Provider) GetUpgradesByHeight(ctx context.Context, height int64) ([]*urproto.Upgrade, error) {
	upgrades, err := p.GetUpgrades(ctx)
	if err != nil {
		return []*urproto.Upgrade{}, err
	}

	filtered := make([]*urproto.Upgrade, 0, len(upgrades))
	for _, upgrade := range upgrades {
		if upgrade.Height == height {
			filtered = append(filtered, upgrade)
		}
	}

	return filtered, nil
}

func (p *Provider) AddUpgrade(_ context.Context, _ *urproto.Upgrade, _ bool) error {
	return errors.New("add upgrade is not supported for chain provider")
}

func (p *Provider) RegisterVersion(_ uint64, _ string) error {
	return errors.New("register version is not supported for chain provider")
}

func (p *Provider) GetVersion(_ uint64) (string, error) {
	return "", errors.New("get version is not supported for chain provider")
}

func (p *Provider) CancelUpgrade(_ context.Context, _ int64, _ string) error {
	return errors.New("cancel upgrade is not supported for chain provider")
}

func (p *Provider) Type() urproto.ProviderType {
	return urproto.ProviderType_CHAIN
}

func (p *Provider) fetchAllUpgrades(ctx context.Context) ([]chainUpgrade, error) {
	upgrades, errV1 := p.getUpgradeProposalsV1(ctx)
	if errV1 != nil {
		var errV1beta1 error

		upgrades, errV1beta1 = p.getUpgradeProposalsV1beta1(ctx)
		if errV1beta1 != nil {
			return []chainUpgrade{}, errors.Wrapf(errors.Join(errV1, errV1beta1), "failed to scrape upgrade proposals from both v1 and v1beta endpoints")
		}
	}

	return upgrades, nil
}

func (p *Provider) getUpgradeProposalsV1beta1(ctx context.Context) ([]chainUpgrade, error) {
	proposals, err := p.cosmosClient.GetProposalsV1beta1(ctx)
	upgrades := make([]chainUpgrade, 0, 10)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get proposals from v1beta1 endpoint")
	}

	for _, proposal := range proposals {
		status := fromV1beta1(proposal.Status)
		if status != REJECTED && status != FAILED {
			upgrade, err := parseProposal(
				proposal.Content.TypeUrl,
				proposal.Content.Value,
				status,
				proposal.ProposalId,
				p.chain,
			)
			if err != nil {
				return nil, err
			}
			if upgrade != nil {
				upgrades = append(upgrades, *upgrade)
			}
			// NOTE: Blazar doesn't support MsgCancelUpgrade because we haven't seen it ever to be used
		}
	}
	return upgrades, nil
}

func (p *Provider) getUpgradeProposalsV1(ctx context.Context) ([]chainUpgrade, error) {
	proposals, err := p.cosmosClient.GetProposalsV1(ctx)
	upgrades := make([]chainUpgrade, 0, 10)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get proposals from v1 endpoint")
	}

	for _, proposal := range proposals {
		status := fromV1(proposal.Status)
		if status != REJECTED && status != FAILED {
			for _, msg := range proposal.Messages {
				var (
					typeURL = msg.GetTypeUrl()
					content = msg.GetValue()
				)
				if msg.GetTypeUrl() == "/cosmos.gov.v1.MsgExecLegacyContent" {
					legacyContent := &v1.MsgExecLegacyContent{}
					if err := legacyContent.Unmarshal(msg.GetValue()); err != nil {
						return nil, errors.Wrapf(err, "failed to unmarshal MsgExecLegacyContent in proposal id %d", proposal.GetId())
					}

					typeURL = legacyContent.Content.TypeUrl
					content = legacyContent.Content.GetValue()
				}

				upgrade, err := parseProposal(
					typeURL,
					content,
					status,
					proposal.GetId(),
					p.chain,
				)
				if err != nil {
					return nil, err
				}
				if upgrade != nil {
					upgrades = append(upgrades, *upgrade)
				}
				// NOTE: Blazar doesn't support MsgCancelUpgrade because we haven't seen it ever to be used
			}
		}
	}
	return upgrades, nil
}

func parseProposal(typeURL string, content []byte, status ProposalStatus, proposalID uint64, chain string) (*chainUpgrade, error) {
	upgrade, err := trySoftwareUpgradeProposal(typeURL, content, status, chain)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to process proposal id %d", proposalID)
	}
	if upgrade != nil {
		upgrade.ProposalID = proposalID
		return upgrade, nil
	}

	upgrade, err = tryMsgSoftwareUpgrade(typeURL, content, status, chain)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to process proposal id %d", proposalID)
	}
	if upgrade != nil {
		upgrade.ProposalID = proposalID
		return upgrade, nil
	}

	return nil, nil
}

func toProto(upgrades []chainUpgrade, priority int32) []*urproto.Upgrade {
	newUpgrades := make([]*urproto.Upgrade, 0, len(upgrades))

	for _, upgrade := range upgrades {
		upg := upgrade.ToProto()
		provider.PostProcessUpgrade(&upg, urproto.ProviderType_CHAIN, priority)
		newUpgrades = append(newUpgrades, &upg)
	}

	return newUpgrades
}
