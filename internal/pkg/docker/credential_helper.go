package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"blazar/internal/pkg/cmd"
	"blazar/internal/pkg/errors"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker/api/types/registry"
)

var (
	ErrCredHelperEmpty = errors.New("docker credential helper returned empty username or password")
)

type CredentialHelper interface {
	GetRegistryAuth(ctx context.Context) (string, error)
}

type commandCredentialHelper struct {
	command string
	timeout time.Duration
}

func (cch *commandCredentialHelper) GetRegistryAuth(ctx context.Context) (string, error) {
	stdout, _, err := cmd.CheckOutputWithDeadline(ctx, cch.timeout, []string{}, cch.command, "get")
	if err != nil {
		return "", errors.Wrapf(err, "error running docker credential helper")
	}

	var creds credentials.Credentials
	if err := json.Unmarshal(stdout.Bytes(), &creds); err != nil {
		return "", errors.Wrapf(err, "error unmarshalling docker credential helper output")
	}
	if creds.Username == "" || creds.Secret == "" {
		return "", ErrCredHelperEmpty
	}

	auth := registry.AuthConfig{
		Username: creds.Username,
		Password: creds.Secret,
	}

	jsonAuth, err := json.Marshal(auth)
	if err != nil {
		return "", errors.Wrapf(err, "error marshalling docker auth config")
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func NewCredentialHelper(command string, timeout time.Duration) CredentialHelper {
	return &commandCredentialHelper{
		command: command,
		timeout: timeout,
	}
}
