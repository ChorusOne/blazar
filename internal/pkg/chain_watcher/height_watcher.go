package chain_watcher

import (
	"context"
	"time"

	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"

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

	logger := log.FromContext(ctx)

	go func() {
		for {
			select {
			case <-ticker.C:
				height, err := cosmosClient.GetLatestBlockHeight(ctx)

				if err != nil {
					logger.Debugf("Height watcher observed an error: %v", err)
				} else {
					logger.Debugf("Height watcher observed new height: %d", height)
				}

				select {
				case heights <- NewHeight{
					Height: height,
					Error:  err,
				}:

				// prevents deadlock with heights channel
				case <-cancel:
					logger.Debug("Height watcher exiting")
					return
				}
			// this isn't necessary since we exit in the above select statement
			// but this will help in early exit in case cancel is called before the ticker fires
			case <-cancel:
				logger.Debug("Height watcher exiting")
				return
			}
		}
	}()

	return &HeightWatcher{
		Heights: heights,
		cancel:  cancel,
	}
}

func NewStreamingHeightWatcher(ctx context.Context, cosmosClient *cosmos.Client, timeout time.Duration) (*HeightWatcher, error) {
	cancel := make(chan struct{})
	heights := make(chan NewHeight)

	// subscribe call hangs if the node is not running, this at least prevents
	// the watcher from hanging at the start
	if _, err := cosmosClient.GetStatus(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to get cometbft status")
	}

	// create some wiggle room in case blazar can't process the blocks fast enough
	capacity := 10

	name, query := "blazar-client", "tm.event = 'NewBlock'"

	txs, err := cosmosClient.GetCometbftClient().Subscribe(ctx, name, query, capacity)
	if err != nil {
		return nil, err
	}

	logger := log.FromContext(ctx)

	go func() {
		var lastHeight int64
		for {
			timeout := time.NewTimer(timeout)

			select {
			case <-timeout.C:
				logger.Warn("Height watcher has been stuck for too long, checking if chain is stuck")
				var height int64
				for {
					// We will keep retrying until we get a height. There are too many things that can go wrong so we
					// return errors to the channel which will make the error metric go up, hence informaing the user
					// that something is wrong
					height, err = cosmosClient.GetLatestBlockHeight(ctx)
					if err != nil {
						select {
						case heights <- NewHeight{
							Height: 0,
							Error:  errors.Wrapf(err, "error querying chain for height"),
						}:
						// prevents deadlock with heights channel
						case <-cancel:
							return
						}
						time.Sleep(time.Second)
					} else {
						break
					}
				}
				if height == lastHeight {
					logger.Warn("Chain is stuck, I will NOT re-create ws subscription")
					continue
				}
				logger.Warnf("Chain is moving, latest height seen by subscription: %d, latest height seen on chain: %d. Re-creating ws subscription", lastHeight, height)
				if err = cosmosClient.GetCometbftClient().Unsubscribe(ctx, name, query); err != nil {
					logger.Warnf("Failed to unsubscribe from websocket, continuing anyways: %v", err)
				}
				for {
					// Similar approach as the height polling
					txs, err = cosmosClient.GetCometbftClient().Subscribe(ctx, name, query, capacity)
					if err != nil {
						select {
						case heights <- NewHeight{
							Height: 0,
							Error:  errors.Wrapf(err, "timeout loop failed re-creating ws subscription"),
						}:
						// prevents deadlock with heights channel
						case <-cancel:
							return
						}
						time.Sleep(time.Second)
					} else {
						break
					}
				}
				logger.Infof("Re-created ws subscription")

			case tx := <-txs:
				if data, ok := tx.Data.(ctypes.EventDataNewBlock); ok {
					lastHeight = data.Block.Header.Height
					logger.Debugf("Height watcher observed new height: %d", lastHeight)
					select {
					case heights <- NewHeight{
						Height: lastHeight,
						Error:  nil,
					}:
					// prevents deadlock with heights channel
					case <-cancel:
						logger.Debug("Height watcher exiting")
						return
					}
				}
			// this isn't necessary since we exit in the above select statement
			// but this will help in early exit in case cancel is called before the new height fires
			case <-cancel:
				logger.Debug("Height watcher exiting")
				return
			}
		}
	}()
	return &HeightWatcher{
		Heights: heights,
		cancel:  cancel,
	}, nil
}
