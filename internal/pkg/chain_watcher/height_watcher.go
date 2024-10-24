package chain_watcher

import (
	"context"
	"time"

	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"

	ctypes "github.com/cometbft/cometbft/types"
)

type NewHeight struct {
	Height int64
	Error  error
}

type HeightWatcher struct {
	Heights <-chan NewHeight
	cancel  chan<- struct{}
}

func (hw *HeightWatcher) Cancel() {
	hw.cancel <- struct{}{}
}

func NewPeriodicHeightWatcher(ctx context.Context, cosmosClient *cosmos.Client, heightInterval time.Duration) *HeightWatcher {
	ticker := time.NewTicker(heightInterval)
	cancel := make(chan struct{})
	heights := make(chan NewHeight)

	go func() {
		for {
			select {
			case <-ticker.C:
				height, err := cosmosClient.GetLatestBlockHeight(ctx)

				select {
				case heights <- NewHeight{
					Height: height,
					Error:  err,
				}:

				// prevents deadlock with heights channel
				case <-cancel:
					return
				}
			// this isn't necessary since we exit in the above select statement
			// but this will help in early exit in case cancel is called before the ticker fires
			case <-cancel:
				return
			}
		}
	}()

	return &HeightWatcher{
		Heights: heights,
		cancel:  cancel,
	}
}

func NewStreamingHeightWatcher(ctx context.Context, cosmosClient *cosmos.Client) (*HeightWatcher, error) {
	cancel := make(chan struct{})
	heights := make(chan NewHeight)

	// subscribe call hangs if the node is not running, this at least prevents
	// the watcher from hanging at the start
	if _, err := cosmosClient.GetCometbftClient().Status(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to get cometbft status")
	}

	// create some wiggle room in case blazar can't process the blocks fast enough
	capacity := 10

	txs, err := cosmosClient.GetCometbftClient().Subscribe(ctx, "blazar-client", "tm.event = 'NewBlock'", capacity)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case tx := <-txs:
				if data, ok := tx.Data.(ctypes.EventDataNewBlock); ok {
					select {
					case heights <- NewHeight{
						Height: data.Block.Header.Height,
						Error:  nil,
					}:
					// prevents deadlock with heights channel
					case <-cancel:
						return
					}
				}
			// this isn't necessary since we exit in the above select statement
			// but this will help in early exit in case cancel is called before the new height fires
			case <-cancel:
				return
			}
		}
	}()
	return &HeightWatcher{
		Heights: heights,
		cancel:  cancel,
	}, nil
}
