package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blazar/internal/pkg/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// To ensure that the config file is read correctly
// and config.sample.toml is properly formatted
func TestReadConfigToml(t *testing.T) {
	cfg, err := ReadConfig("../../../blazar.sample.toml")
	require.NoError(t, err)
	assert.Equal(t, &Config{
		ComposeFile:    "<path>",
		ComposeService: "<service>",
		UpgradeMode:    UpgradeInComposeFile,
		ChainHome:      "<path>",
		LogLevel:       1,
		Host:           "0.0.0.0",
		HTTPPort:       1234,
		GrpcPort:       5678,
		Watchers: Watchers{
			UIInterval: 300 * time.Millisecond,
			HInterval:  0,
			HTimeout:   20 * time.Second,
			UPInterval: 10 * time.Minute,
		},
		Clients: Clients{
			Host:         "<host>",
			GrpcPort:     9090,
			CometbftPort: 25567,
			Timeout:      10 * time.Second,
		},
		Compose: ComposeCli{
			DownTimeout: time.Minute,
			UpDeadline:  time.Minute,
			EnvPrefix:   "",
		},
		Checks: Checks{
			PreUpgrade: PreUpgrade{
				Enabled: []string{"READ_IMAGE_VERSION","PULL_DOCKER_IMAGE", "SET_HALT_HEIGHT"},
				Blocks:  200,
				SetHaltHeight: &SetHaltHeight{
					DelayBlocks: 0,
				},
			},
			PostUpgrade: PostUpgrade{
				Enabled: []string{"GRPC_RESPONSIVE", "CHAIN_HEIGHT_INCREASED", "FIRST_BLOCK_VOTED"},
				GrpcResponsive: &GrpcResponsive{
					PollInterval: 1 * time.Second,
					Timeout:      3 * time.Minute,
				},
				ChainHeightIncreased: &ChainHeightIncreased{
					PollInterval:  1 * time.Second,
					NotifInterval: 1 * time.Minute,
					Timeout:       5 * time.Minute,
				},
				FirstBlockVoted: &FirstBlockVoted{
					PollInterval:  1 * time.Second,
					NotifInterval: 1 * time.Minute,
					Timeout:       5 * time.Minute,
				},
			},
		},
		Slack: &Slack{
			WebhookNotifier: &SlackWebhookNotifier{
				WebhookURL: "<url or absolute path of file containing url>",
			},
		},
		CredentialHelper: &DockerCredentialHelper{
			Command: "<path>",
			Timeout: 10 * time.Second,
		},
		UpgradeRegistry: UpgradeRegistry{
			SelectedProviders: []string{"chain", "database", "local"},
			Network:           "<network>",
			Provider: Provider{
				Database: &DatabaseProvider{
					DefaultPriority: int32(3),
					Host:            "<db-host>",
					Port:            5432,
					DB:              "<db-name>",
					User:            "<db-user>",
					Password:        "<db-password>",
					PasswordFile:    "<path-to-file-containing-password>",
					SslMode:         Disable,
					AutoMigrate:     false,
				},
				Local: &LocalProvider{
					ConfigPath:      "./local-provider.db.json",
					DefaultPriority: int32(2),
				},
				Chain: &ChainProvider{
					DefaultPriority: int32(1),
				},
			},

			VersionResolvers: &VersionResolvers{
				Providers: []string{"local", "database"},
			},
			StateMachine: StateMachine{
				Provider: "local",
			},
		},
	}, cfg)

	assert.NoError(t, cfg.ValidateGrpcClient())
	assert.NoError(t, cfg.ValidatePreUpgradeChecks())
	assert.NoError(t, cfg.ValidatePostUpgradeChecks())
}

// To ensure that the config file is read correctly
// and config.sample.toml is properly formatted
func TestValidateComposeFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "config", "validate-compose-file", "")

	tests := []struct {
		name        string
		composeFile string
		expectedErr error
	}{
		{
			name:        "ValidFile",
			composeFile: filepath.Join(tempDir, "valid-compose.yaml"),
			expectedErr: nil,
		},
		{
			name:        "FileNotFound",
			composeFile: filepath.Join(tempDir, "nonexistent-compose.yaml"),
			expectedErr: errors.New("error validating compose-file: file not found: stat " + filepath.Join(tempDir, "nonexistent-compose.yaml") + ": no such file or directory"),
		},
		{
			name:        "InvalidPathIsDir",
			composeFile: filepath.Join(tempDir, "some-directory"),
			expectedErr: errors.New("error validating compose-file: the path \"" + filepath.Join(tempDir, "some-directory") + "\" already exists but is not a file"),
		},
		{
			name:        "InvalidPathIsRelative",
			composeFile: "valid-compose.yaml",
			expectedErr: errors.New("error validating compose-file: \"valid-compose.yaml\" must be an absolute path"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{ComposeFile: test.composeFile}

			if err := cfg.ValidateComposeFile(); test.expectedErr != nil {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

	permTests := []struct {
		name        string
		path        string
		mode        fs.FileMode
		expectedErr string
	}{
		{
			name:        "ComposeFileNoWrite",
			path:        filepath.Join(tempDir, "valid-compose.yaml"),
			mode:        0555,
			expectedErr: "error validating compose-file: requested permission bits 110 not found on \"" + filepath.Join(tempDir, "valid-compose.yaml") + "\": permission denied",
		},
		{
			name:        "ComposeFileNoRead",
			path:        filepath.Join(tempDir, "valid-compose.yaml"),
			mode:        0333,
			expectedErr: "error validating compose-file: requested permission bits 110 not found on \"" + filepath.Join(tempDir, "valid-compose.yaml") + "\": permission denied",
		},
		{
			name:        "BaseDirNoWrite",
			path:        tempDir,
			mode:        0555,
			expectedErr: "error validating compose-file: requested permission bits 110 not found on \"" + tempDir + "\": permission denied",
		},
		{
			name:        "BaseDirNoRead",
			path:        tempDir,
			mode:        0333,
			expectedErr: "error validating compose-file: requested permission bits 110 not found on \"" + tempDir + "\": permission denied",
		},
	}
	for _, test := range permTests {
		t.Run(test.name, func(t *testing.T) {
			stat, err := os.Stat(test.path)
			require.NoError(t, err)
			err = os.Chmod(test.path, test.mode)
			require.NoError(t, err)
			cfg := &Config{ComposeFile: filepath.Join(tempDir, "valid-compose.yaml")}
			err = cfg.ValidateComposeFile()
			assert.Equal(t, test.expectedErr, err.Error())
			err = os.Chmod(test.path, stat.Mode())
			require.NoError(t, err)
		})
	}
}

func TestValidateChainHome(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "config", "validate-chain-home", "")

	tests := []struct {
		name        string
		chainHome   string
		expectedErr error
	}{
		{
			name:        "ValidDir",
			chainHome:   filepath.Join(tempDir, "chain-home-dir"),
			expectedErr: nil,
		},
		{
			name:        "DirNotFound",
			chainHome:   filepath.Join(tempDir, "nonexistent-dir"),
			expectedErr: errors.New("error validating chain-home: directory not found: stat " + filepath.Join(tempDir, "nonexistent-dir") + ": no such file or directory"),
		},
		{
			name:        "InvalidPathIsFile",
			chainHome:   filepath.Join(tempDir, "chain-home-file"),
			expectedErr: errors.New("error validating chain-home: the path \"" + filepath.Join(tempDir, "chain-home-file") + "\" already exists but is not a directory"),
		},
		{
			name:        "InvalidPathIsRelative",
			chainHome:   "chain-home-dir",
			expectedErr: errors.New("error validating chain-home: \"chain-home-dir\" must be an absolute path"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{ChainHome: test.chainHome}

			if err := cfg.ValidateChainHome(); test.expectedErr != nil {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

	permTests := []struct {
		name        string
		path        string
		chainHome   string
		mode        fs.FileMode
		expectedErr string
	}{
		{
			name:        "ChainHomeNoWrite",
			path:        filepath.Join(tempDir, "chain-home-dir"),
			chainHome:   filepath.Join(tempDir, "chain-home-dir"),
			mode:        0555,
			expectedErr: "error validating chain-home: requested permission bits 110 not found on \"" + filepath.Join(tempDir, "chain-home-dir") + "\": permission denied",
		},
		{
			name:        "ChainHomeNoRead",
			path:        filepath.Join(tempDir, "chain-home-dir"),
			chainHome:   filepath.Join(tempDir, "chain-home-dir"),
			mode:        0333,
			expectedErr: "error validating chain-home: requested permission bits 110 not found on \"" + filepath.Join(tempDir, "chain-home-dir") + "\": permission denied",
		},
		{
			name:        "DataDirNoRead",
			path:        filepath.Join(tempDir, "chain-home-dir/data"),
			chainHome:   filepath.Join(tempDir, "chain-home-dir"),
			mode:        0333,
			expectedErr: "error validating chain-home/data: requested permission bits 100 not found on \"" + filepath.Join(tempDir, "chain-home-dir/data") + "\": permission denied",
		},
		{
			name:        "DataDirNoWrite",
			path:        filepath.Join(tempDir, "chain-home-dir/data"),
			chainHome:   filepath.Join(tempDir, "chain-home-dir"),
			mode:        0555,
			expectedErr: "",
		},
		{
			name:        "UpgradeInfoFileNoRead",
			path:        filepath.Join(tempDir, "chain-home-with-upgrade-json/data/upgrade-info.json"),
			chainHome:   filepath.Join(tempDir, "chain-home-with-upgrade-json"),
			mode:        0333,
			expectedErr: "error validating chain-home/data/upgrade-info.json: requested permission bits 100 not found on \"" + filepath.Join(tempDir, "chain-home-with-upgrade-json/data/upgrade-info.json") + "\": permission denied",
		},
		{
			name:        "UpgradeInfoFileNoWrite",
			path:        filepath.Join(tempDir, "chain-home-with-upgrade-json/data/upgrade-info.json"),
			chainHome:   filepath.Join(tempDir, "chain-home-with-upgrade-json"),
			mode:        0555,
			expectedErr: "",
		},
	}
	for _, test := range permTests {
		t.Run(test.name, func(t *testing.T) {
			stat, err := os.Stat(test.path)
			require.NoError(t, err)
			err = os.Chmod(test.path, test.mode)
			require.NoError(t, err)
			cfg := &Config{ChainHome: test.chainHome}
			err = cfg.ValidateChainHome()
			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				assert.Equal(t, test.expectedErr, err.Error())
			}
			err = os.Chmod(test.path, stat.Mode())
			require.NoError(t, err)
		})
	}
}

func TestValidateDockerCredentialHelper(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "config", "validate-docker-credential-helper", "")

	tests := []struct {
		name                   string
		dockerCredentialHelper string
		expectedErr            error
	}{
		{
			name:                   "ValidFile",
			dockerCredentialHelper: filepath.Join(tempDir, "valid-docker-credential-helper"),
			expectedErr:            nil,
		},
		{
			name:                   "FileNotFound",
			dockerCredentialHelper: filepath.Join(tempDir, "nonexistent-docker-credential-helper"),
			expectedErr:            errors.New("error validating docker-credential-helper.command: file not found: stat " + filepath.Join(tempDir, "nonexistent-docker-credential-helper") + ": no such file or directory"),
		},
		{
			name:                   "InvalidPathIsDir",
			dockerCredentialHelper: filepath.Join(tempDir, "some-directory"),
			expectedErr:            errors.New("error validating docker-credential-helper.command: the path \"" + filepath.Join(tempDir, "some-directory") + "\" already exists but is not a file"),
		},
		{
			name:                   "InvalidPathIsRelative",
			dockerCredentialHelper: "valid-docker-credential-helper",
			expectedErr:            errors.New("error validating docker-credential-helper.command: \"valid-docker-credential-helper\" must be an absolute path"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				CredentialHelper: &DockerCredentialHelper{
					Command: test.dockerCredentialHelper,
					Timeout: time.Second,
				},
			}

			if err := cfg.ValidateCredentialHelper(); test.expectedErr != nil {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

	permTests := []struct {
		name        string
		path        string
		mode        fs.FileMode
		expectedErr string
	}{
		{
			name:        "FileNoExec",
			mode:        0444,
			expectedErr: "error validating docker-credential-helper.command: requested permission bits 101 not found on \"" + filepath.Join(tempDir, "valid-docker-credential-helper") + "\": permission denied",
		},
		{
			name:        "FileNoRead",
			mode:        0333,
			expectedErr: "error validating docker-credential-helper.command: requested permission bits 101 not found on \"" + filepath.Join(tempDir, "valid-docker-credential-helper") + "\": permission denied",
		},
	}
	testPath := filepath.Join(tempDir, "valid-docker-credential-helper")
	for _, test := range permTests {
		t.Run(test.name, func(t *testing.T) {
			stat, err := os.Stat(testPath)
			require.NoError(t, err)

			err = os.Chmod(testPath, test.mode)
			require.NoError(t, err)

			cfg := &Config{
				CredentialHelper: &DockerCredentialHelper{
					Command: testPath,
					Timeout: time.Second,
				},
			}
			err = cfg.ValidateCredentialHelper()
			assert.Equal(t, test.expectedErr, err.Error())

			err = os.Chmod(testPath, stat.Mode())
			require.NoError(t, err)
		})
	}
}

func TestLoadWebhookUrl(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "config", "load-webhook-url", "")

	doesntExist := filepath.Join(tempDir, "doesnt-exist")

	tests := []struct {
		name        string
		val         string
		expectedVal string
		expectedErr error
	}{
		{
			name:        "Valid",
			val:         "1234",
			expectedVal: "1234",
			expectedErr: nil,
		},
		{
			name:        "ValidFile",
			val:         filepath.Join(tempDir, "webhook"),
			expectedVal: "abcd",
			expectedErr: nil,
		},
		{
			name:        "NonExistentFile",
			val:         doesntExist,
			expectedVal: doesntExist,
			expectedErr: fmt.Errorf("failed reading %s file: open %s: no such file or directory", doesntExist, doesntExist),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Slack: &Slack{
					WebhookNotifier: &SlackWebhookNotifier{
						WebhookURL: test.val,
					},
				},
			}

			if err := cfg.LoadWebhookURL(); test.expectedErr != nil {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.expectedVal, cfg.Slack.WebhookNotifier.WebhookURL)
		})
	}
}
