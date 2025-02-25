package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blazar/internal/pkg/errors"

	cstypes "github.com/cometbft/cometbft/consensus/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cometbft "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/query"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultPaginationLimit = query.DefaultLimit

type Client struct {
	tmClient       tmservice.ServiceClient
	v1Client       v1.QueryClient
	v1beta1Client  v1beta1.QueryClient
	cometbftClient *cometbft.HTTP

	isCometbftStarted bool
	timeout           time.Duration
	paginationLimit   uint64
	callOptions       []grpc.CallOption
}

func NewCosmosGrpcOnlyClient(host string, grpcPort uint16, timeout time.Duration) (*Client, error) {
	grpcConn, err := createGrpcConn(host, grpcPort)
	if err != nil {
		return nil, err
	}
	return &Client{
		tmClient:        tmservice.NewServiceClient(grpcConn),
		v1Client:        v1.NewQueryClient(grpcConn),
		v1beta1Client:   v1beta1.NewQueryClient(grpcConn),
		timeout:         timeout,
		paginationLimit: defaultPaginationLimit,
		// https://github.com/cosmos/cosmos-sdk/blob/a86c2a9980ffc4fed1f8c423889e0628193ffaab/server/config/config.go#L140
		callOptions: []grpc.CallOption{grpc.MaxCallRecvMsgSize(math.MaxInt32)},
	}, nil
}

func NewClient(host string, grpcPort uint16, cometbftPort uint16, timeout time.Duration) (*Client, error) {
	grpcConn, err := createGrpcConn(host, grpcPort)
	if err != nil {
		return nil, err
	}

	cometbftClient, err := cometbft.New(fmt.Sprintf("tcp://%s", net.JoinHostPort(host, strconv.Itoa(int(cometbftPort)))), "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cometbft http client")
	}

	return &Client{
		tmClient:        tmservice.NewServiceClient(grpcConn),
		v1Client:        v1.NewQueryClient(grpcConn),
		v1beta1Client:   v1beta1.NewQueryClient(grpcConn),
		cometbftClient:  cometbftClient,
		timeout:         timeout,
		paginationLimit: defaultPaginationLimit,
		// https://github.com/cosmos/cosmos-sdk/blob/a86c2a9980ffc4fed1f8c423889e0628193ffaab/server/config/config.go#L140
		callOptions: []grpc.CallOption{grpc.MaxCallRecvMsgSize(math.MaxInt32)},
	}, nil
}

func (cc *Client) StartCometbftClient() error {
	if cc.isCometbftStarted {
		return nil
	}

	err := cc.cometbftClient.Start()
	if err == nil {
		cc.isCometbftStarted = true
	}
	return err
}

func (cc *Client) GetLatestBlockHeight(ctx context.Context) (int64, error) {
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(ctx, cc.timeout)
	defer cancel()

	res, err := cc.tmClient.GetLatestBlock(ctx, &tmservice.GetLatestBlockRequest{}, cc.callOptions...)
	if err != nil {
		status, err2 := cc.GetStatus(ctx)
		if err2 != nil {
			return 0, errors.Wrapf(err, "failed to get latest block & status")
		}
		height, err := strconv.ParseInt(status.Result.SyncInfo.LatestBlockHeight, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to parse height from status")
		}
		return height, nil
	}

	if res.SdkBlock != nil {
		return res.SdkBlock.Header.Height, nil
	}
	// This is deprecated in sdk v0.47, but many chains don't return the
	// alternative sdk_block structure.
	return res.Block.Header.Height, nil
}

func (cc *Client) GetProposalsV1(ctx context.Context) (v1.Proposals, error) {
	var key []byte
	proposals := make(v1.Proposals, 0, 50)

	for {
		pageCtx, cancel := context.WithTimeout(ctx, cc.timeout)
		res, err := cc.v1Client.Proposals(pageCtx, &v1.QueryProposalsRequest{
			Pagination: &query.PageRequest{
				Key:   key,
				Limit: cc.paginationLimit,
			},
		}, cc.callOptions...)
		cancel()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get proposals")
		}

		proposals = append(proposals, res.Proposals...)
		if key = res.Pagination.NextKey; key == nil {
			break
		}
	}
	return proposals, nil
}

func (cc *Client) GetProposalsV1beta1(ctx context.Context) (v1beta1.Proposals, error) {
	var key []byte
	proposals := make(v1beta1.Proposals, 0, 50)

	for {
		pageCtx, cancel := context.WithTimeout(ctx, cc.timeout)
		res, err := cc.v1beta1Client.Proposals(pageCtx, &v1beta1.QueryProposalsRequest{
			Pagination: &query.PageRequest{
				Key:   key,
				Limit: cc.paginationLimit,
			},
		}, cc.callOptions...)
		cancel()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get proposals")
		}

		proposals = append(proposals, res.Proposals...)
		if key = res.Pagination.NextKey; key == nil {
			break
		}
	}
	return proposals, nil
}

type StatusResponse struct {
	Result struct {
		ValidatorInfo struct {
			Address     string `json:"address"`
			VotingPower string `json:"voting_power"`
		} `json:"validator_info"`
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockTime   string `json:"latest_block_time"`
			LatestBlockHeight string `json:"latest_block_height"`
		} `json:"sync_info"`
	} `json:"result"`
}

func (cc *Client) GetStatus(ctx context.Context) (*StatusResponse, error) {
	// This code is manually deserializing the raw JSON output of the /status endpoint
	// instead of using `cc.cometbftClient.Status(ctx)` because certain chains
	// are using BLS12-381 keys, which are not part of the support enum for key format
	// on the cometbft proto definitions, defined at
	// https://github.com/cometbft/cometbft/blob/v0.38.17/proto/tendermint/crypto/keys.proto#L13
	// BLS12-381 support was added to the enum on 1.0.0, in this commit
	// https://github.com/cometbft/cometbft/commit/354c6bedd35a5825accb9defd60d65e27c6de643
	// but 1.0.0 is not yet usable in this context; cosmossdk needs to release a new version for it

	statusURL := strings.Replace(fmt.Sprintf("%s/status", cc.cometbftClient.Remote()), "tcp://", "http://", 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request")
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to perform HTTP request")
	}
	defer resp.Body.Close()

	var statusResp StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode JSON")
	}
	return &statusResp, nil
}

func (cc *Client) NodeInfo(ctx context.Context) (*tmservice.GetNodeInfoResponse, error) {
	response, err := cc.tmClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{}, cc.callOptions...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node info")
	}
	return response, nil
}

func (cc *Client) GetCometbftClient() *cometbft.HTTP {
	return cc.cometbftClient
}

type PrevoteInfo struct {
	Height int64
	Round  int32
	Step   uint8
	// these are int64 in cometbft
	TotalVP  int64
	OnlineVP int64
}

func (cc *Client) GetPrevoteInfo(ctx context.Context) (*PrevoteInfo, error) {
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(ctx, cc.timeout)
	defer cancel()

	// DumpConsensusState returns data in a more structured manner
	// but unfortunately it is broken on certain versions of tendermint
	// https://github.com/cometbft/cometbft/issues/863
	// Also in my experiments, reloading the /dump_consensus_state route
	// always shows all nil-votes for all prevotes, hence I am not sure if
	// that is reliable. It is meant for debugging so maybe it updated only
	// on certain conditions or it is just too slow. On the other hand,
	// /consensus_state route shows prevotes information even when the other
	// route shows nil-votes for everyone, so this seems more reliable.
	// try running:
	// watch -n 0.2 "curl -s http://ip:port/dump_consensus_state | jq .result.round_state.votes[0].prevotes_bit_array"
	// and
	// watch -n 0.2 "curl -s http://ip:port/consensus_state | jq .result.round_state.height_vote_set[0].prevotes_bit_array"
	// to see my observations
	//
	// Additionally, the actual vote information is serialised as a private
	// struct with no unmarshaller so we cannot deserialize it without writing our own unmarshaller
	// https://github.com/cometbft/cometbft/blob/v0.37.2/consensus/types/height_vote_set.go#L261
	// https://github.com/cometbft/cometbft/blob/v0.37.2/consensus/types/height_vote_set.go#L238

	res, err := cc.cometbftClient.ConsensusState(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get consensus state")
	}
	var roundState cstypes.RoundStateSimple
	if err := cmtjson.Unmarshal(res.RoundState, &roundState); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal round_state")
	}

	parts := strings.Split(roundState.HeightRoundStep, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("failed to parse height_round_step=%s", roundState.HeightRoundStep)
	}
	height, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse height=%s", parts[0])
	}
	roundI64, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse round=%s", parts[1])
	}
	stepU64, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse step=%s", parts[2])
	}
	round, step := int32(roundI64), uint8(stepU64)

	voteSets := []struct {
		// we only need one field
		PrevotesBitArray string `json:"prevotes_bit_array"`
	}{}
	if err := cmtjson.Unmarshal(roundState.Votes, &voteSets); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal height_vote_set")
	}

	if len(voteSets) <= int(round) {
		// this should never be hit, but just in case
		return nil, fmt.Errorf("len(height_vote_set)=%d <= round=%d", len(voteSets), round)
	}
	currPrevotes := voteSets[round].PrevotesBitArray
	// structure for reference:
	// "BA{100:____________________________________________________________________________________________________} 0/151215484 = 0.00
	//  We want this ->                                                                                              ^^^^^^^^^^^
	parts = strings.Split(currPrevotes, " ")
	if len(parts) != 4 {
		return nil, fmt.Errorf("unrecognized prevotes_bit_array format: %s", currPrevotes)
	}
	parts = strings.Split(parts[1], "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unrecognized prevotes_bit_array format: %s", currPrevotes)
	}
	onlineVP, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse online vp=%s", parts[0])
	}
	totalVP, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse total vp=%s", parts[1])
	}
	return &PrevoteInfo{
		Height:   height,
		Round:    round,
		Step:     step,
		TotalVP:  totalVP,
		OnlineVP: onlineVP,
	}, nil
}

func getCodec() *codec.ProtoCodec {
	ir := types.NewInterfaceRegistry()
	std.RegisterInterfaces(ir)

	return codec.NewProtoCodec(ir)
}

func createGrpcConn(host string, grpcPort uint16) (*grpc.ClientConn, error) {
	grpcConn, err := grpc.NewClient(
		net.JoinHostPort(host, strconv.Itoa(int(grpcPort))),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(getCodec().GRPCCodec())),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to grpc")
	}
	return grpcConn, nil
}
