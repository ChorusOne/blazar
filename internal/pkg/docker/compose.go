package docker

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"blazar/internal/pkg/cmd"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"

	compose "github.com/compose-spec/compose-go/cli"
	composeTypes "github.com/compose-spec/compose-go/types"
)

var (
	ErrContainerRunning    = errors.New("container running")
	ErrContainerNotRunning = errors.New("container not running")
)

type ComposeClient struct {
	client      *Client
	versionFile string
	composeFile string
	upgradeMode config.UpgradeMode
}

func NewDefaultComposeClient(ctx context.Context, ch CredentialHelper, versionFile, composeFile string, upgradeMode config.UpgradeMode) (*ComposeClient, error) {
	dc, err := NewClient(ctx, ch)
	if err != nil {
		return nil, err
	}

	return NewComposeClient(dc, versionFile, composeFile, upgradeMode)
}

func NewComposeClient(dockerClient *Client, versionFile, composeFile string, upgradeMode config.UpgradeMode) (*ComposeClient, error) {
	if !slices.Contains(config.ValidUpgradeModes, upgradeMode) {
		return nil, fmt.Errorf("invalid upgradeMode '%s', pick one of %+v", upgradeMode, config.ValidUpgradeModes)
	}

	return &ComposeClient{
		client:      dockerClient,
		versionFile: versionFile,
		composeFile: composeFile,
		upgradeMode: upgradeMode,
	}, nil
}

func (dcc *ComposeClient) DockerClient() *Client {
	return dcc.client
}

func (dcc *ComposeClient) GetImageAndVersionFromCompose(serviceName string) (string, string, error) {
	project, err := LoadComposeFile(dcc.composeFile)
	if err != nil {
		return "", "", errors.Wrapf(err, "compose file loading failed")
	}

	service, err := getServiceFromProject(serviceName, project)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get service from project")
	}
	image, version, err := ParseImageName(service.Image)
	if err != nil {
		return "", "", fmt.Errorf("invalid image - missing tag: %s", service.Image)
	}

	return image, version, nil
}

func (dcc *ComposeClient) getVersionFromEnv(serviceName, composeFile string) (string, error) {
	version, err := LoadServiceVersionFile(composeFile, serviceName)
	if err != nil {
		return "", errors.Wrapf(err, "version file loading failed")
	}

	return version, nil
}

func (dcc *ComposeClient) GetVersionForService(serviceName string) (string, error) {
	if dcc.upgradeMode == config.UpgradeInEnvFile {
		version, err := dcc.getVersionFromEnv(serviceName, dcc.versionFile)
		if err != nil {
			return "", err
		}
		return version, nil
	} else if dcc.upgradeMode == config.UpgradeInComposeFile {
		_, composeVersion, err := dcc.GetImageAndVersionFromCompose(serviceName)
		if err != nil {
			return "", err
		}
		return composeVersion, err
	}
	return "", fmt.Errorf("invalid upgrade mode %+v", dcc.upgradeMode)
}

func (dcc *ComposeClient) GetPlatform(serviceName string) (string, error) {
	project, err := LoadComposeFile(dcc.composeFile)
	if err != nil {
		return "", errors.Wrapf(err, "compose file loading failed")
	}

	service, err := getServiceFromProject(serviceName, project)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get service from project")
	}

	return service.Platform, nil
}

func (dcc *ComposeClient) UpgradeImage(ctx context.Context, serviceName, newVersion string) error {
	switch dcc.upgradeMode {
	case config.UpgradeInEnvFile:
		return dcc.upgradeImageInEnvFile(ctx, serviceName, newVersion)
	case config.UpgradeInComposeFile:
		return dcc.upgradeImageInComposeFile(ctx, serviceName, newVersion)
	}
	return fmt.Errorf("invalid upgrade mode %+v", dcc.upgradeMode)
}

// Updates the `version` field in the version file, which is used in docker-compose
// to determine the image version image to run
func (dcc *ComposeClient) upgradeImageInEnvFile(ctx context.Context, serviceName, newVersion string) error {
	oldVersion, err := LoadServiceVersionFile(dcc.versionFile, serviceName)
	if err != nil {
		return errors.Wrapf(err, "loading the service version file failed")
	}
	log.FromContext(ctx).Infof("Updating version on %s from %s to %s", dcc.versionFile, oldVersion, newVersion)

	return dcc.updateVersionFile(serviceName, newVersion)
}

// Updates version in the `image` field in docker compose
//
// This method does not write the parsed config into yaml, but instead uses simple string replacement
// to preserve user formatting
func (dcc *ComposeClient) upgradeImageInComposeFile(ctx context.Context, serviceName, newVersion string) error {
	project, err := LoadComposeFile(dcc.composeFile)
	if err != nil {
		return errors.Wrapf(err, "compose file loading failed")
	}

	currService, err := getServiceFromProject(serviceName, project)
	if err != nil {
		return err
	}

	image, _, err := ParseImageName(currService.Image)
	if err != nil {
		return err
	}

	newImage := fmt.Sprintf("%s:%s", image, newVersion)
	if currService.Image == newImage {
		log.FromContext(ctx).Warnf("image %s already registered in compose file", image)
		return nil
	}

	isImagePresent, err := dcc.client.IsImagePresent(ctx, newImage)
	if err != nil {
		return errors.Wrapf(err, "check for docker image present failed")
	}

	if !isImagePresent {
		return fmt.Errorf("image %s not present on the system", newImage)
	}

	updatedContent, err := readAndReplace(dcc.composeFile, currService.Image, newImage)
	if err != nil {
		return err
	}

	return updateComposeFile(dcc.composeFile, updatedContent)
}

func (dcc *ComposeClient) Down(ctx context.Context, serviceName string, timeout time.Duration) error {
	// 5 seconds buffer to handle the case when docker timeout (-t)
	// takes slightly longer than defined timeout (thus delay context cancellation)
	deadline := timeout + 5*time.Second
	timeoutSeconds := int(math.Round(timeout.Seconds()))

	err := cmd.ExecuteWithDeadlineAndLog(ctx, deadline, []string{}, "docker", "compose", "-f", dcc.composeFile, "down", "--remove-orphans", "-t", strconv.Itoa(timeoutSeconds))
	if err != nil {
		return errors.Wrapf(err, "docker compose down failed")
	}

	// verify from docker api that the container is down
	isImageContainerRunning, err := dcc.IsServiceRunning(ctx, serviceName, timeout)
	if err != nil {
		return errors.Wrapf(err, "check for container running failed")
	}
	if isImageContainerRunning {
		return errors.Wrapf(ErrContainerRunning, "compose down didn't stop the container")
	}

	return nil
}

func (dcc *ComposeClient) Up(ctx context.Context, serviceName string, timeout time.Duration, ephemeralEnvVars ...string) error {
	isImageContainerRunning, err := dcc.IsServiceRunning(ctx, serviceName, timeout)
	if err != nil {
		return errors.Wrapf(err, "check for container running failed")
	}
	if isImageContainerRunning {
		return errors.Wrapf(ErrContainerRunning, "expected the container to be down before calling docker compose up")
	}

	// docker-compose up supports -t flag but it is only used
	// when containers are already running and need to be shut down
	// before starting them again, we are ensuring that containers are
	// not running at this point, so we don't need to use -t flag
	err = cmd.ExecuteWithDeadlineAndLog(ctx, timeout, ephemeralEnvVars, "docker", "compose", "-f", dcc.composeFile, "up", "-d", "--force-recreate")
	if err != nil {
		return errors.Wrapf(err, "docker compose up failed")
	}

	// verify from docker api that the container is up
	isImageContainerRunning, err = dcc.IsServiceRunning(ctx, serviceName, timeout)
	if err != nil {
		return errors.Wrapf(err, "check for container running failed")
	}
	if !isImageContainerRunning {
		return errors.Wrapf(ErrContainerNotRunning, "compose up didn't start container")
	}

	return nil
}

func (dcc *ComposeClient) RestartServiceWithHaltHeight(ctx context.Context, composeConfig *config.ComposeCli, serviceName string, upgradeHeight int64) error {
	isImageContainerRunning, err := dcc.IsServiceRunning(ctx, serviceName, composeConfig.DownTimeout)
	if err != nil {
		return errors.Wrapf(err, "check for container running failed")
	}
	if !isImageContainerRunning {
		return errors.Wrapf(ErrContainerNotRunning, "expected the container to run before restarting with halt height")
	}
	// The check above is prone to race conditions and the
	// container can exit after the check. That should be super rare
	// and it should be safe to Down it anyways, since, we are sure that we want
	// to register halt height and restart.
	// If the container crashed due to some issue, let it crash again after the restart
	// blazar can't do anything about that
	err = dcc.Down(ctx, serviceName, composeConfig.DownTimeout)
	if err != nil && !errors.Is(err, ErrContainerNotRunning) {
		return errors.Wrapf(err, "docker compose down failed")
	}

	return dcc.Up(ctx, serviceName, composeConfig.UpDeadline, fmt.Sprintf("HALT_HEIGHT=%d", upgradeHeight))
}
func (dcc *ComposeClient) IsServiceRunning(ctx context.Context, serviceName string, timeout time.Duration) (bool, error) {
	// +1s to give some wiggle room for the docker compose cli to respond
	containerID, err := dcc.GetContainerID(ctx, serviceName, timeout+time.Second)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if service is running")
	}

	if containerID == "" {
		return false, nil
	}

	return dcc.client.IsContainerRunning(ctx, containerID)
}

func (dcc *ComposeClient) GetContainerID(ctx context.Context, serviceName string, timeout time.Duration) (string, error) {
	stdout, stderr, err := cmd.CheckOutputWithDeadline(
		ctx, timeout, []string{}, "docker", "compose", "-f", dcc.composeFile, "ps", "-a", "-q",
		// we are only interested in finding the running container
		// --status stringArray   Filter services by status. Values: [paused | restarting | removing | running | dead | created | exited]
		"--status", "restarting",
		"--status", "removing",
		"--status", "running",
		"--status", "created",
		serviceName,
	)
	if err != nil {
		return "", errors.Wrapf(err, "docker compose ps failed: %s", stderr.String())
	}

	// the cli returns the one container id per line, we expect only one container
	containers := []string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		// we expect the container id to be 64 characters long
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(trimmed) == 64 {
			containers = append(containers, trimmed)
		}
	}

	if len(containers) > 1 {
		return "", fmt.Errorf("multiple containers found for service: %s, IDs: %s", serviceName, containers)
	}

	// no container means likely that a service is down, this is not an error
	if len(containers) == 0 {
		return "", nil
	}

	return containers[0], nil
}

func (dcc *ComposeClient) Version(ctx context.Context) (string, error) {
	stdout, _, err := cmd.CheckOutputWithDeadline(ctx, 2*time.Second, []string{}, "docker", "compose", "version", "--short")

	return strings.ReplaceAll(stdout.String(), "\n", ""), err
}

func (dcc *ComposeClient) updateVersionFile(serviceName, newContent string) error {
	versions, err := GetServiceVersions(dcc.versionFile)
	if err != nil {
		return err
	}

	found := false
	lines := []string{}
	for _, service := range versions {
		if service.Name == serviceName {
			found = true
			service.Version = newContent
		}
		lines = append(lines, fmt.Sprintf("VERSION_%s=%s", service.Name, service.Version))
	}

	if !found {
		return fmt.Errorf("could not find VERSION_%s on %s", serviceName, dcc.versionFile)
	}

	err = os.WriteFile(dcc.versionFile, []byte(strings.Join(lines, "\n")), 0600)
	if err != nil {
		return errors.Wrapf(err, "failed to update the version file %s", dcc.versionFile)
	}

	return nil
}

type ServiceVersionLine struct {
	Name    string
	Version string
}

func GetServiceVersions(filename string) ([]ServiceVersionLine, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var services []ServiceVersionLine
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "VERSION_") {
			continue // Skip invalid lines
		}

		serviceName := strings.TrimPrefix(parts[0], "VERSION_")
		services = append(services, ServiceVersionLine{Name: serviceName, Version: parts[1]})
	}

	return services, nil
}

func LoadServiceVersionFile(versionFile, serviceName string) (string, error) {
	versions, err := GetServiceVersions(versionFile)
	if err != nil {
		return "", err
	}

	for _, ver := range versions {
		if ver.Name == serviceName {
			return ver.Version, nil
		}
	}

	return "", fmt.Errorf("could not find VERSION_%s on %s", serviceName, versionFile)
}

func LoadComposeFile(composeFile string) (*composeTypes.Project, error) {
	opts, err := compose.NewProjectOptions([]string{composeFile},
		compose.WithDiscardEnvFile,
		compose.WithOsEnv,
		compose.WithDotEnv,
		compose.WithInterpolation(true),
	)
	if err != nil {
		return nil, err
	}

	project, err := compose.ProjectFromOptions(opts)
	if err != nil {
		return nil, err
	}

	// this should not happen, but just in case
	if svcs := len(project.Services); svcs == 0 {
		return nil, errors.New("no services found in docker compose file")
	}

	for _, service := range project.Services {
		if service.Image == "" {
			return nil, fmt.Errorf("service %s has no image defined", service.Name)
		}
	}

	return project, nil
}

func verifyCompose(baseDir string, content string) error {
	// We need to create temp file in the same dir as the compose
	// may have an env_file section and loading it will fail if
	// the files aren't found
	f, err := os.CreateTemp(baseDir, "docker-compose-upgraded.*.blazar")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if err := os.WriteFile(f.Name(), []byte(content), 0600); err != nil {
		return err
	}

	_, err = LoadComposeFile(f.Name())
	if err != nil {
		return errors.Wrapf(err, "compose validation failed")
	}

	return nil
}

func readAndReplace(path, from, to string) (string, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file")
	}
	content := string(contentBytes)

	if strings.Count(content, from) != 1 {
		return "", fmt.Errorf("file contains multiple instances of %s string", from)
	}

	return strings.Replace(content, from, to, 1), nil
}

func updateComposeFile(composeFile, newContent string) error {
	composeDir := filepath.Dir(composeFile)
	composeFilePath := filepath.Base(composeFile)

	if err := verifyCompose(composeDir, newContent); err != nil {
		return err
	}

	// Backup the old file anyway
	filename := fmt.Sprintf("%s.%s.blazar.bkp", composeFilePath, time.Now().UTC().Format(time.RFC3339))
	err := os.Rename(composeFile, filepath.Join(composeDir, filename))
	if err != nil {
		return errors.Wrapf(err, "backup of %s failed", composeFile)
	}

	// Replace the old file with the new one
	err = os.WriteFile(composeFile, []byte(newContent), 0600)
	if err != nil {
		return errors.Wrapf(err, "failed to update compose file %s", composeFile)
	}

	return nil
}

func getServiceFromProject(serviceName string, project *composeTypes.Project) (*composeTypes.ServiceConfig, error) {
	for _, service := range project.Services {
		if service.Name == serviceName {
			return &service, nil
		}
	}
	return nil, fmt.Errorf("service %s not found in compose file", serviceName)
}
