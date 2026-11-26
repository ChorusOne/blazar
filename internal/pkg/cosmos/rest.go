package cosmos

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"blazar/internal/pkg/errors"

	"github.com/cosmos/cosmos-sdk/types/query"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/gogoproto/jsonpb"
	"github.com/cosmos/gogoproto/proto"
)

// queryWithRESTFallback gives each transport its own timeout and tries REST only
// when gRPC fails and a REST endpoint is configured.
func queryWithRESTFallback[T proto.Message](ctx context.Context, cc *Client, path string, pagination *query.PageRequest, restResponse T, grpcQuery func(context.Context) (T, error)) (T, error) {
	grpcCtx, cancel := context.WithTimeout(ctx, cc.timeout)
	response, grpcErr := grpcQuery(grpcCtx)
	cancel()
	if grpcErr == nil {
		return response, nil
	}
	if cc.restURL == "" {
		return response, grpcErr
	}

	restCtx, cancel := context.WithTimeout(ctx, cc.timeout)
	restErr := cc.getREST(restCtx, path, pagination, restResponse)
	cancel()
	if restErr != nil {
		return restResponse, errors.Wrapf(errors.Join(grpcErr, restErr), "both gRPC and REST queries failed")
	}
	return restResponse, nil
}

func (cc *Client) getREST(ctx context.Context, path string, pagination *query.PageRequest, response proto.Message) error {
	params := url.Values{}
	if pagination != nil {
		params.Set("pagination.key", base64.StdEncoding.EncodeToString(pagination.Key))
		params.Set("pagination.limit", strconv.FormatUint(pagination.Limit, 10))
		params.Set("pagination.reverse", strconv.FormatBool(pagination.Reverse))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cc.restURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create REST request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "REST query %s failed", path)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("REST query %s returned %s: %s", path, resp.Status, body)
	}
	decoder := jsonpb.Unmarshaler{AnyResolver: upgradeResolver{}, AllowUnknownFields: true}
	if err := decoder.Unmarshal(resp.Body, response); err != nil {
		return errors.Wrapf(err, "failed to decode REST response from %s", path)
	}
	return nil
}

// Only upgrade messages need their contents decoded, just as the gRPC provider
// only inspects their Any values. Other messages may use chain-specific types.
type upgradeResolver struct{}

func (upgradeResolver) Resolve(typeURL string) (proto.Message, error) {
	switch typeURL {
	case "/cosmos.gov.v1.MsgExecLegacyContent":
		return &v1.MsgExecLegacyContent{}, nil
	case "/cosmos.upgrade.v1beta1.SoftwareUpgradeProposal":
		return &upgradetypes.SoftwareUpgradeProposal{}, nil
	case "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade":
		return &upgradetypes.MsgSoftwareUpgrade{}, nil
	default:
		return &ignoredRESTMessage{}, nil
	}
}

// An empty protobuf message lets jsonpb skip unrelated Any content while
// preserving its type URL for the provider's existing filtering.
type ignoredRESTMessage struct{}

func (*ignoredRESTMessage) Reset()         {}
func (*ignoredRESTMessage) String() string { return "" }
func (*ignoredRESTMessage) ProtoMessage()  {}
