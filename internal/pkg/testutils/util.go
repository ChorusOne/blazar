package testutils

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blazar/internal/pkg/log/logger"

	"github.com/otiai10/copy"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const TestdataDirPath = "../../../testdata"

func WriteTmpl(file string, data interface{}) error {
	t, err := template.New(filepath.Base(file)).ParseFiles(file)
	if err != nil {
		return err
	}

	newF := strings.Replace(file, ".tmpl", "", 1)
	f, err := os.OpenFile(newF, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	err = t.Execute(f, data)
	if err != nil {
		return err
	}

	return f.Close()
}

func BuildTestImages(ctx context.Context, dockerProvider *testcontainers.DockerProvider) (string, string) {
	simd1RepoTag, err := dockerProvider.BuildImage(ctx, &testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: "../../../testdata/daemon/images/v0.0.1/",
		},
	})

	if err != nil {
		fmt.Println("failed to build simd v0.0.1 container")
		os.Exit(1)
	}

	simd2RepoTag, err := dockerProvider.BuildImage(ctx, &testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Repo:    strings.Split(simd1RepoTag, ":")[0],
			Context: "../../../testdata/daemon/images/v0.0.2/",
		},
	})

	if err != nil {
		fmt.Println("failed to build simd v0.0.2 container")
		os.Exit(1)
	}

	return simd1RepoTag, simd2RepoTag
}

func PrepareTestData(t *testing.T, prefix, path, dst string) string {
	tempDir := t.TempDir()
	pth := filepath.Join(prefix, path)
	target := filepath.Join(tempDir, dst)

	MustCopy(t, pth, target)

	return target
}

func MustCopy(t *testing.T, src, dst string) {
	err := copy.Copy(filepath.Join(TestdataDirPath, src), dst)
	assert.NoError(t, err)
}

func NewChainHomeDir(t *testing.T) (string, string) {
	chainHome := t.TempDir()
	blazarDir, err := filepath.Abs(filepath.Join(chainHome, "blazar"))
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(chainHome, "blazar"), 0755)
	require.NoError(t, err)

	chainHomeAbs, err := filepath.Abs(chainHome)
	assert.NoError(t, err)

	return chainHomeAbs, blazarDir
}

func MakeImageWith(t *testing.T, imageName, tag string, dockerProvider *testcontainers.DockerProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := dockerProvider.BuildImage(ctx, &testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: filepath.Join(TestdataDirPath, "docker", "sleep-dockerfile"),
			Repo:    imageName,
			Tag:     tag,
		},
	})
	require.NoError(t, err)
}

func MakeEnvEchoImageWith(t *testing.T, dockerProvider *testcontainers.DockerProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := dockerProvider.BuildImage(ctx, &testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: filepath.Join(TestdataDirPath, "docker", "echo-dockerfile"),
			Repo:    "testrepo/env-echo",
			Tag:     "latest",
		},
	})
	require.NoError(t, err)
}

func NewContext() context.Context {
	lg := zerolog.New(io.Discard).With().Logger()
	return logger.WithContext(context.Background(), &lg)
}
