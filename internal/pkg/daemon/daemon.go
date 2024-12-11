package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"blazar/internal/pkg/chain_watcher"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/daemon/checks"
	"blazar/internal/pkg/docker"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"
	"blazar/internal/pkg/log/notification"
	"blazar/internal/pkg/metrics"
	blazarproto "blazar/internal/pkg/proto/blazar"
	checksproto "blazar/internal/pkg/proto/daemon"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	sm "blazar/internal/pkg/state_machine"
	"blazar/internal/pkg/upgrades_registry"

	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Daemon struct {
	// extenral clients
	dcc          *docker.ComposeClient
	dc           *docker.Client
	cosmosClient *cosmos.Client

	// internal state handling
	ur           *upgrades_registry.UpgradeRegistry
	stateMachine *sm.StateMachine

	// telemetry
	metrics *metrics.Metrics

	// initial counters
	startupHeight int64
	nodeAddress   bytes.HexBytes
	nodeInfo      *tmservice.GetNodeInfoResponse

	// tracking current height
	currHeight          int64
	currHeightTime      time.Time
	observedBlockSpeeds []time.Duration
	currBlockSpeed      time.Duration
}

func NewDaemon(ctx context.Context, cfg *config.Config, m *metrics.Metrics) (*Daemon, error) {
	if _, err := docker.LoadComposeFile(cfg.ComposeFile); err != nil {
		return nil, errors.Wrapf(err, "failed to parse docker compose file")
	}

	// setup updates registry
	ur, err := upgrades_registry.NewUpgradesRegistryFromConfig(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load upgrade registry")
	}

	// setup docker compose client
	dc, err := docker.NewClientWithConfig(ctx, cfg.CredentialHelper)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create docker client")
	}
	dcc, err := docker.NewComposeClient(dc, cfg.VersionFile, cfg.ComposeFile, cfg.UpgradeMode)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create docker compose client")
	}

	// setup new cosmos client
	cosmosClient, err := cosmos.NewClient(cfg.Clients.Host, cfg.Clients.GrpcPort, cfg.Clients.CometbftPort, cfg.Clients.Timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create cosmos client")
	}

	if err := cosmosClient.StartCometbftClient(); err != nil {
		return nil, errors.Wrapf(err, "failed to start cometbft client")
	}

	return &Daemon{
		dcc:          dcc,
		dc:           dc,
		cosmosClient: cosmosClient,
		metrics:      m,

		// setup by Init()
		startupHeight:       0,
		currHeight:          0,
		currHeightTime:      time.Time{},
		observedBlockSpeeds: make([]time.Duration, 5),
		currBlockSpeed:      0,

		ur:           ur,
		stateMachine: ur.GetStateMachine(),
	}, nil
}

func (d *Daemon) Init(ctx context.Context, cfg *config.Config) error {
	logger := log.FromContext(ctx).With("package", "daemon")
	logger.Info("Starting up blazar daemon...")

	// mark the daemon is up
	d.metrics.Up.Set(1)

	// test docker and docker compose
	logger.Info("Setting up docker and docker compose clients")
	if _, err := d.dcc.DockerClient().ContainerList(ctx, true); err != nil {
		return errors.Wrapf(err, "failed to fetch list of containers from docker client")
	}

	if _, err := d.dcc.Version(ctx); err != nil {
		return errors.Wrapf(err, "could not find docker compose plugin")
	}

	// test cosmos client
	logger.Info("Attempting to get data from /status endpoint with Cosmos RPC client")
	status, err := d.cosmosClient.GetStatus(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get status response")
	}

	// display information about the node
	d.nodeInfo, err = d.cosmosClient.NodeInfo(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get node info")
	}
	logger.Infof("Connected to the %s node ID: %s", d.nodeInfo.ApplicationVersion.Name, d.nodeInfo.DefaultNodeInfo.DefaultNodeID)

	// if the env prefix is not set, we set it to <APP_NAME>_ (e.g "GAIAD_")
	if cfg.Compose.EnvPrefix == "" {
		cfg.Compose.EnvPrefix = strings.ToUpper(d.nodeInfo.ApplicationVersion.AppName) + "_"
	}
	logger.Infof("Using env prefix: %s", cfg.Compose.EnvPrefix)

	// ensure required settings and flags are present in the compose file
	if err := validateComposeSettings(cfg); err != nil {
		return errors.Wrapf(err, "failed to validate docker compose settings")
	}

	logger.Infof("Observed latest block height: %d", status.SyncInfo.LatestBlockHeight)
	d.currHeight = status.SyncInfo.LatestBlockHeight
	d.currHeightTime = status.SyncInfo.LatestBlockTime
	d.startupHeight = d.currHeight

	logger.Infof("Observed node address: %s", status.ValidatorInfo.Address.String())
	d.nodeAddress = status.ValidatorInfo.Address

	// test consensus state endpoint
	logger.Info("Attempting to get consensus state")
	pvp, err := d.cosmosClient.GetPrevoteInfo(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get consensus state")
	}
	logger.Infof(
		"Total VP: %d, Node VP: %d, Node share: %.2f", pvp.TotalVP, status.ValidatorInfo.VotingPower,
		(float64(status.ValidatorInfo.VotingPower)/float64(pvp.TotalVP))*100,
	)

	// fetch future upgrades
	logger.Info("Attempting to fetch upgrades from all providers")
	if _, _, _, _, err := d.ur.Update(ctx, d.currHeight, true); err != nil {
		return errors.Wrapf(err, "failed getting upgrades from all providers")
	}

	totalUpgrades := d.ur.GetAllUpgradesWithCache()
	logger.Infof("Total %d resolved upgrades from all providers", len(totalUpgrades))

	overridenUpgrades := d.ur.GetOverriddenUpgradesWithCache()
	logger.Infof("Total %d upgrades had more than one entry, blazar picked one with the lowest priority", len(overridenUpgrades))

	upgrades := d.ur.GetUpcomingUpgradesWithCache(d.currHeight, urproto.UpgradeStatus_ACTIVE)
	logger.Infof("Found %d future upgrades with status ACTIVE (waiting for execution)", len(upgrades))

	for _, upgrade := range upgrades {
		if _, ok := overridenUpgrades[upgrade.Height]; ok {
			logger.Infof("[height=%d] The proposal from: %s with name '%s' and tag '%s' won by priority (%d) with %d other entries", upgrade.Height, upgrade.Source, upgrade.Name, upgrade.Tag, upgrade.Priority, len(overridenUpgrades[upgrade.Height]))
		}
	}

	// export metrics related to all future proposal
	d.updateMetrics()

	return nil
}

func (d *Daemon) ListenAndServe(ctx context.Context, cfg *config.Config) error {
	httpAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.HTTPPort)))
	grpcAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.GrpcPort)))

	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return errors.Wrapf(err, "error listening on grpc address")
	}

	server := grpc.NewServer()
	urServer := NewServer(cfg, d.ur)
	urproto.RegisterUpgradeRegistryServer(server, urServer)
	vrproto.RegisterVersionResolverServer(server, urServer)
	blazarproto.RegisterBlazarServer(server, urServer)

	go func() {
		logger := log.FromContext(ctx)
		if err = server.Serve(grpcListener); err != nil {
			logger.Err(err).Error("error serving grpc server")
			panic(err)
		}
	}()

	// lets wait for the server to start
	time.Sleep(time.Second)

	grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrapf(err, "couldn't dial to self grpc address")
	}

	mux := runtime.NewServeMux()

	err = urproto.RegisterUpgradeRegistryHandler(ctx, mux, grpcConn)
	if err != nil {
		return errors.Wrapf(err, "failed registering upgrades registry handler")
	}

	err = vrproto.RegisterVersionResolverHandler(ctx, mux, grpcConn)
	if err != nil {
		return errors.Wrapf(err, "failed registering versions resolver handler")
	}

	err = blazarproto.RegisterBlazarHandler(ctx, mux, grpcConn)
	if err != nil {
		return errors.Wrapf(err, "failed registering blazar handler")
	}

	if err = metrics.RegisterHandler(mux); err != nil {
		return errors.Wrapf(err, "failed registering metrics handler")
	}

	if err = RegisterIndexHandler(mux, d, cfg.Watchers.UPInterval); err != nil {
		return errors.Wrapf(err, "failed registering status handler")
	}

	// start the http server
	// this is used by metrics and upgrades registry
	go func() {
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			fmt.Println("error serving http server", err)
			panic(err)
		}
	}()
	return nil
}

func (d *Daemon) Run(ctx context.Context, cfg *config.Config) error {
	logger := log.FromContext(ctx).With("compose file", cfg.ComposeFile)
	ctx = logger.WithContext(ctx)

	for {
		// step 0: wait for upgrade height
		upgradeHeight, err := d.waitForUpgrade(ctx, cfg)
		if err != nil {
			// failure to wait for the upgrade is a critical error, therefore we stop the daemon
			logger.Err(err).Error("Monitor routine failed").Notify(ctx)
			return errors.Wrapf(err, "monitor routine failed")
		}

		// step 0a: setup the context with the upgrade height for all further notifications
		ctxWithHeight := notification.WithUpgradeHeight(ctx, upgradeHeight)

		// step 1: perform upgrade
		err = d.performUpgrade(ctxWithHeight, &cfg.Compose, cfg.ComposeService, upgradeHeight)
		d.updateMetrics()

		if err != nil {
			ctxWithHeight := notification.WithUpgradeHeight(ctx, upgradeHeight)
			logger.Err(err).Error("Upgrade routine failed").Notify(ctxWithHeight)

			// failure to perform upgrade is not a critical errors, therefore we let the daemon continue to run
			continue
		}

		// step 2: wait for post-upgrade checks
		err = d.postUpgradeChecks(ctxWithHeight, d.stateMachine, &cfg.Checks.PostUpgrade, upgradeHeight)
		d.updateMetrics()

		if err != nil {
			ctxWithHeight := notification.WithUpgradeHeight(ctx, upgradeHeight)
			logger.Err(err).Error("Post-upgrade check failed").Notify(ctxWithHeight)

			// failure of post-upgrade check is not a critical error, therefore we let the daemon continue to run
			continue
		}

		// step 3: mark upgrade as completed
		d.stateMachine.MustSetStatus(upgradeHeight, urproto.UpgradeStatus_COMPLETED)
		d.updateMetrics()
	}
}

func (d *Daemon) waitForUpgrade(ctx context.Context, cfg *config.Config) (int64, error) {
	logger := log.FromContext(ctx)

	logger.Infof("Monitoring %s for new upgrades", cfg.UpgradeInfoFilePath())
	uiw, err := chain_watcher.NewUpgradeInfoWatcher(cfg.UpgradeInfoFilePath(), cfg.Watchers.UIInterval)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to start upgrade-info.json poller")
	}

	logger.Info("Monitoring on-chain latest block height")
	var hw *chain_watcher.HeightWatcher
	if cfg.Watchers.HInterval > 0 {
		hw = chain_watcher.NewPeriodicHeightWatcher(ctx, d.cosmosClient, cfg.Watchers.HInterval)
	} else {
		hw, err = chain_watcher.NewStreamingHeightWatcher(ctx, d.cosmosClient)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to start streaming height watcher")
		}
	}

	logger.Info("Monitoring on-chain upgrade proposals")
	upw := chain_watcher.NewUpgradeProposalsWatcher(ctx, d.cosmosClient, d.ur, cfg.Watchers.UPInterval)

	// blockDelta is used to print the current block height every 10 blocks
	blockDelta := int64(0)

	for {
		select {
		case newHeight := <-hw.Heights:
			if newHeight.Error != nil {
				d.metrics.HwErrs.Inc()
				logger.Err(err).Error("Error received from HeightWatcher")
				continue
			}
			currBlockHeight := newHeight.Height
			lastBlockHeight := d.currHeight

			// update the block speed and height
			d.updateHeightAndBlockSpeed(currBlockHeight)

			// display the current block height every 10 blocks (unless in debug mode)
			blockDelta += (currBlockHeight - lastBlockHeight)
			if blockDelta >= 10 {
				blockDelta = 0

				logger.Infof("Current observed height: %d", currBlockHeight)
			} else {
				logger.Debugf("Current observed height: %d", currBlockHeight)
			}

			// move to core logic
			d.updateMetrics()

			upcomingUpgrades := d.ur.GetUpcomingUpgradesWithCache(d.currHeight, urproto.UpgradeStatus_ACTIVE)
			if len(upcomingUpgrades) > 0 {
				futureUpgrade := upcomingUpgrades[0]

				// this case should ideally not happen, unless the upgrade status is not persisted and blazar is restarted
				if d.startupHeight > futureUpgrade.Height {
					logger.Warnf("Skipping upgrade at height %d since it is before daemon startup height %d", futureUpgrade.Height, d.startupHeight)
					continue
				}

				// let the user know that blazar sees the upcoming upgrade
				if d.stateMachine.GetStep(futureUpgrade.Height) == urproto.UpgradeStep_NONE {
					d.stateMachine.SetStep(futureUpgrade.Height, urproto.UpgradeStep_MONITORING)
				}

				// perform pre upgrade upgrade checks if we are close to the upgrade height
				if futureUpgrade.Height < d.currHeight+cfg.Checks.PreUpgrade.Blocks {
					newHeight, preErr := d.preUpgradeChecks(ctx, d.currHeight, d.stateMachine, d.dcc, &cfg.Compose, &cfg.Checks.PreUpgrade, cfg.ComposeService, futureUpgrade)
					if preErr != nil {
						d.stateMachine.MustSetStatus(futureUpgrade.Height, urproto.UpgradeStatus_FAILED)
					}
					d.updateMetrics()

					// cheat and update the height if we have a new height
					if newHeight != 0 {
						logger.Infof("Setting observed height to: %d", futureUpgrade.Height)
						d.currHeight = newHeight
					}
				}

				// perform upgrade if we have hit the upgrade height
				// NOTE: Governance coordinated upgrades are triggered by the upgrade info watcher (upgrade-info.json)
				if futureUpgrade.Height <= d.currHeight && slices.Contains([]urproto.UpgradeType{
					urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
					urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED,
				}, futureUpgrade.Type) {
					// cancel existing watchers
					hw.Cancel()
					upw.Cancel()

					logger.Infof("Received upgrade height from the chain rpc: %v", futureUpgrade.Height)
					return futureUpgrade.Height, nil
				}
			}
		case upgrade := <-uiw.Upgrades:
			if upgrade.Error != nil {
				d.metrics.UiwErrs.Inc()
				logger.Err(err).Error("Error received from UpgradesInfoWatcher")
			}
			d.updateMetrics()

			upgradeHeight := upgrade.Plan.Height

			// cancel existing watchers
			hw.Cancel()
			upw.Cancel()

			logger.Infof("Received upgrade data from upgrade-info.json: %v", upgrade)
			return upgradeHeight, nil
		case err := <-upw.Errors:
			d.metrics.UpwErrs.Inc()
			logger.Err(err).Error("Error received from UpgradesProposalsWatcher")
		}
	}
}

func (d *Daemon) performUpgrade(
	ctx context.Context,
	compose *config.ComposeCli,
	serviceName string,
	upgradeHeight int64,
) (err error) {
	defer func() {
		// ensure we update the status to failed if any error was encountered
		if err != nil {
			d.stateMachine.MustSetStatus(upgradeHeight, urproto.UpgradeStatus_FAILED)
		}
	}()
	ctx = notification.WithUpgradeHeight(ctx, upgradeHeight)

	d.stateMachine.MustSetStatus(upgradeHeight, urproto.UpgradeStatus_EXECUTING)

	// ensure the upgrade is still valid
	upgrade := d.ur.GetUpgradeWithCache(upgradeHeight)
	if upgrade == nil {
		return fmt.Errorf("upgrade with height %d not found", upgradeHeight)
	}

	// sanity check to ensure we are not performing upgrades at wrong times
	if upgradeHeight < d.currHeight {
		return fmt.Errorf("upgrade height %d is less than last observed height %d", upgradeHeight, d.currHeight)
	}

	// ensure the docker image is present on the host (this should be done in a pre-check phase though). Better safe than sorry
	var currImage, newImage string
	currImage, newImage, err = checks.PullDockerImage(ctx, d.dcc, serviceName, upgrade.Tag, upgrade.Height)
	if err != nil {
		return err
	}

	logger := log.FromContext(ctx)

	logger.Infof("Current image: %s. New image: %s found on the host", currImage, newImage)
	d.stateMachine.MustSetStatusAndStep(upgradeHeight, urproto.UpgradeStatus_EXECUTING, urproto.UpgradeStep_COMPOSE_FILE_UPGRADE)

	// take container down or check if it is down already
	isRunning, err := d.dcc.IsServiceRunning(ctx, serviceName, compose.DownTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to check if service is running")
	}

	// This check is prone to race conditions, the image could be up at this point
	// but exits before dcc.Down is called. However, at this point we are certain
	// that upgrade height has been hit, so, it should be safe to Down an exited
	// container.
	if isRunning {
		logger.Info("Executing compose down").Notifyf(ctx, "Shutting down chain to perform upgrade. Current image: %s, new image: %s", currImage, newImage)
		if err = d.dcc.Down(ctx, serviceName, compose.DownTimeout); err != nil {
			return errors.Wrapf(err, "failed to down compose")
		}
	}

	logger.Info("Changing image in compose file")
	if err = d.dcc.UpgradeImage(ctx, serviceName, upgrade.Tag); err != nil {
		return errors.Wrapf(err, "failed to upgrade image")
	}

	logger.Info("Executing compose up")
	if err = d.dcc.Up(ctx, serviceName, compose.UpDeadline); err != nil {
		return errors.Wrapf(err, "failed to up compose")
	}

	msg := fmt.Sprintf("Upgrade completed. New image: %s. Now waiting for post-upgrade check to pass", newImage)
	logger.Info(msg).Notify(ctx)

	return nil
}

func (d *Daemon) updateHeightAndBlockSpeed(newHeight int64) {
	// calculate block speed based on the last few observed blocks
	lastBlockHeight := d.currHeight
	lastHeightTime := d.currHeightTime
	d.currHeight = newHeight
	d.currHeightTime = time.Now()

	// this may happen when polling
	if d.currHeight != lastBlockHeight {
		n := newHeight % int64(cap(d.observedBlockSpeeds))
		d.observedBlockSpeeds[n] = time.Millisecond * time.Duration(d.currHeightTime.Sub(lastHeightTime).Milliseconds()/(d.currHeight-lastBlockHeight))
	}

	sum, cnt := 0.0, 0
	for _, blockSpeed := range d.observedBlockSpeeds {
		if blockSpeed != 0 {
			sum += blockSpeed.Seconds()
			cnt++
		}
	}
	if cnt != 0 {
		d.currBlockSpeed = time.Duration(sum / float64(cnt) * float64(time.Second))
	}
}

func validateComposeSettings(cfg *config.Config) error {
	composeFile, err := docker.LoadComposeFile(cfg.ComposeFile)
	if err != nil {
		return errors.Wrapf(err, "failed to parse docker compose file")
	}

	if slices.Contains(cfg.Checks.PreUpgrade.Enabled, checksproto.PreCheck_SET_HALT_HEIGHT.String()) {
		prefix := cfg.Compose.EnvPrefix + "HALT_HEIGHT"
		service, err := composeFile.GetService(cfg.ComposeService)
		if err != nil {
			return errors.Wrapf(err, "failed to get service %s from compose file", cfg.ComposeService)
		}

		if _, ok := service.Environment[prefix]; !ok {
			return fmt.Errorf("please add '%s=${HALT_HEIGHT}' to services.%s.environment docker compose section", prefix, cfg.ComposeService)
		}

		if !slices.Contains([]string{"no", ""}, service.Restart) {
			return errors.New("SET_HALT_HEIGHT precheck won't work with a restart policy set, please remove it")
		}
	}

	return nil
}
