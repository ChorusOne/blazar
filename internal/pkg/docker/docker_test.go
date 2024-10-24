package docker

import (
	"fmt"
	"testing"

	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsImagePresent(t *testing.T) {
	img, ver := "img", "tag"
	testutils.MakeImageWith(t, img, ver, dockerProvider)
	tests := []struct {
		image     string
		isPresent bool
	}{
		{
			image:     fmt.Sprintf("%s:%s", img, ver),
			isPresent: true,
		},
		{
			image:     fmt.Sprintf("%s:%sinvalidtag", img, ver),
			isPresent: false,
		},
	}

	ctx, dc := newDockerClientWithCtx(t)

	for _, test := range tests {
		isPresent, err := dc.IsImagePresent(ctx, test.image)
		require.NoError(t, err)
		assert.Equal(t, test.isPresent, isPresent)
	}
}

func TestParseImageName(t *testing.T) {
	tests := []struct {
		imageName string
		image     string
		tag       string
		err       error
	}{
		{
			imageName: "testrepo/testimage:latest",
			image:     "testrepo/testimage",
			tag:       "latest",
			err:       nil,
		},
		{
			imageName: "testrepo/testimage:latest:invalid",
			image:     "",
			tag:       "",
			err:       errors.New("invalid image name: testrepo/testimage:latest:invalid"),
		},
		{
			imageName: "testrepo/testimage",
			image:     "",
			tag:       "",
			err:       errors.New("invalid image name: testrepo/testimage"),
		},
	}

	for _, test := range tests {
		image, tag, err := ParseImageName(test.imageName)
		assert.Equal(t, test.image, image)
		assert.Equal(t, test.tag, tag)
		assert.Equal(t, test.err, err)
	}
}

func TestPullImage(t *testing.T) {
	ctx, dc := newDockerClientWithCtx(t)

	// We need to use an existing image for this test
	err := dc.PullImage(ctx, "luca3m/sleep:latest", "linux/amd64")
	require.NoError(t, err)

	err = dc.PullImage(ctx, "luca3m/sleep:invalidtag", "linux/amd64")
	assert.Error(t, err)
}
