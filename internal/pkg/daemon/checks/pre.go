package checks

import (
	"context"
	"fmt"

	"blazar/internal/pkg/daemon/util"
	"blazar/internal/pkg/docker"
	"blazar/internal/pkg/errors"
)

// return current image, upgrade image, error
func PullDockerImage(ctx context.Context, dcc *docker.ComposeClient, serviceName, upgradeTag string, upgradeHeight int64) (string, string, error) {
	if upgradeTag == "" {
		return "", "", fmt.Errorf("failed to check docker image, upgrade tag is empty, for upgrade height: %d", upgradeHeight)
	}

	currImage, newImage, err := util.GetCurrImageUpgradeImage(dcc, serviceName, upgradeTag)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get new upgrade image for height: %d, tag: %s", upgradeHeight, upgradeTag)
	}

	isImagePresent, err := dcc.DockerClient().IsImagePresent(ctx, newImage)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to check if new image %s is present", newImage)
	}

	if !isImagePresent {
		// let's try to pull once
		platform, err := dcc.GetPlatform(serviceName)
		if err != nil {
			return "", "", errors.Wrapf(err, "new image %s is not present on host and failed to get platform from compose file", newImage)
		}

		if err := dcc.DockerClient().PullImage(ctx, newImage, platform); err != nil {
			return "", "", errors.Wrapf(err, "new image %s is not present on host and pull failed", newImage)
		}
	}

	return currImage, newImage, nil
}
