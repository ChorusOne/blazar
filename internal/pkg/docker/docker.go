package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Client struct {
	client           *client.Client
	credentialHelper CredentialHelper
}

func NewClientWithConfig(ctx context.Context, cfg *config.DockerCredentialHelper) (*Client, error) {
	if cfg != nil {
		return NewClientWithCredsHelper(ctx, cfg.Command, cfg.Timeout)
	}
	return NewClient(ctx, nil)
}

func NewClientWithCredsHelper(ctx context.Context, cmd string, timeout time.Duration) (*Client, error) {
	return NewClient(ctx, NewCredentialHelper(cmd, timeout))
}

func NewClient(ctx context.Context, ch CredentialHelper) (*Client, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	dc.NegotiateAPIVersion(ctx)

	return &Client{
		client:           dc,
		credentialHelper: ch,
	}, nil
}

func (dc *Client) IsImagePresent(ctx context.Context, name string) (bool, error) {
	images, err := dc.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, err
	}

	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			if repoTag == name {
				return true, nil
			}
		}
	}

	return false, nil
}

func (dc *Client) PullImage(ctx context.Context, name string, platform string) error {
	imagePullOptions := types.ImagePullOptions{
		Platform: platform,
	}

	if dc.credentialHelper != nil {
		creds, err := dc.credentialHelper.GetRegistryAuth(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get authorization token using credential helper")
		}
		imagePullOptions.RegistryAuth = creds
	}

	reader, err := dc.client.ImagePull(ctx, name, imagePullOptions)
	if err != nil {
		return err
	}
	defer reader.Close()

	logger := log.FromContext(ctx)

	// read and log pull response
	buf := make([]byte, 8*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := reader.Read(buf)
			if err == io.EOF {
				return nil
			} else if err != nil {
				return errors.Wrapf(err, "failed to read from image pull response")
			}
			logger.Infof("%s", string(buf[:n]))
		}
	}
}

func (dc *Client) PullImageWithRetry(ctx context.Context, name string,
	platform string, maxRetries int, backoffDuration time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
			}
		}

		err := dc.PullImage(ctx, name, platform)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxRetries {
			backoffDuration *= 2
		}
	}

	return lastErr
}

func (dc *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	if containerID == "" {
		return false, errors.New("containerId is empty")
	}

	containers, err := dc.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		if container.ID == containerID {
			return true, nil
		}
	}

	return false, nil
}

func (dc *Client) ContainerList(ctx context.Context, all bool) ([]types.Container, error) {
	return dc.client.ContainerList(ctx, types.ContainerListOptions{All: all})
}

func ParseImageName(imageName string) (string, string, error) {
	parts := strings.Split(imageName, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid image name: %s", imageName)
	}

	return parts[0], parts[1], nil
}
