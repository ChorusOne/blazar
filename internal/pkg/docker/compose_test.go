package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"testing"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	// not checking malformed docker-compose files
	// since that should be taken care by the compose-go package
	tests := []struct {
		name        string
		dir         string
		expectedErr error
	}{
		{
			name:        "InvalidComposeNoImage",
			dir:         "compose-no-image",
			expectedErr: errors.New("service s1 has no image defined"),
		},
		{
			name:        "ValidCompose",
			dir:         "compose-valid",
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempDir := testutils.PrepareTestData(t, "docker", test.dir, test.dir)
			composePath := filepath.Join(tempDir, "docker-compose.yml")

			if _, err := LoadComposeFile(composePath); test.expectedErr != nil {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetServiceVersionsMultiple(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "envfile", "envfile")
	envPath := filepath.Join(tempDir, "env-with-multiple-services")
	versions, err := GetServiceVersions(envPath)
	expected := []ServiceVersionLine{
		{
			Name:    "s1",
			Version: "5.3",
		},
		{
			Name:    "S1",
			Version: "5.4",
		},
	}
	require.NoError(t, err)
	assert.Equal(t, expected, versions)
}

func TestGetImageVersionFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "envfile", "envfile")
	envPath := filepath.Join(tempDir, "env")
	composeTempDir := testutils.PrepareTestData(t, "docker", "compose-valid", "compose-valid")
	composePath := filepath.Join(composeTempDir, "docker-compose.yml")

	_, dcc := newDockerComposeClientWithCtx(t, envPath, composePath, config.UpgradeInEnvFile)

	version, err := dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, "5.3", version)
}

func TestGetImageComposeFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-valid", "compose-valid")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	_, dcc := newDockerComposeClientWithCtx(t, "", composePath, config.UpgradeInComposeFile)

	version, err := dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, "latest", version)
}

func TestUpgradeImageVersionFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-upgrade-test", "compose-upgrade-test")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	envDir := testutils.PrepareTestData(t, "docker", "envfile", "envfile")
	envPath := filepath.Join(envDir, "env")

	ctx, dcc := newDockerComposeClientWithCtx(t, envPath, composePath, config.UpgradeInEnvFile)
	originalVersion := "5.3"
	testutils.MakeImageWith(t, "abcd/efgh", originalVersion, dockerProvider)
	// step 1: assert image name
	version, err := dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, originalVersion, version)

	// step 2: upgrade image
	testVersion := "unset"
	err = dcc.UpgradeImage(ctx, "s1", testVersion)
	require.NoError(t, err)

	// step 3: assert that docker-compose.yaml was updated
	version, err = dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, testVersion, version)
}

func TestUpgradeImageMultipleVersionFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-upgrade-test", "compose-upgrade-test")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	envDir := testutils.PrepareTestData(t, "docker", "envfile", "envfile")
	envPath := filepath.Join(envDir, "env-with-multiple-services")

	ctx, dcc := newDockerComposeClientWithCtx(t, envPath, composePath, config.UpgradeInEnvFile)
	originalVersion := "5.3"
	testutils.MakeImageWith(t, "abcd/efgh", originalVersion, dockerProvider)
	// step 1: assert image name
	version, err := dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, originalVersion, version)

	// and assert unrelated service
	version, err = dcc.GetVersionForService("S1")
	require.NoError(t, err)
	assert.Equal(t, "5.4", version)

	// step 2: upgrade image
	testVersion := "unset"
	err = dcc.UpgradeImage(ctx, "s1", testVersion)
	require.NoError(t, err)

	// step 3: assert that the version file was updated
	version, err = dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, testVersion, version)

	// step 4: assert that the other service was NOT updated
	version, err = dcc.GetVersionForService("S1")
	require.NoError(t, err)
	assert.Equal(t, "5.4", version)
}
func TestUpgradeImageComposeFile(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-upgrade-test", "compose-upgrade-test")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	ctx, dcc := newDockerComposeClientWithCtx(t, "", composePath, config.UpgradeInComposeFile)
	testVersion := "new-version"
	testutils.MakeImageWith(t, "abcd/efgh", testVersion, dockerProvider)

	// step 1: assert image name
	// The version is parsed from the compose file, not from the running image
	version, err := dcc.GetVersionForService("s1")
	require.NoError(t, err)
	assert.Equal(t, "ijkl", version)

	// step 2: upgrade image
	err = dcc.UpgradeImage(ctx, "s1", testVersion)
	require.NoError(t, err)

	// step 3: assert that docker-compose.yaml was updated
	version, err = dcc.GetVersionForService("s1")
	require.NoError(t, err)

	assert.Equal(t, testVersion, version)
}

func TestUpDownCompose(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-valid-template", "compose-valid-template")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	ctx, dcc := newDockerComposeClientWithCtx(t, "", composePath, config.UpgradeInComposeFile)
	testutils.MakeImageWith(t, "image", "version", dockerProvider)

	err := testutils.WriteTmpl(
		filepath.Join(tempDir, "docker-compose.yml.tmpl"),
		struct{ Image string }{Image: "image:version"},
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		testFn func(t *testing.T, composePath string)
	}{
		{
			name: "simple down test with docker compose",
			testFn: func(t *testing.T, composePath string) {
				cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
				err := cmd.Run()
				require.NoError(t, err)

				err = dcc.Down(ctx, "s1", 0)
				require.NoError(t, err)

				isRunning, err := dcc.IsServiceRunning(ctx, "s1", 5*time.Second)
				require.NoError(t, err)
				assert.False(t, isRunning)
			},
		},
		{
			name: "simulate case where docker compose down didn't kill the container",
			testFn: func(t *testing.T, composePath string) {
				// step 1: start test container
				err = exec.Command("docker", "compose", "-f", composePath, "up", "-d").Run()
				require.NoError(t, err)

				// step 2: get docker binary path
				cmd := exec.Command("which", "docker")
				buf := new(bytes.Buffer)
				cmd.Stdout = buf
				require.NoError(t, cmd.Run())
				dockerBinaryPath := strings.ReplaceAll(buf.String(), "\n", "")

				// step 3: fake docker binary to simulate docker compose down timeout (container isn't stopped)
				err = os.WriteFile(filepath.Join(tempDir, "docker"), []byte(fmt.Sprintf(
					`#!/bin/sh
				                     # this is to allow compose client to check if container is running
				                     # we expect format such as:
				                     #   compose -f /tmp/.../docker-compose.yml ps -a -q s1
				                     if [ "$4" = "ps" ]; then %s "$@"; else exit 0; fi
				                    `, dockerBinaryPath,
				)), 0700)
				require.NoError(t, err)

				oldPath := os.Getenv("PATH")
				os.Setenv("PATH", tempDir+":"+oldPath)

				// step 4: with docker compose client try to stop container and expect it still running
				err = dcc.Down(ctx, "s1", 0)
				require.ErrorIs(t, err, ErrContainerRunning)

				// step 5: restore docker binary, and see the docker client take down the container
				os.Setenv("PATH", oldPath)
				err = dcc.Down(ctx, "s1", 0)
				require.NoError(t, err)
			},
		},
		{
			name: "simple up test with docker compose",
			testFn: func(t *testing.T, composePath string) {
				isRunning, err := dcc.IsServiceRunning(ctx, "s1", 5*time.Second)
				require.NoError(t, err)
				assert.False(t, isRunning)

				err = dcc.Up(ctx, "s1", 10*time.Second)
				require.NoError(t, err)

				isRunning, err = dcc.IsServiceRunning(ctx, "s1", 5*time.Second)
				require.NoError(t, err)
				assert.True(t, isRunning)

				// cleanup
				err = exec.Command("docker", "compose", "-f", composePath, "down").Run()
				require.NoError(t, err)
			},
		},
		{
			name: "simulate case where docker compose up didn't start the container",
			testFn: func(t *testing.T, _ string) {
				// step 1: fake docker binary to simulate docker compose up timeout (container isn't running)
				err = os.WriteFile(filepath.Join(tempDir, "docker"), []byte("#!/bin/sh\nexit 0\n"), 0600)
				require.NoError(t, err)

				oldPath := os.Getenv("PATH")
				os.Setenv("PATH", tempDir+":"+oldPath)

				// step 2: with docker compose client try to start container and expect it to fail
				err = dcc.Down(ctx, "s1", 0)
				require.ErrorIs(t, err, ErrContainerNotRunning)

				os.Setenv("PATH", oldPath)
			},
		},
	}

	for _, test := range tests {
		test.testFn(t, composePath)
	}
}

func TestRestartEnvCompose(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "compose-env-echo", "compose-env-echo")
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	ctx, dcc := newDockerComposeClientWithCtx(t, "", composePath, config.UpgradeInComposeFile)
	testutils.MakeEnvEchoImageWith(t, dockerProvider)
	err := dcc.Up(ctx, "s1", 5*time.Second)
	require.NoError(t, err)

	// First run without env var, we get back an empty HALT_HEIGHT
	resp, err := http.Get("http://127.0.0.1:4444")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "HALT_HEIGHT=\n"))

	// Upon restart, we get back a populated HALT_HEIGHT
	composeConfig := config.ComposeCli{
		DownTimeout: 10 * time.Second,
		UpDeadline:  10 * time.Second,
	}
	err = dcc.RestartServiceWithHaltHeight(ctx, &composeConfig, "s1", 1234)
	require.NoError(t, err)
	resp, err = http.Get("http://127.0.0.1:4444")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "HALT_HEIGHT=1234\n"))

	// this Down is not meaningful for the test, only cleans up the test environment
	err = dcc.Down(ctx, "s1", 2*time.Second)
	require.NoError(t, err)
}
