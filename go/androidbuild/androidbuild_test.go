package androidbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func TestToFromKey(t *testing.T) {
	unittest.SmallTest(t)
	k, err := toKey("git_master-skia", "razor-userdebug", "1814540")
	if err != nil {
		t.Fatalf("Failed to create key: %s", err)
	}
	branch, target, buildID, err := fromKey(k)
	if err != nil {
		t.Fatalf("Failed to parse key: %s", err)
	}
	if got, want := branch, "git_master-skia"; got != want {
		t.Errorf("Parse key failed: Got %v Want %v", got, want)
	}
	if got, want := target, "razor-userdebug"; got != want {
		t.Errorf("Parse key failed: Got %v Want %v", got, want)
	}
	if got, want := buildID, "1814540"; got != want {
		t.Errorf("Parse key failed: Got %v Want %v", got, want)
	}

	// Now check error conditions.
	_, err = toKey("git_master-skia", "razor-userdebug", "not a number")
	if err == nil {
		t.Fatalf("Failed to create err on an invalid buildID.")
	}

	_, _, _, err = fromKey([]byte(":"))
	if err == nil {
		t.Fatalf("Failed to create err on an invalid key.")
	}
	_, _, _, err = fromKey([]byte("a:b:c"))
	if err == nil {
		t.Fatalf("Failed to create err on an invalid key.")
	}
}

type mockCommits struct {
}

func (m mockCommits) Get(branch, target, endBuildID string) (*vcsinfo.ShortCommit, error) {
	return nil, fmt.Errorf("always an error")
}

func (m mockCommits) List(branch, target, endBuildID string) (map[string]*vcsinfo.ShortCommit, error) {
	return map[string]*vcsinfo.ShortCommit{
		"100": {
			Hash:    "1234567890",
			Author:  "fred@example.com",
			Subject: "A commit",
		},
		"102": {
			Hash:    "987654321",
			Author:  "barney@example.com",
			Subject: "Another commit",
		},
	}, nil
}

func TestInfo(t *testing.T) {
	unittest.MediumTest(t)
	levelDBDir := filepath.Join(os.TempDir(), "android-leveldb")
	defer util.RemoveAll(levelDBDir)

	db, err := leveldb.OpenFile(levelDBDir, nil)
	if err != nil {
		t.Fatalf("Failed to create test leveldb: %s", err)
	}

	i := &info{
		db:      db,
		commits: mockCommits{},
	}
	assert.Equal(t, []string{}, i.branchtargets())
	i.single_poll()

	// The first time we Get on an unknown target well get nil, err.
	commit, err := i.Get("git_master-skia", "razor-userdebug", "100")
	assert.Nil(t, commit)
	assert.Error(t, err)

	// After the first time we'll get an error.
	commit, err = i.Get("git_master-skia", "razor-userdebug", "100")
	assert.Nil(t, commit)
	assert.Error(t, err)

	// Now poll.
	i.single_poll()

	// Now the commits should be populated so the Get should succeed.
	commit, err = i.Get("git_master-skia", "razor-userdebug", "100")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "1234567890"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "razor-userdebug", "101")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "1234567890"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "razor-userdebug", "103")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "987654321"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "razor-userdebug", "99")
	assert.Nil(t, commit)
	assert.Error(t, err)

	// Now add another target.

	// The first time we Get on an unknown target well get nil, err.
	commit, err = i.Get("git_master-skia", "volantis-userdebug", "100")
	assert.Nil(t, commit)
	assert.Error(t, err)

	// After the first time we'll get an error.
	commit, err = i.Get("git_master-skia", "volantis-userdebug", "100")
	assert.Nil(t, commit)
	assert.Error(t, err)

	// Now poll.
	i.single_poll()

	// Now the commits should be populated so the Get should succeed.
	commit, err = i.Get("git_master-skia", "volantis-userdebug", "100")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "1234567890"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "volantis-userdebug", "101")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "1234567890"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "volantis-userdebug", "105")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	if got, want := commit.Hash, "987654321"; got != want {
		t.Errorf("Wrong commit returned: Got %v Want %v", got, want)
	}

	commit, err = i.Get("git_master-skia", "volantis-userdebug", "99")
	assert.Nil(t, commit)
	assert.Error(t, err)
}
