package checks

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"

	"github.com/cometbft/cometbft/libs/bytes"
)

type CheckBlockStatus int

const (
	// Should not be possible - but prevents usage of a default value on a CheckBlockStatus
	InvalidBlockState CheckBlockStatus = iota
	// Blazar could not observe whether we voted in time (chain passed upgrade height before the check)
	BlockSkipped
	// Blazar observed the validator's signature in the pre-vote
	BlockSigned
	// Blazar has not yet observed the validator's signature in the pre-vote
	BlockNotSignedYet
)

func GrpcResponsive(ctx context.Context, cosmosClient *cosmos.Client, cfg *config.GrpcResponsive) (int64, error) {
	logger := log.FromContext(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	height := int64(0)
	timeout := time.NewTimer(cfg.Timeout)

	grpcResponsive, cometbftResponsive := false, false

	for {
		select {
		case <-ticker.C:
			if !grpcResponsive {
				// lets test if the status endpoint is working
				var err error
				height, err = cosmosClient.GetLatestBlockHeight(ctx)
				if err != nil {
					logger.Err(err).Warn("Grpc endpoint gives an error, will retry")
				} else {
					if height > 0 {
						grpcResponsive = true
					} else {
						// this should never reach but just in case
						return 0, fmt.Errorf("grpc endpoint is now responsive but observed chain height=%d <= 0, assuming upgrade failed", height)
					}
				}
			}
			if !cometbftResponsive {
				// lets test if the /consensus_state endpoint is working
				var err error
				pvp, err := cosmosClient.GetPrevoteInfo(ctx)
				if err != nil {
					logger.Err(err).Warn("Cometbft endpoint gives an error, will retry")
				} else {
					if pvp.TotalVP > 0 {
						cometbftResponsive = true
					} else {
						// this should never reach but just in case
						return 0, fmt.Errorf("cometbft endpoint is now responsive but observed total VP=%d <= 0, assuming upgrade failed", pvp.TotalVP)
					}
				}
			}
			if cometbftResponsive && grpcResponsive {
				logger.Infof("Post upgrade check passed, grpc and cometbft services are now responsive, observed chain height: %d.", height).Notify(ctx)
				return height, nil
			}
		case <-timeout.C:
			return 0, fmt.Errorf("services responsiveness post-upgrade check timed out after %s with status grpc responsive=%t cometbft responsive=%t, assuming upgrade failed", cfg.Timeout.String(), grpcResponsive, cometbftResponsive)
		case <-ctx.Done():
			return 0, errors.Wrapf(ctx.Err(), "grpc responsiveness post-upgrade check cancelled due to context timeout")
		}
	}
}

func ChainHeightIncreased(ctx context.Context, cosmosClient *cosmos.Client, cfg *config.ChainHeightIncreased, upgradeHeight int64) error {
	logger := log.FromContext(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	vpReportTicker := time.NewTicker(cfg.NotifInterval)
	timeout := time.NewTimer(cfg.Timeout)

	for {
		select {
		case <-vpReportTicker.C:
			pvp, err := cosmosClient.GetPrevoteInfo(ctx)
			if err != nil {
				logger.Err(err).Warn("Error in getting prevote vp, will retry")
				continue
			}

			switch {
			case pvp.Height == upgradeHeight+1:
				logger.Infof("Post upgrade check: height did not increase yet. Prevote status: online VP=%d total VP=%d 2/3+1 VP=%f online VP ratio=%f", pvp.OnlineVP, pvp.TotalVP, (2.0*float32(pvp.TotalVP))/3.0, float32(pvp.OnlineVP)/float32(pvp.TotalVP)).Notify(ctx)
			case pvp.Height > upgradeHeight+1:
				logger.Infof("Queried for prevote VP but height observed=%d > upgrade height=%d, skipping notification as this post upgrade check should pass soon", pvp.Height, upgradeHeight)
			default:
				// this should never be hit
				return fmt.Errorf("height decreased while querying for prevote vp: %d, assuming upgrade failed", pvp.Height)
			}
		case <-ticker.C:
			// we rely on another endpoint for height, because I don't
			// really trust the /consensus_state endpoint yet.
			newHeight, err := cosmosClient.GetLatestBlockHeight(ctx)
			if err != nil {
				logger.Err(err).Warn("Grpc endpoint gives an error, will retry")
				continue
			}
			if newHeight > upgradeHeight {
				logger.Infof("Post upgrade check passed, chain height increased, newly observed chain height: %d. All Post upgrade checks passed.", newHeight).Notify(ctx)
				return nil
			}

			if newHeight == upgradeHeight {
				logger.Info("Height didn't increase yet, will retry")
			} else {
				// this should never reach but just in case
				return fmt.Errorf("height decreased after grpc endpoint became responsive: %d, assuming upgrade failed", newHeight)
			}
		case <-timeout.C:
			return fmt.Errorf("height increase post-upgrade check timed out after %s, assuming upgrade failed", cfg.Timeout.String())
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "height increase post-upgrade check cancelled due to context timeout")
		}
	}
}

type RoundState struct {
	HeightRoundStep   string    `json:"height/round/step"`
	StartTime         time.Time `json:"start_time"`
	ProposalBlockHash string    `json:"proposal_block_hash"`
	LockedBlockHash   string    `json:"locked_block_hash"`
	ValidBlockHash    string    `json:"valid_block_hash"`
	HeightVoteSet     []struct {
		Round              int      `json:"round"`
		Prevotes           []string `json:"prevotes"`
		PrevotesBitArray   string   `json:"prevotes_bit_array"`
		Precommits         []string `json:"precommits"`
		PrecommitsBitArray string   `json:"precommits_bit_array"`
	} `json:"height_vote_set"`
	Proposer struct {
		Address string `json:"address"`
		Index   int    `json:"index"`
	} `json:"proposer"`
}

type PreVote struct {
	SignaturePrefix string
}

var prevoteRegex = regexp.MustCompile(`Vote\{\d+:([A-Fa-f0-9]+)\s`)

func ParsePreVote(s string) (PreVote, error) {
	matches := prevoteRegex.FindStringSubmatch(s)
	if len(matches) < 2 {
		return PreVote{}, errors.New("signature not found in prevote string")
	}

	return PreVote{
		SignaturePrefix: matches[1],
	}, nil
}

func HasAddressSigned(address bytes.HexBytes, rs RoundState) (bool, error) {
	validatorAddress := strings.ToUpper(hex.EncodeToString(address))
	for _, voteSet := range rs.HeightVoteSet {
		for _, preVoteStr := range voteSet.Prevotes {
			if preVoteStr == "nil-Vote" {
				continue
			}
			preVote, err := ParsePreVote(preVoteStr)
			if err != nil {
				return false, errors.Wrapf(err, "Could not parse prevote '%s': %v", preVoteStr)
			}
			upperPrefix := strings.ToUpper(preVote.SignaturePrefix)
			if strings.HasPrefix(validatorAddress, upperPrefix) {
				return true, nil
			}
		}
	}
	return false, nil
}

func CheckBlockSignedBy(address bytes.HexBytes, height int64, consensusState json.RawMessage) (CheckBlockStatus, error) {
	var rs RoundState
	if err := json.Unmarshal(consensusState, &rs); err != nil {
		return InvalidBlockState, errors.Wrapf(err, "Error in parsing consensus state: %v, will retry")
	}
	parts := strings.Split(rs.HeightRoundStep, "/")
	currentHeight, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return InvalidBlockState, errors.Wrapf(err, "Error in parsing height from consensus state")
	}

	if currentHeight > height {
		return BlockSkipped, nil
	}

	signed, err := HasAddressSigned(address, rs)

	if err != nil {
		return InvalidBlockState, err
	}
	if signed {
		return BlockSigned, nil
	}
	return BlockNotSignedYet, nil
}

func NextBlockSignedPostCheck(ctx context.Context, cosmosClient *cosmos.Client, postUpgradeChecks *config.FirstBlockVoted, observedHeight int64) error {
	logger := log.FromContext(ctx)

	ticker := time.NewTicker(postUpgradeChecks.PollInterval)
	notifTicker := time.NewTicker(postUpgradeChecks.NotifInterval)
	timeout := time.NewTimer(postUpgradeChecks.Timeout)

	status, err := cosmosClient.GetCometbftClient().Status(ctx)
	if err != nil {
		return errors.Wrapf(err, "Could not get node status")
	}

	nodeAddress := status.ValidatorInfo.Address
	logger.Infof("Post upgrade check 2: Waiting to sign the first block after upgrade=%d, address=%s", observedHeight, nodeAddress.String()).Notify(ctx)
	if status.ValidatorInfo.VotingPower == 0 {
		logger.Info("Post upgrade check 2: skipping signature check, as VP is 0").Notify(ctx)
		return nil
	}
	for {
		select {
		case <-notifTicker.C:
			logger.Info("Post upgrade check: block not signed yet.").Notify(ctx)
		case <-ticker.C:
			consensusState, err := cosmosClient.GetCometbftClient().ConsensusState(ctx)
			if err != nil {
				logger.Err(err).Warn("Error in getting consensus state, will retry")
				continue
			}
			state, err := CheckBlockSignedBy(nodeAddress, observedHeight, consensusState.RoundState)
			if err != nil {
				logger.Err(err).Warn("Error checking if we voted, will retry")
				continue
			}
			switch state {
			case BlockSkipped:
				logger.Info("Post upgrade check 2 inconclusive, height increased before we could observe our own vote").Notify(ctx)
				return nil
			case BlockSigned:
				logger.Info("Post upgrade check 2 successful, observed our own signature on the upgrade block").Notify(ctx)
				return nil
			case BlockNotSignedYet:
				continue
			default:
				panic(fmt.Sprintf("programming error: state from block at %d was %d, which is illegal", observedHeight, state))
			}
		case <-timeout.C:
			return fmt.Errorf("post-upgrade check for fist block signature timed out after %s, assuming upgrade failed", postUpgradeChecks.Timeout.String())
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "post-upgrade check for first block signature cancelled due to context timeout")
		}
	}
}
