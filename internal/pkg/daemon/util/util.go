package util

import (
	"fmt"
	"os"

	"blazar/internal/pkg/docker"
	"blazar/internal/pkg/errors"
)

func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return hostname
}

// Get the current image from compose file and the image corresponding to the
// upgradeTag
func GetCurrImageUpgradeImage(dcc *docker.ComposeClient, serviceName, upgradeTag string) (string, string, error) {
	currImage, currVersion, err := dcc.GetImageAndVersionFromCompose(serviceName)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get image for service %s", serviceName)
	}
	currComposeImage := fmt.Sprintf("%s:%s", currImage, currVersion)
	newImage := fmt.Sprintf("%s:%s", currImage, upgradeTag)

	return currComposeImage, newImage, nil
}
