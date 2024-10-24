package docker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/testutils"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

var (
	dockerProvider *testcontainers.DockerProvider
)

func TestMain(m *testing.M) {
	var err error
	dockerProvider, err = testcontainers.NewDockerProvider()
	if err != nil {
		fmt.Println("failed to create docker provider")
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}

func newDockerClientWithCtx(t *testing.T) (context.Context, *Client) {
	ctx := testutils.NewContext()
	dc, err := NewClient(ctx, nil)
	require.NoError(t, err)

	return ctx, dc
}

func newDockerComposeClientWithCtx(t *testing.T, versionFile, composeFile string, upgradeMode config.UpgradeMode) (context.Context, *ComposeClient) {
	ctx := testutils.NewContext()
	dcc, err := NewDefaultComposeClient(ctx, nil, versionFile, composeFile, upgradeMode)
	require.NoError(t, err)

	return ctx, dcc
}
