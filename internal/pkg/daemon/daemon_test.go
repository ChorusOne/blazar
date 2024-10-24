package daemon

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"blazar/internal/pkg/cmd"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/cosmos"
	"blazar/internal/pkg/docker"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log/logger"
	"blazar/internal/pkg/log/notification"
	"blazar/internal/pkg/metrics"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/provider"
	"blazar/internal/pkg/provider/database"
	"blazar/internal/pkg/provider/local"
	"blazar/internal/pkg/state_machine"
	"blazar/internal/pkg/testutils"
	"blazar/internal/pkg/upgrades_registry"

	"github.com/cometbft/cometbft/proto/tendermint/p2p"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	simd1RepoTag string
	simd2RepoTag string
)

func TestMain(m *testing.M) {
	dockerProvider, err := testcontainers.NewDockerProvider()
	if err != nil {
		fmt.Println("failed to create docker provider")
		os.Exit(1)
	}

	// build test simapp images (v0.0.1 and v0.0.2)
	simd1RepoTag, simd2RepoTag = testutils.BuildTestImages(context.Background(), dockerProvider)

	os.Exit(m.Run())
}

// Blazar end-to-end integration test for LOCAL and DATABASE providers.
//
// The simd v0.0.1 image is configured to perform upgrade at height 10.
// The target v0.0.2 image has a upgrade handler compiled in to simulate the real case upgrade process.
func TestIntegrationDaemon(t *testing.T) {
	defer func() {
		if t.Failed() {
			yellow, reset := "\033[33m", "\033[0m"
			t.Logf("%sWARNING: Test failed, please check if you any stray containers running in docker ps, and kill them%s", yellow, reset)
		}
	}()

	// we can't register 2 metrics, but this sharing this should probably cause no problems
	metrics, err := metrics.NewMetrics("/path/to/docker-compose.yml", "dummy", "test")
	require.NoError(t, err)

	ports := getFreePorts(t, 4)

	t.Run("LocalProvider", func(t *testing.T) {
		name := fmt.Sprintf("blazar-e2e-test-local-simapp-%d", rand.Uint64())
		t.Parallel()
		tempDir := testutils.PrepareTestData(t, "", "daemon", name)

		provider, err := local.NewProvider(
			path.Join(tempDir, "blazar", "local.db.json"),
			"test",
			1,
		)
		if err != nil {
			t.Fatalf("failed to create local provider: %v", err)
		}

		run(t, metrics, provider, urproto.ProviderType_LOCAL, tempDir, name, ports[0], ports[1])
	})

	t.Run("DatabaseProvider", func(t *testing.T) {
		name := fmt.Sprintf("blazar-e2e-test-db-simapp-%d", rand.Uint64())
		t.Parallel()
		tempDir := testutils.PrepareTestData(t, "", "daemon", name)

		provider, err := prepareMockDatabaseProvider()
		if err != nil {
			t.Fatalf("failed to create database provider: %v", err)
		}

		run(t, metrics, provider, urproto.ProviderType_DATABASE, tempDir, name, ports[2], ports[3])
	})
}

// The integration test for the daemon asserts that all 3 types of upgrades are successfully executed (for a given provider). This is:
// 1. GOVERNANCE
// 2. NON_GOVERNANCE_UNCOORDINATED
// 3. NON_GOVERNANCE_COORDINATED
func run(t *testing.T, metrics *metrics.Metrics, prvdr provider.UpgradeProvider, source urproto.ProviderType, tempDir, serviceName string, grpcPort, cometbftPort int) {
	// ------ PREPARE ENVIRONMENT ------ //
	cfg := generateConfig(t, tempDir, serviceName, grpcPort, cometbftPort)

	// inject test logger
	outBuffer := &threadSafeBuffer{}
	output := zerolog.ConsoleWriter{Out: outBuffer, TimeFormat: time.Kitchen, NoColor: true}
	log := zerolog.New(output).With().Str("module", "blazar").Timestamp().Logger()

	ctx := logger.WithContext(context.Background(), &log)
	fallbackNotifier := notification.NewFallbackNotifier(cfg, nil, &log, "test")
	ctx = notification.WithContextFallback(ctx, fallbackNotifier)

	// compose client with logger
	dcc, err := docker.NewDefaultComposeClient(ctx, nil, cfg.VersionFile, cfg.ComposeFile, cfg.UpgradeMode)
	require.NoError(t, err)

	// ensure we run container with current user (not root!)
	currentUser, err := user.Current()
	require.NoError(t, err)
	err = os.Setenv("MY_UID", currentUser.Uid)
	require.NoError(t, err)

	// initialzie new upgrade registry
	sm := state_machine.NewStateMachine(nil)
	ur := upgrades_registry.NewUpgradeRegistry(
		map[urproto.ProviderType]provider.UpgradeProvider{source: prvdr},
		[]urproto.ProviderType{source},
		sm,
		"test",
	)

	// add test upgrades
	require.NoError(t, ur.AddUpgrade(ctx, &urproto.Upgrade{
		Height:     10,
		Tag:        strings.Split(simd2RepoTag, ":")[1],
		Network:    "test",
		Name:       "test",
		Type:       urproto.UpgradeType_GOVERNANCE,
		Status:     urproto.UpgradeStatus_UNKNOWN,
		Step:       urproto.UpgradeStep_NONE,
		Source:     source,
		Priority:   1,
		ProposalId: nil,
	}, false))

	require.NoError(t, ur.AddUpgrade(ctx, &urproto.Upgrade{
		// this fails with 11/12 as the post upgrade cheecks finish when 11
		// and sometimes 12(when I runs 6 instance s of this test parallelly) has been hit,
		// and the next height detected by the height watcher is 12/13
		//
		// So GetUpcomingUpgradesWithCache would skip it
		//
		// maybe we should not allow users to regiser upgrades for gov upgrade height + 1
		// as they are guaranteed to be skipped
		Height:     13,
		Tag:        strings.Split(simd2RepoTag, ":")[1],
		Network:    "test",
		Name:       "test_uncoordinated",
		Type:       urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED,
		Status:     urproto.UpgradeStatus_UNKNOWN,
		Step:       urproto.UpgradeStep_NONE,
		Source:     source,
		Priority:   1,
		ProposalId: nil,
	}, false))

	require.NoError(t, ur.AddUpgrade(ctx, &urproto.Upgrade{
		// Similar reasoning as above height
		Height:     19,
		Tag:        strings.Split(simd2RepoTag, ":")[1],
		Network:    "test",
		Name:       "test_coordinated",
		Type:       urproto.UpgradeType_NON_GOVERNANCE_COORDINATED,
		Status:     urproto.UpgradeStatus_UNKNOWN,
		Step:       urproto.UpgradeStep_NONE,
		Source:     source,
		Priority:   1,
		ProposalId: nil,
	}, false))

	// refresh the upgrade registry cache
	_, _, _, _, err = ur.Update(ctx, 0, true)
	require.NoError(t, err)

	daemon := Daemon{
		dcc:                 dcc,
		ur:                  ur,
		stateMachine:        sm,
		metrics:             metrics,
		observedBlockSpeeds: make([]time.Duration, 5),
		nodeInfo: &tmservice.GetNodeInfoResponse{
			DefaultNodeInfo: &p2p.DefaultNodeInfo{
				Network: "test",
			},
		},
	}
	require.NoError(t, err)

	// start the siapp node
	_, _, err = cmd.CheckOutputWithDeadline(ctx, 5*time.Second, nil, "docker", "compose", "-f", cfg.ComposeFile, "up", "-d", "--force-recreate")
	require.NoError(t, err)

	// start cosmos client and wait for it to be ready
	cosmosClient, err := cosmos.NewClient(cfg.Clients.Host, cfg.Clients.GrpcPort, cfg.Clients.CometbftPort, cfg.Clients.Timeout)
	require.NoError(t, err)

	for range 20 {
		if err = cosmosClient.StartCometbftClient(); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	require.NoError(t, err)
	daemon.cosmosClient = cosmosClient

	// wait just in case the rpc is not responsive yet
	time.Sleep(2 * time.Second)

	// ------ TEST GOVERNANCE UPGRADE ------ //
	// we expect the chain to upgrade to simd2RepoTag at height 10 //
	latestHeight, err := cosmosClient.GetLatestBlockHeight(ctx)
	require.NoError(t, err)
	require.LessOrEqual(t, latestHeight, int64(8), "the test is faulty, the chain is already at height >= 8")

	height, err := daemon.waitForUpgrade(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(10), height)

	// get simapp container logs
	var stdout bytes.Buffer
	cmd := exec.Command("docker", "compose", "-f", cfg.ComposeFile, "logs")
	cmd.Stdout = &stdout
	err = cmd.Run()
	require.NoError(t, err)

	// chain process must have logged upgrade height being hit
	require.Contains(t, stdout.String(), "UPGRADE \"test1\" NEEDED at height: 10")

	// perform the upgrade
	err = daemon.performUpgrade(ctx, &cfg.Compose, cfg.ComposeService, height)
	require.NoError(t, err)

	// ensure the upgrade was successful
	isImageContainerRunning, err := dcc.IsServiceRunning(ctx, cfg.ComposeService, time.Second*2)
	require.NoError(t, err)
	require.True(t, isImageContainerRunning)

	// blazar should have logged all this
	require.Contains(t, outBuffer.String(), fmt.Sprintf("Monitoring %s for new upgrades", cfg.UpgradeInfoFilePath()))
	require.Contains(t, outBuffer.String(), "Received upgrade data from upgrade-info.json")
	require.Contains(t, outBuffer.String(), "Executing compose up")
	require.Contains(t, outBuffer.String(), fmt.Sprintf("Upgrade completed. New image: %s", simd2RepoTag))

	// lets see if post upgrade checks pass
	err = daemon.postUpgradeChecks(ctx, sm, &cfg.Checks.PostUpgrade, height)
	require.NoError(t, err)

	outBuffer.Reset()

	// ------ TEST NON_GOVERNANCE_UNCOORDINATED UPGRADE ------ //
	// we expect the chain to upgrade to simd2RepoTag at height 13 //
	latestHeight, err = cosmosClient.GetLatestBlockHeight(ctx)
	require.NoError(t, err)
	require.LessOrEqual(t, latestHeight, int64(11), "the test is faulty, the chain is already at height >= 11")

	upgrades, err := ur.GetUpcomingUpgrades(ctx, false, 11, urproto.UpgradeStatus_SCHEDULED, urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStatus_EXECUTING)
	require.NoError(t, err)
	require.Len(t, upgrades, 2)

	height, err = daemon.waitForUpgrade(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(13), height)

	require.Contains(t, outBuffer.String(), fmt.Sprintf("Monitoring %s for new upgrades", cfg.UpgradeInfoFilePath()))
	require.Contains(t, outBuffer.String(), "Received upgrade height from the chain rpc")

	err = daemon.performUpgrade(ctx, &cfg.Compose, cfg.ComposeService, height)
	require.NoError(t, err)

	require.Contains(t, outBuffer.String(), "Executing compose up")
	require.Contains(t, outBuffer.String(), fmt.Sprintf("Upgrade completed. New image: %s", simd2RepoTag))

	// Lets see if post upgrade checks pass
	err = daemon.postUpgradeChecks(ctx, sm, &cfg.Checks.PostUpgrade, height)
	require.NoError(t, err)

	outBuffer.Reset()

	// ------ TEST NON_GOVERNANCE_COORDINATED UPGRADE (with HALT_HEIGHT) ------ //
	// we expect the chain to upgrade to simd2RepoTag at height 19 //
	latestHeight, err = cosmosClient.GetLatestBlockHeight(ctx)
	require.NoError(t, err)
	require.LessOrEqual(t, latestHeight, int64(14), "the test is faulty, the chain is already at height > 14")

	upgrades, err = ur.GetUpcomingUpgrades(ctx, false, 14, urproto.UpgradeStatus_SCHEDULED, urproto.UpgradeStatus_ACTIVE, urproto.UpgradeStatus_EXECUTING)
	require.NoError(t, err)
	require.Len(t, upgrades, 1)

	height, err = daemon.waitForUpgrade(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(19), height)

	// get container logs
	stdout.Reset()
	cmd = exec.Command("docker", "compose", "-f", cfg.ComposeFile, "logs")
	cmd.Stdout = &stdout
	err = cmd.Run()
	require.NoError(t, err)

	require.Contains(t, stdout.String(), "halt per configuration height 19")

	require.Contains(t, outBuffer.String(), fmt.Sprintf("Monitoring %s for new upgrades", cfg.UpgradeInfoFilePath()))
	require.Contains(t, outBuffer.String(), "Received upgrade height from the chain rpc")

	// older cosmos-sdk versions exit the node while the newer ones don't
	// in this case simapp will halt at height 19 but won't exit
	// we want to be sure the pre-check worked
	require.Contains(t, outBuffer.String(), "HALT_HEIGHT likely worked but didn't shut down the node")

	err = daemon.performUpgrade(ctx, &cfg.Compose, cfg.ComposeService, height)
	require.NoError(t, err)

	// lets see if post upgrade checks pass
	err = daemon.postUpgradeChecks(ctx, sm, &cfg.Checks.PostUpgrade, height)
	require.NoError(t, err)

	// cleanup
	err = dcc.Down(ctx, cfg.ComposeService, cfg.Compose.DownTimeout)
	require.NoError(t, err)
}

type threadSafeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *threadSafeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func generateConfig(t *testing.T, tempDir, serviceName string, grpcPort, cometbftPort int) *config.Config {
	err := testutils.WriteTmpl(filepath.Join(tempDir, "docker-compose.yml.tmpl"), struct {
		ServiceName  string
		Image        string
		GrpcPort     int
		CometbftPort int
	}{
		ServiceName:  serviceName,
		Image:        simd1RepoTag,
		GrpcPort:     grpcPort,
		CometbftPort: cometbftPort,
	})
	require.NoError(t, err)

	return &config.Config{
		ChainHome:      filepath.Join(tempDir, "chain-home"),
		ComposeFile:    filepath.Join(tempDir, "docker-compose.yml"),
		ComposeService: serviceName,
		VersionFile:    "",
		UpgradeMode:    config.UpgradeInComposeFile,
		Host:           "dummy",
		Watchers: config.Watchers{
			UIInterval: time.Millisecond * 5,
			HInterval:  time.Second * 0,
			UPInterval: time.Minute * 5,
		},
		Clients: config.Clients{
			Host:         "localhost",
			GrpcPort:     uint16(grpcPort),
			CometbftPort: uint16(cometbftPort),
			Timeout:      10 * time.Second,
		},
		Checks: config.Checks{
			PreUpgrade: config.PreUpgrade{
				Enabled: []string{"SET_HALT_HEIGHT"},
				// as soon as possible
				Blocks: 100,
				SetHaltHeight: &config.SetHaltHeight{
					DelayBlocks: 0,
				},
			},
			PostUpgrade: config.PostUpgrade{
				// cannot enable FIRST_BLOCK_VOTED here as the test validator has
				// prevotes_bit_array	"BA{1:_} 0/1 = 0.00"
				Enabled: []string{"GRPC_RESPONSIVE", "CHAIN_HEIGHT_INCREASED"},
				GrpcResponsive: &config.GrpcResponsive{
					PollInterval: 300 * time.Millisecond,
					Timeout:      20 * time.Second,
				},
				ChainHeightIncreased: &config.ChainHeightIncreased{
					PollInterval:  300 * time.Millisecond,
					Timeout:       20 * time.Second,
					NotifInterval: 10 * time.Minute,
				},
				FirstBlockVoted: &config.FirstBlockVoted{
					PollInterval:  300 * time.Millisecond,
					Timeout:       20 * time.Second,
					NotifInterval: 10 * time.Minute,
				},
			},
		},
		Compose: config.ComposeCli{
			DownTimeout: time.Second * 30,
			UpDeadline:  time.Second * 30,
			EnvPrefix:   "SIMD_",
		},
	}
}

func getFreePorts(t *testing.T, n int) []int {
	var ports []int
	var listeners []net.Listener

	// create n listeners
	for range n {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)

		// get the assigned port from the listener
		addr := listener.Addr().(*net.TCPAddr)
		ports = append(ports, addr.Port)

		// add listener to slice for later closing
		listeners = append(listeners, listener)
	}

	// close all listeners after getting ports
	for _, listener := range listeners {
		err := listener.Close()
		require.NoError(t, err)
	}

	return ports
}

func prepareMockDatabaseProvider() (*database.Provider, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect database")
	}
	err = db.AutoMigrate(&urproto.Upgrade{})
	if err != nil {
		return nil, errors.Wrapf(err, "database migration failed for upgrades table")
	}

	err = db.AutoMigrate(&vrproto.Version{})
	if err != nil {
		return nil, errors.Wrapf(err, "database migration failed for versions table")
	}
	return database.NewDatabaseProviderWithDB(db, "test", 1), nil
}
