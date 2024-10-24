package chain

import (
	"context"
	"slices"

	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	"blazar/internal/pkg/provider"

	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

type Provider struct {
	cosmosClient *cosmos.Client
	chain        string
	priority     int32
}

func NewProvider(cosmosClient *cosmos.Client, chain string, priority int32) *Provider {
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

	// cosmos-sdk allows changing parameters of a previously passed upgrade
	// by creating a new upgrade proposal with the same name in upgrade plan
	// https://github.com/cosmos/cosmos-sdk/blob/41f92723399ef0affa90c6b3d8e7b47b82361280/x/upgrade/keeper/keeper.go#L185
	// since, upgrades is sorted by proposal ID and we'll only keep last instance for a name
	// if a passed upgrade already exists for that name
	passedNames := make(map[string]struct{}, len(upgrades))
	filtered := make([]chainUpgrade, 0, len(upgrades))
	slices.Reverse(upgrades)
	for _, upgrade := range upgrades {
		if _, ok := passedNames[upgrade.Name]; !ok {
			if upgrade.Status == PASSED {
				passedNames[upgrade.Name] = struct{}{}
			}
			filtered = append(filtered, upgrade)
		}
	}

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
