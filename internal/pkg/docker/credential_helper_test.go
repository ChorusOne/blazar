package docker

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"blazar/internal/pkg/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandCredentialHelper(t *testing.T) {
	tempDir := testutils.PrepareTestData(t, "docker", "docker-credential-helper", "")

	tests := []struct {
		name     string
		file     string
		assertFn func(error)
	}{
		{
			name: "Valid",
			file: "/valid.sh",
			assertFn: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "WrongJson",
			file: "/wrong-json.sh",
			assertFn: func(err error) {
				assert.ErrorIs(t, err, ErrCredHelperEmpty)
			},
		},
		{
			name: "Exit1",
			file: "/exit-1.sh",
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "exit status 1")
			},
		},
		{
			name: "Sleep",
			file: "/sleep.sh",
			assertFn: func(err error) {
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.ErrorContains(t, err, "signal: killed")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			credHelper := commandCredentialHelper{
				command: filepath.Join(tempDir, test.file),
				timeout: time.Second,
			}

			_, err := credHelper.GetRegistryAuth(context.Background())
			test.assertFn(err)
		})
	}
}
