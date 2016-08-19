package gs

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"golang.org/x/net/context"
)

// compareStringSlices compares two string slices, and returns true iff the
// contents and sequence of the two slices are identical.
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGetLatestGSDirs(t *testing.T) {
	startTS := time.Date(1970, time.November, 29, 13, 45, 20, 67, time.UTC).Unix()
	endTS := time.Date(1972, time.February, 2, 3, 45, 20, 67, time.UTC).Unix()
	results := GetLatestGSDirs(startTS, endTS, "prefix")
	expected := []string{
		"prefix/1970/11/29",
		"prefix/1970/11/30",
		"prefix/1970/12",
		"prefix/1971",
		"prefix/1972/01",
		"prefix/1972/02/01",
		"prefix/1972/02/02/00",
		"prefix/1972/02/02/01",
		"prefix/1972/02/02/02",
		"prefix/1972/02/02/03",
	}
	if !compareStringSlices(results, expected) {
		t.Errorf("GetLatestGSDirs unexpected results! Got:\n%s\nWant:\n%s", results, expected)
	}
}

func TestDownloadHelper(t *testing.T) {
	testutils.SkipIfShort(t)

	// Setup.
	workdir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)

	gs, err := storage.NewClient(context.Background())
	assert.NoError(t, err)

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
		assert.NoError(t, err)
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
	assert.True(t, os.IsNotExist(check(a, hashA)))

	// Download the binary.
	assert.NoError(t, d.MaybeDownload(a, hashA))
	assert.NoError(t, check(a, hashA))

	// Modify the binary.
	fakeContents := "blah blah blah"
	assert.NoError(t, ioutil.WriteFile(pathA, []byte(fakeContents), 0755))
	assert.NotNil(t, check(a, hashA))

	// Ensure that we end up with the right binary.
	assert.NoError(t, d.MaybeDownload(a, hashA))
	assert.NoError(t, check(a, hashA))
	contents, err := ioutil.ReadFile(pathA)
	assert.NoError(t, err)
	assert.NotEqual(t, fakeContents, string(contents))

	// chmod the binary.
	assert.NoError(t, os.Chmod(pathA, 0644))
	assert.NotNil(t, check(a, hashA))

	// Ensure that we end up with the right binary.
	assert.NoError(t, d.MaybeDownload(a, hashA))
	assert.NoError(t, check(a, hashA))
}
