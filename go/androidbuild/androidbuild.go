// androidbuild implements a simple interface to look up skia git commit
// hashes from android buildIDs.
package androidbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	lutil "github.com/syndtr/goleveldb/leveldb/util"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	NUM_RETRIES    = 10
	PAGE_SIZE      = 5
	SLEEP_DURATION = 3

	// TARGETS_KEY is the key for the list of space separated branch:targets we are monitoring.
	TARGETS_KEY = "android_ingest_targets"

	// LAST_BUILD_FOR_TARGET_TEMPLATE is expanded with the branch and target name to build they key for the last buildID seen for that target.
	LAST_BUILD_FOR_TARGET_TEMPLATE = "android_ingest_last_build:%s:%s"
)

// Info is an inferface for querying for commit info from an Android branch, target, and buildID.
type Info interface {
	Get(branch, target, buildID string) (*gitinfo.ShortCommit, error)
}

// info implements Info.
//
// It uses a leveldb database to store information it has already read.
//
// The first time a caller tries to Get(branch, target, buildID) and the
// (branch, target) pair has never been seen before it will be added to
// the list of (branch, target) pairs to periodically refresh. The pairs
// are stored in the leveldb at TARGETS_KEY in the leveldb.
//
// Periodically info will request the build info for all (branch:target)
// pairs it is monitoring. The request will be for all builds from the most
// current back to the last one seen. The last buildID seen for each (branch:target)
// pair is stored at LAST_BUILD_FOR_TARGET_TEMPLATE.
type info struct {
	db *leveldb.DB

	// commits is an interface for fetching data from the Android Build API,
	// broken out as an interface for testability.
	commits commits
}

// New creates a new *info as Info.
//
// dir is the directory where the leveldb that caches responses will be written.
// client must be an authenticated client to make requests to the Android Build API.
func New(dir string, client *http.Client) (Info, error) {
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil && errors.IsCorrupted(err) {
		db, err = leveldb.RecoverFile(dir, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to open leveldb at %s: %s", dir, err)
	}
	c, err := newAndroidCommits(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create commits: %s", err)
	}

	i := &info{db: db, commits: c}
	go i.poll()

	return i, nil
}

// Get returns the ShortCommit info for the given branch, target, and buildID.
func (i *info) Get(branch, target, buildID string) (*gitinfo.ShortCommit, error) {
	// Get the list of targets and confirm that this target is in it, otherwise add it to the list of targets.
	branchtargets := i.branchtargets()
	branchtarget := fmt.Sprintf("%s:%s", branch, target)
	if !util.In(branchtarget, branchtargets) {
		// If we aren't currently scanning results for this (branch, target) pair
		// then add it to the list.
		branchtargets = append(branchtargets, branchtarget)
		err := i.db.Put([]byte(TARGETS_KEY), []byte(strings.Join(branchtargets, " ")), nil)
		if err != nil {
			glog.Errorf("Failed to add new target %s: %s", branchtarget, err)
		}
		// Always try to fetch the information from the Android Build API directly if
		// we don't have it yet.
		return i.single_get(branch, target, buildID)
	} else {
		key, err := toKey(branch, target, buildID)
		if err != nil {
			return nil, fmt.Errorf("Can't Get with an invalid build ID %q: %s", buildID, err)
		}
		// Scan backwards through the build info until we find a buildID that is equal to or
		// comes before the buildID we are looking for.
		iter := i.db.NewIterator(lutil.BytesPrefix([]byte(toPrefix(branch, target))), nil)
		defer iter.Release()
		if found := iter.Seek([]byte(key)); found {
			value := &gitinfo.ShortCommit{}
			if err := json.Unmarshal(iter.Value(), value); err != nil {
				return nil, fmt.Errorf("Unable to deserialize value: %s", err)
			}
			return value, nil
		} else {
			return i.single_get(branch, target, buildID)
		}
	}
}

// single_get uses i.commits to attempt to retrieve a single commit, storing the results
// in the leveldb if successful.
func (i *info) single_get(branch, target, buildID string) (*gitinfo.ShortCommit, error) {
	c, err := i.commits.Get(branch, target, buildID)
	if err != nil {
		return nil, err
	}
	i.store(branch, target, buildID, c)
	return c, nil
}

// branchtargets returns the list of (branch:target)s we are monitoring.
func (i *info) branchtargets() []string {
	b, err := i.db.Get([]byte(TARGETS_KEY), nil)
	if err != nil {
		glog.Errorf("Failed to get TARGETS_KEY: %s", err)
		return []string{}
	}
	parts := strings.Split(string(b), " ")
	ret := []string{}
	for _, s := range parts {
		if s != "" {
			ret = append(ret, s)
		}
	}
	return ret
}

// store the given commit in the leveldb.
func (i *info) store(branch, target, buildID string, commit *gitinfo.ShortCommit) {
	key, err := toKey(branch, target, buildID)
	if err != nil {
		glog.Errorf("store: invalid build ID %s: %s", key, err)
		return
	}
	b, err := json.Marshal(commit)
	if err != nil {
		glog.Errorf("store: can't encode %#v: %s", commit, err)
		return
	}
	if err := i.db.Put([]byte(key), b, nil); err != nil {
		glog.Errorf("Failed to store commit: %s", err)
	}
}

// single_poll does a single loop of polling the API for each target we are monitoring.
func (i *info) single_poll() {
	for _, branchtarget := range i.branchtargets() {
		parts := strings.Split(branchtarget, ":")
		if len(parts) != 2 {
			glog.Errorf("Found an invalid branchtarget: %s", branchtarget)
			continue
		}
		branch := parts[0]
		target := parts[1]
		// Find the last buildID we've seen so far.
		lastBuildID := ""
		lastBuildIDKey := []byte(fmt.Sprintf(LAST_BUILD_FOR_TARGET_TEMPLATE, branch, target))
		b, err := i.db.Get(lastBuildIDKey, nil)
		if err == nil {
			lastBuildID = string(b)
		}

		// Query for commits from latest to lastBuildID.
		builds, err := i.commits.List(branch, target, lastBuildID)
		if err != nil {
			glog.Errorf("Failed to get commits for %s %s %s: %s", branch, target, lastBuildID, err)
			continue
		}
		// Save each buildID we found.
		for k, v := range builds {
			i.store(branch, target, k, v)
		}
		// Now find the largest buildID and store it.
		buildIDs := []int{}
		for id, _ := range builds {
			i, err := strconv.Atoi(id)
			if err == nil {
				buildIDs = append(buildIDs, i)
			}
		}
		sort.Sort(sort.IntSlice(buildIDs))
		if len(buildIDs) > 0 {
			lastBuildID = strconv.Itoa(buildIDs[len(buildIDs)-1])
			err := i.db.Put(lastBuildIDKey, []byte(lastBuildID), nil)
			if err != nil {
				glog.Errorf("Failed to write last build ID: %s", err)
			}
		}
	}
}

// poll periodically polls each target we are monitoring.
func (i *info) poll() {
	for _ = range time.Tick(time.Minute) {
		i.single_poll()
	}
}

// invertBSlice inverts the bytes in a slice so that sorting
// will work in the reverse order.
//
// So leveldb will only sort keys in ascending order, but we want to sort based
// in the buildID in reverse order, so we will invert the bytes so that sorting
// works in the direction that we want.
func invertBSlice(a []byte) []byte {
	inverted := make([]byte, len(a))
	copy(inverted, a)
	for i := 0; i < len(a); i++ {
		inverted[i] = 0xff - a[i]
	}
	return inverted
}

// toKey converts branch, target, and buildID into a string usable as a leveldb key.
//
// For example:
//
//     toKey("git_master-skia", "razor-userdebug", "1814540")
//
// Returns
//
//     "git_master-skia:razor-userdebug:<inverted bytes of 00000000000001814540>"
func toKey(branch, target, buildID string) ([]byte, error) {
	id, err := strconv.Atoi(buildID)
	if err != nil {
		return []byte{}, err
	}
	paddedID := []byte(fmt.Sprintf("%020d", id))
	return append([]byte(fmt.Sprintf("%s:%s:", branch, target)), invertBSlice(paddedID)...), nil
}

// toPrefix returns a prefix for restricting searches to one particular target in the leveldb.
func toPrefix(branch, target string) []byte {
	return []byte(fmt.Sprintf("%s:%s:", branch, target))
}

// fromKey converts a key returned from toKey back into (branch, target, buildID).
func fromKey(key []byte) (string, string, string, error) {
	parts := bytes.Split(key, []byte(":"))
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("Invalid key format, wrong number of parts: %q", key)
	}
	id, err := strconv.Atoi(string(invertBSlice(parts[2])))
	if err != nil {
		return "", "", "", fmt.Errorf("Invalid key format, buildID not a valid int: %q", key)
	}
	return string(parts[0]), string(parts[1]), strconv.Itoa(id), nil
}
