package gcs

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestDownloadHelper(t *testing.T) {

	// Setup.
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)

	gs, err := storage.NewClient(context.Background())
	require.NoError(t, err)

	d := NewDownloadHelper(gs, "skia-infra-testdata", "gs-testdata/hashed-binaries", workdir)

	check := func(executable, hash string) error {
		fp := path.Join(workdir, executable)
		info, err := os.Stat(fp)
		if err != nil {
			return err
		}
		if info.Mode() != 0755 {
			return fmt.Errorf("Not executable")
		}
		contents, err := ioutil.ReadFile(fp)
		require.NoError(t, err)
		sha1sum := fmt.Sprintf("%x", sha1.Sum(contents))
		if sha1sum != hash {
			return fmt.Errorf("Wrong hash.\nExpect: %s\nGot:    %s", hash, sha1sum)
		}
		return nil
	}

	a := "a.sh"
	hashA := "9189a75b337c003f542686e33b794cf5b7ffea57"
	pathA := path.Join(workdir, a)

	// Verify that we don't already have the binary.
	require.True(t, os.IsNotExist(check(a, hashA)))

	// Download the binary.
	require.NoError(t, d.MaybeDownload(a, hashA))
	require.NoError(t, check(a, hashA))

	// Modify the binary.
	fakeContents := "blah blah blah"
	require.NoError(t, ioutil.WriteFile(pathA, []byte(fakeContents), 0755))
	require.NotNil(t, check(a, hashA))

	// Ensure that we end up with the right binary.
	require.NoError(t, d.MaybeDownload(a, hashA))
	require.NoError(t, check(a, hashA))
	contents, err := ioutil.ReadFile(pathA)
	require.NoError(t, err)
	require.NotEqual(t, fakeContents, string(contents))

	// chmod the binary.
	require.NoError(t, os.Chmod(pathA, 0644))
	require.NotNil(t, check(a, hashA))

	// Ensure that we end up with the right binary.
	require.NoError(t, d.MaybeDownload(a, hashA))
	require.NoError(t, check(a, hashA))
}
