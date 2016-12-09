package poprepo

import (
	"io/ioutil"
	"testing"

	"go.skia.org/infra/go/git/testutils"
	testsize "go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	testsize.MediumTest(t)
	gb := testutils.GitInit(t)
	defer gb.Cleanup()

	gb.Add(BUILDID_FILENAME, "0 0")
	gb.CommitMsg("init")

	// Create tmp dir that gets cleaned up.
	workdir, err := ioutil.TempDir("", "poprepo")
	assert.NoError(t, err)
	defer func() {
		//	_ = os.RemoveAll(workdir)
	}()

	// Start testing.
	p, err := NewPopRepo(gb.Dir(), workdir)
	assert.NoError(t, err)

	buildid, ts, err := p.GetLast()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), buildid)
	assert.Equal(t, int64(0), ts)

	err = p.Add(3516196, 1479855768)
	assert.NoError(t, err)

	err = p.Add(3516727, 1479863307)
	assert.NoError(t, err)

	// Try to add something wrong.
	err = p.Add(3516727-1, 1479863307-1)
	assert.Error(t, err)
	buildid, ts, err = p.GetLast()
	assert.Equal(t, int64(3516727), buildid)
	assert.Equal(t, int64(1479863307), ts)
}
