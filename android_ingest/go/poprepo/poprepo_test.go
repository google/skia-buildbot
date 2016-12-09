package poprepo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.skia.org/infra/go/exec"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	srcname, err := ioutil.TempDir("", "poprepo")
	assert.NoError(t, err)
	workdir, err := ioutil.TempDir("", "poprepo")
	assert.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(srcname)
		_ = os.RemoveAll(workdir)
	}()

	err = exec.Run(&exec.Command{
		Name: "git",
		Args: []string{"init"},
		Dir:  srcname,
	})
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(srcname, BUILDID_FILENAME), []byte("0 0"), 0644)
	assert.NoError(t, err)
	err = exec.Run(&exec.Command{
		Name: "git",
		Args: []string{"add", "--all"},
		Dir:  srcname,
	})
	assert.NoError(t, err)

	err = exec.Run(&exec.Command{
		Name: "git",
		Args: []string{"commit", "-m", "init"},
		Dir:  srcname,
	})
	assert.NoError(t, err)

	_, err = NewPopRepo(srcname, workdir)
	assert.NoError(t, err)
}
