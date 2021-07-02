package validate

import (
	"context"
	_ "embed"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestInstanceConfigBytes_AllExistingConfigs_ShouldBeValid(t *testing.T) {
	unittest.MediumTest(t)
	ctx := context.Background()

	allExistingConfigs, err := filepath.Glob("../../../configs/*.json")
	require.NoError(t, err)
	for _, filename := range allExistingConfigs {
		err := util.WithReadFile(filename, func(r io.Reader) error {
			b, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			_, err = InstanceConfigBytes(ctx, b)
			return err
		})
		require.NoError(t, err, filename)
	}
}

func TestInstanceConfigBytes_EmptyJSONObject_ShouldBeInValid(t *testing.T) {
	unittest.MediumTest(t)
	ctx := context.Background()

	_, err := InstanceConfigBytes(ctx, []byte("{}"))
	require.Error(t, err)
}
