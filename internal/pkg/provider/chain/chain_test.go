package chain

import (
	"context"
	"fmt"
	"testing"
	"time"

	urproto "blazar/internal/pkg/proto/upgrades_registry"

	sdk "github.com/cosmos/cosmos-sdk/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCosmosClient struct {
	v1Proposals      v1.Proposals
	v1beta1Proposals v1beta1.Proposals
}

func (m *mockCosmosClient) GetProposalsV1(_ context.Context) (v1.Proposals, error) {
	return m.v1Proposals, nil
}

func (m *mockCosmosClient) GetProposalsV1beta1(_ context.Context) (v1beta1.Proposals, error) {
	return m.v1beta1Proposals, nil
}

func TestGetUpgrades(t *testing.T) {
	tests := []struct {
		name      string
		proposals v1.Proposals
		expected  []*urproto.Upgrade
	}{
		{
			name:      "EmptyProposals",
			proposals: v1.Proposals{},
			expected:  []*urproto.Upgrade{},
		},
		{
			name: "Simple",
			proposals: v1.Proposals{
				newProposal(t, 1, 100, v1.StatusPassed),
				newProposal(t, 2, 200, v1.StatusVotingPeriod),
			},
			expected: []*urproto.Upgrade{
				{
					Height: 200,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_SCHEDULED,
					Source: urproto.ProviderType_CHAIN,
				},
				{
					Height: 100,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_ACTIVE,
					Source: urproto.ProviderType_CHAIN,
				},
			},
		},
		{
			name: "MultiplePassed",
			proposals: v1.Proposals{
				newProposal(t, 1, 100, v1.StatusPassed),
				newProposal(t, 2, 200, v1.StatusPassed),
			},
			expected: []*urproto.Upgrade{
				{
					Height: 200,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_ACTIVE,
					Source: urproto.ProviderType_CHAIN,
				},
				{
					Height: 100,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_CANCELLED,
					Source: urproto.ProviderType_CHAIN,
				},
			},
		},
		{
			name: "DuplicateProposalsWithPassedStatus",
			proposals: v1.Proposals{
				newProposal(t, 1, 100, v1.StatusPassed),
				newProposal(t, 2, 100, v1.StatusPassed),
				newProposal(t, 3, 200, v1.StatusDepositPeriod),
			},
			expected: []*urproto.Upgrade{
				{
					Height: 200,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_SCHEDULED,
					Source: urproto.ProviderType_CHAIN,
				},
				{
					Height: 100,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_ACTIVE,
					Source: urproto.ProviderType_CHAIN,
					// the latest proposal in passed state should be returned
					ProposalId: int64ptr(2),
				},
			},
		},
		{
			name: "DuplicateProposalsInVotingPeriod",
			proposals: v1.Proposals{
				newProposal(t, 1, 100, v1.StatusVotingPeriod),
				newProposal(t, 2, 100, v1.StatusVotingPeriod),
				newProposal(t, 3, 200, v1.StatusDepositPeriod),
			},
			expected: []*urproto.Upgrade{
				{
					Height: 200,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_SCHEDULED,
					Source: urproto.ProviderType_CHAIN,
				},
				{
					Height: 100,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_SCHEDULED,
					Source: urproto.ProviderType_CHAIN,
					// in case of two equal proposals in non-active state we expect the one with the highest proposal id
					ProposalId: int64ptr(2),
				},
			},
		},
		{
			name: "DuplicateProposalsInActiveAndVotingPeriod",
			proposals: v1.Proposals{
				newProposal(t, 1, 100, v1.StatusPassed),
				newProposal(t, 2, 100, v1.StatusVotingPeriod),
				newProposal(t, 3, 200, v1.StatusDepositPeriod),
			},
			expected: []*urproto.Upgrade{
				{
					Height: 200,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_SCHEDULED,
					Source: urproto.ProviderType_CHAIN,
				},
				{
					Height: 100,
					Type:   urproto.UpgradeType_GOVERNANCE,
					Status: urproto.UpgradeStatus_ACTIVE,
					Source: urproto.ProviderType_CHAIN,
					// in case of two proposals where one is in active state and the other in non-active state
					// we expect the one in active state
					ProposalId: int64ptr(1),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cosmosClient := &mockCosmosClient{
				v1Proposals: tt.proposals,
			}
			provider := NewProvider(cosmosClient, "test-chain", 1)

			upgrades, err := provider.GetUpgrades(context.Background())
			require.NoError(t, err)

			assert.Equal(t, len(upgrades), len(tt.expected))

			for i, upgrade := range upgrades {
				assert.Equal(t, tt.expected[i].Height, upgrade.Height)
				assert.Equal(t, tt.expected[i].Type, upgrade.Type)
				assert.Equal(t, tt.expected[i].Status, upgrade.Status)
				assert.Equal(t, tt.expected[i].Source, upgrade.Source)

				if tt.expected[i].ProposalId != nil {
					assert.Equal(t, *tt.expected[i].ProposalId, *upgrade.ProposalId)
				}
			}
		})
	}
}

func newProposal(t *testing.T, id uint64, height int64, status v1.ProposalStatus) *v1.Proposal {
	sup := &upgradetypes.MsgSoftwareUpgrade{
		Authority: "x/gov",
		Plan: upgradetypes.Plan{
			Name:   fmt.Sprintf("test upgrade: %d", height),
			Time:   time.Now().Add(30 * time.Minute),
			Info:   "test upgrade info",
			Height: height,
		},
	}

	proposal, err := v1.NewProposal([]sdk.Msg{sup}, id, time.Now(), time.Now(), "", "title", "summary", sdk.AccAddress{})
	require.NoError(t, err)

	proposal.Status = status

	return &proposal
}

func int64ptr(i int64) *int64 {
	return &i
}
