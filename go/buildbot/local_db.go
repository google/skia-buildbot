package buildbot

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/util"
)

var (
	// Builds.
	BUCKET_BUILD_NUMS_BY_COMMIT  = []byte("buildNums_byCommit")  // map[string]int
	BUCKET_BUILDS                = []byte("builds")              // map[BuildID]Build
	BUCKET_BUILDS_BY_COMMIT      = []byte("builds_byCommit")     // map[commit|BuildID]bit
	BUCKET_BUILDS_BY_FINISH_TIME = []byte("builds_byFinishTime") // map[time.Time|BuildID]bit

	// Build comments.
	BUCKET_BUILD_COMMENTS = []byte("buildComments") // map[id]BuildComment

	// Builder comments.
	BUCKET_BUILDER_COMMENTS            = []byte("builderComments")           // map[id]BuilderComment
	BUCKET_BUILDER_COMMENTS_BY_BUILDER = []byte("builderComments_byBuilder") // map[builder|id]id

	// Commit comments.
	BUCKET_COMMIT_COMMENTS           = []byte("commitComments")          // map[id]CommitComment
	BUCKET_COMMIT_COMMENTS_BY_COMMIT = []byte("commitComments_byCommit") // map[commit|id]id

	// Special keys for storing the next ID.
	KEY_BUILD_COMMENTS_NEXT_ID   = []byte("buildComments_nextID")
	KEY_BUILDER_COMMENTS_NEXT_ID = []byte("builderComments_nextID")
	KEY_COMMIT_COMMENTS_NEXT_ID  = []byte("commitComments_nextID")
)

const (
	// Initial ID number.
	INITIAL_ID = 1

	// If ingestion latency is greater than this, an alert will be triggered.
	INGEST_LATENCY_ALERT_THRESHOLD = 2 * time.Minute
)

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

func intToBytesBigEndian(i int64) []byte {
	rv := make([]byte, 8)
	binary.BigEndian.PutUint64(rv, uint64(i))
	return rv
}

func bytesToIntBigEndian(b []byte) (int64, error) {
	if len(b) != 8 {
		return -1, fmt.Errorf("Invalid length byte slice (%d); must be 8", len(b))
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

func key_BUILD_NUMS_BY_COMMIT(master, builder, c string) []byte {
	return []byte(fmt.Sprintf("%s|%s|%s", master, builder, c))
}

func key_BUILDS(b *Build) []byte {
	return b.Id()
}

func key_BUILDS_BY_COMMIT(b *Build, c string) []byte {
	return []byte(fmt.Sprintf("%s|%s", c, b.Id()))
}

func key_BUILDS_BY_FINISH_TIME(b *Build) []byte {
	return []byte(fmt.Sprintf("%s|%s", b.Finished.Format(time.RFC3339Nano), b.Id()))
}

func key_BUILDER_COMMENTS(id int64) []byte {
	return intToBytesBigEndian(id)
}

func key_BUILDER_COMMENTS_BY_BUILDER(builder string, id int64) []byte {
	return []byte(fmt.Sprintf("%s|%s", builder, string(key_BUILDER_COMMENTS(id))))
}

func key_COMMIT_COMMENTS(id int64) []byte {
	return intToBytesBigEndian(id)
}

func key_COMMIT_COMMENTS_BY_COMMIT(commit string, id int64) []byte {
	return []byte(fmt.Sprintf("%s|%s", commit, string(key_COMMIT_COMMENTS(id))))
}

// localDB is a struct used for interacting with a local database.
type localDB struct {
	db *bolt.DB
}

// NewLocalDB returns a local DB instance.
func NewLocalDB(filename string) (DB, error) {
	d, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}

	if err := d.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILD_NUMS_BY_COMMIT); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILDS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILDS_BY_COMMIT); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILDS_BY_FINISH_TIME); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILD_COMMENTS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILDER_COMMENTS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_BUILDER_COMMENTS_BY_BUILDER); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_COMMIT_COMMENTS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_COMMIT_COMMENTS_BY_COMMIT); err != nil {
			return err
		}

		// Initialize special next-id entries.
		var initialId bytes.Buffer
		if err := gob.NewEncoder(&initialId).Encode(INITIAL_ID); err != nil {
			return err
		}
		if tx.Bucket(BUCKET_BUILD_COMMENTS).Get(KEY_BUILD_COMMENTS_NEXT_ID) == nil {
			if err := tx.Bucket(BUCKET_BUILD_COMMENTS).Put(KEY_BUILD_COMMENTS_NEXT_ID, initialId.Bytes()); err != nil {
				return err
			}
		}
		if tx.Bucket(BUCKET_BUILDER_COMMENTS).Get(KEY_BUILDER_COMMENTS_NEXT_ID) == nil {
			if err := tx.Bucket(BUCKET_BUILDER_COMMENTS).Put(KEY_BUILDER_COMMENTS_NEXT_ID, initialId.Bytes()); err != nil {
				return err
			}
		}
		if tx.Bucket(BUCKET_COMMIT_COMMENTS).Get(KEY_COMMIT_COMMENTS_NEXT_ID) == nil {
			if err := tx.Bucket(BUCKET_COMMIT_COMMENTS).Put(KEY_COMMIT_COMMENTS_NEXT_ID, initialId.Bytes()); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &localDB{d}, nil
}

// Close closes the db.
func (d *localDB) Close() error {
	return d.db.Close()
}

// See documentation for DB interface.
func (d *localDB) BuildExists(master, builder string, number int) (bool, error) {
	rv := false
	if err := d.db.View(func(tx *bolt.Tx) error {
		rv = (tx.Bucket(BUCKET_BUILDS).Get(MakeBuildID(master, builder, number)) != nil)
		return nil
	}); err != nil {
		return false, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuildNumberForCommit(master, builder, commit string) (int, error) {
	n := -1
	if err := d.db.View(func(tx *bolt.Tx) error {
		serialized := tx.Bucket(BUCKET_BUILD_NUMS_BY_COMMIT).Get(key_BUILD_NUMS_BY_COMMIT(master, builder, commit))
		if serialized == nil {
			// No build exists at this commit, which is okay. Return -1 as the build number.
			return nil
		}
		if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&n); err != nil {
			return fmt.Errorf("GetBuildNumberForCommit: failed to decode stored number: %s", err)
		}
		return nil
	}); err != nil {
		return -1, err
	}
	return n, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuildsForCommits(commits []string, ignore map[string]bool) (map[string][]*Build, error) {
	rv := map[string][]*Build{}
	if err := d.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(BUCKET_BUILDS_BY_COMMIT).Cursor()
		for _, c := range commits {
			ids := []BuildID{}
			for k, v := cursor.Seek([]byte(c)); bytes.HasPrefix(k, []byte(c)); k, v = cursor.Next() {
				ids = append(ids, v)
			}
			builds, err := getBuilds(tx, ids)
			if err != nil {
				return err
			}
			rv[c] = builds
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuild(id BuildID) (*Build, error) {
	var rv *Build
	if err := d.db.View(func(tx *bolt.Tx) error {
		b, err := getBuild(tx, id)
		if err != nil {
			return err
		}
		rv = b
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuildFromDB(master, builder string, number int) (*Build, error) {
	return d.GetBuild(MakeBuildID(master, builder, number))
}

// getBuild retrieves the given build from the database.
func getBuild(tx *bolt.Tx, id BuildID) (*Build, error) {
	serialized := tx.Bucket(BUCKET_BUILDS).Get(id)
	if serialized == nil {
		return nil, fmt.Errorf("No such build in DB: %s", id)
	}
	var b Build
	if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&b); err != nil {
		return nil, err
	}
	b.fixup()
	return &b, nil
}

// getBuilds retrieves the given builds from the database.
func getBuilds(tx *bolt.Tx, ids []BuildID) ([]*Build, error) {
	rv := make([]*Build, 0, len(ids))
	for _, id := range ids {
		b, err := getBuild(tx, id)
		if err != nil {
			return nil, err
		}
		rv = append(rv, b)
	}
	return rv, nil
}

// insertBuild inserts the Build into the database.
func insertBuild(tx *bolt.Tx, b *Build) error {
	// Insert the build into the various buckets.
	id := b.Id()
	b.fixup()

	// Builds.
	var serialized bytes.Buffer
	if err := gob.NewEncoder(&serialized).Encode(b); err != nil {
		return err
	}
	if err := tx.Bucket(BUCKET_BUILDS).Put(id, serialized.Bytes()); err != nil {
		return err
	}

	// Builds by finish time.
	if err := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Put(key_BUILDS_BY_FINISH_TIME(b), id); err != nil {
		return err
	}

	for _, c := range b.Commits {
		// Builds by commit.
		if err := tx.Bucket(BUCKET_BUILDS_BY_COMMIT).Put(key_BUILDS_BY_COMMIT(b, c), id); err != nil {
			return err
		}

		// Build num by commit.
		var numVal bytes.Buffer
		if err := gob.NewEncoder(&numVal).Encode(b.Number); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_BUILD_NUMS_BY_COMMIT).Put(key_BUILD_NUMS_BY_COMMIT(b.Master, b.Builder, c), numVal.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// deleteBuild removes the Build from the database.
func deleteBuild(tx *bolt.Tx, id BuildID) error {
	builds := tx.Bucket(BUCKET_BUILDS)

	// Retrieve the old build.
	serialized := builds.Get(id)
	if serialized == nil {
		return fmt.Errorf("The given build %q does not exist in %s", id, string(BUCKET_BUILDS))
	}
	var b Build
	if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&b); err != nil {
		return err
	}

	// Remove the build from each bucket, ending with the builds bucket.

	for _, c := range b.Commits {
		// Build num by commit.
		if err := tx.Bucket(BUCKET_BUILD_NUMS_BY_COMMIT).Delete(key_BUILD_NUMS_BY_COMMIT(b.Master, b.Builder, c)); err != nil {
			return err
		}

		// Builds by commit.
		if err := tx.Bucket(BUCKET_BUILDS_BY_COMMIT).Delete(key_BUILDS_BY_COMMIT(&b, c)); err != nil {
			return err
		}

	}

	// Builds by finish time.
	if err := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Delete(key_BUILDS_BY_FINISH_TIME(&b)); err != nil {
		return err
	}

	// Builds.
	if err := builds.Delete(id); err != nil {
		return err
	}

	return nil
}

// putBuild inserts the build into the database, replacing any previous version.
func putBuild(tx *bolt.Tx, b *Build) error {
	id := b.Id()
	if tx.Bucket(BUCKET_BUILDS).Get(id) == nil {
		// Measure the time between build start and first DB insertion.
		latency := time.Now().Sub(b.Started)
		if latency > INGEST_LATENCY_ALERT_THRESHOLD {
			// This is probably going to trigger an alert. Log the build for debugging.
			glog.Warningf("Build start to ingestion latency is greater than %s (%s): %s %s #%d", INGEST_LATENCY_ALERT_THRESHOLD, latency, b.Master, b.Builder, b.Number)
		}
		metrics.GetOrRegisterSlidingWindow("buildbot.startToIngestLatency", metrics.DEFAULT_WINDOW).Update(int64(latency))
	} else {
		if err := deleteBuild(tx, id); err != nil {
			return err
		}
	}
	return insertBuild(tx, b)
}

// putBuilds inserts the builds into the database, replacing any previous versions.
func putBuilds(tx *bolt.Tx, builds []*Build) error {
	for _, b := range builds {
		id := b.Id()
		if tx.Bucket(BUCKET_BUILDS).Get(id) != nil {
			if err := deleteBuild(tx, id); err != nil {
				return err
			}
		}
	}
	for _, b := range builds {
		if err := insertBuild(tx, b); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) PutBuild(b *Build) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return putBuild(tx, b)
	})
}

// See documentation for DB interface.
func (d *localDB) PutBuilds(builds []*Build) error {
	defer metrics.NewTimer("buildbot.PutBuilds").Stop()
	return d.db.Update(func(tx *bolt.Tx) error {
		return putBuilds(tx, builds)
	})
}

// See documentation for DB interface.
func (d *localDB) GetLastProcessedBuilds(m string) ([]BuildID, error) {
	rv := []BuildID{}
	// Seek to the end of the bucket, grab the last build for each builder.
	if err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(BUCKET_BUILDS)
		c := b.Cursor()
		k, _ := c.Last()
		if k == nil {
			// The bucket is empty.
			return nil
		}
		for k != nil {
			// We're seeked to the last build on the current builder.
			// Add the build ID to the slice.
			master, builder, number, err := ParseBuildID(k)
			if err != nil {
				return err
			}
			if master == m {
				rv = append(rv, MakeBuildID(master, builder, number))
			}

			// Seek to the first build on the current builder.
			k, _ = c.Seek(MakeBuildID(master, builder, 0))

			// The build before the first build on the current builder is
			// the last build on the previous builder.
			k, _ = c.Prev()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetUnfinishedBuilds(master string) ([]*Build, error) {
	prefix := []byte(fmt.Sprintf("%s|%s|", util.TimeUnixZero.Format(time.RFC3339Nano), master))
	var rv []*Build
	if err := d.db.View(func(tx *bolt.Tx) error {
		ids := []BuildID{}
		cursor := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Cursor()
		for k, v := cursor.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = cursor.Next() {
			ids = append(ids, v)
		}
		builds, err := getBuilds(tx, ids)
		if err != nil {
			return err
		}
		rv = builds
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuildsFromDateRange(start, end time.Time) ([]*Build, error) {
	min := []byte(start.Format(time.RFC3339))
	max := []byte(start.Format(time.RFC3339))
	var rv []*Build
	if err := d.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Cursor()
		ids := []BuildID{}
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			ids = append(ids, v)
		}
		builds, err := getBuilds(tx, ids)
		if err != nil {
			return err
		}
		rv = builds
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetMaxBuildNumber(master, builder string) (int, error) {
	var rv int
	if err := d.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BUCKET_BUILDS).Cursor()
		maxID := MakeBuildID(master, builder, -1)
		_, _ = c.Seek(maxID)
		k, _ := c.Prev()
		_, _, n, err := ParseBuildID(k)
		if err != nil {
			return err
		}
		rv = n
		return nil
	}); err != nil {
		return -1, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) NumIngestedBuilds() (int, error) {
	var n int
	if err := d.db.View(func(tx *bolt.Tx) error {
		n = tx.Bucket(BUCKET_BUILDS).Stats().KeyN
		return nil
	}); err != nil {
		return -1, err
	}
	return n, nil
}

// See documentation for DB interface.
func (d *localDB) PutBuildComment(master, builder string, number int, c *BuildComment) error {
	if c.Id != 0 {
		return fmt.Errorf("Build comments cannot be edited.")
	}
	if err := d.db.Update(func(tx *bolt.Tx) error {
		build, err := getBuild(tx, MakeBuildID(master, builder, number))
		if err != nil {
			return fmt.Errorf("Failed to retrieve build: %s", err)
		}
		// This is a new comment. Determine which ID to use, and set the next ID.
		nextIdSerialized := tx.Bucket(BUCKET_BUILD_COMMENTS).Get(KEY_BUILD_COMMENTS_NEXT_ID)
		var nextId int64
		if err := gob.NewDecoder(bytes.NewBuffer(nextIdSerialized)).Decode(&nextId); err != nil {
			return err
		}
		c.Id = nextId
		nextId++
		nextIdSerialized2 := bytes.NewBuffer(nil)
		if err := gob.NewEncoder(nextIdSerialized2).Encode(nextId); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_BUILD_COMMENTS).Put(KEY_BUILD_COMMENTS_NEXT_ID, nextIdSerialized2.Bytes()); err != nil {
			return err
		}
		build.Comments = append(build.Comments, c)
		return putBuild(tx, build)
	}); err != nil {
		c.Id = 0
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) DeleteBuildComment(master, builder string, number int, id int64) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		build, err := getBuild(tx, MakeBuildID(master, builder, number))
		if err != nil {
			return fmt.Errorf("Failed to retrieve build: %s", err)
		}
		idx := -1
		for i, c := range build.Comments {
			if c.Id == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("No such comment: %d", id)
		}
		build.Comments = append(build.Comments[:idx], build.Comments[idx+1:]...)
		return putBuild(tx, build)
	})
}

// getBuilderComment returns the given builder comment.
func getBuilderComment(tx *bolt.Tx, id []byte) (*BuilderComment, error) {
	serialized := tx.Bucket(BUCKET_BUILDER_COMMENTS).Get(id)
	if serialized == nil {
		return nil, fmt.Errorf("No such comment: %v", id)
	}
	var comment BuilderComment
	if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// getBuilderComments returns the comments for the given builder.
func getBuilderComments(tx *bolt.Tx, builder string) ([]*BuilderComment, error) {
	c := tx.Bucket(BUCKET_BUILDER_COMMENTS_BY_BUILDER).Cursor()
	ids := [][]byte{}
	for k, v := c.Seek([]byte(builder)); bytes.HasPrefix(k, []byte(fmt.Sprintf("%s|", builder))); k, v = c.Next() {
		ids = append(ids, v)
	}
	rv := make([]*BuilderComment, 0, len(ids))
	for _, id := range ids {
		comment, err := getBuilderComment(tx, id)
		if err != nil {
			return nil, err
		}
		rv = append(rv, comment)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuilderComments(builder string) ([]*BuilderComment, error) {
	var rv []*BuilderComment
	if err := d.db.View(func(tx *bolt.Tx) error {
		comments, err := getBuilderComments(tx, builder)
		if err != nil {
			return err
		}
		rv = comments
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetBuildersComments(builders []string) (map[string][]*BuilderComment, error) {
	rv := map[string][]*BuilderComment{}
	if err := d.db.View(func(tx *bolt.Tx) error {
		for _, b := range builders {
			comments, err := getBuilderComments(tx, b)
			if err != nil {
				return err
			}
			rv[b] = comments
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) PutBuilderComment(c *BuilderComment) error {
	oldId := c.Id
	if err := d.db.Update(func(tx *bolt.Tx) error {
		if c.Id == 0 {
			// This is a new comment. Determine which ID to use, and set the next ID.
			nextIdSerialized := tx.Bucket(BUCKET_BUILDER_COMMENTS).Get(KEY_BUILDER_COMMENTS_NEXT_ID)
			var nextId int64
			if err := gob.NewDecoder(bytes.NewBuffer(nextIdSerialized)).Decode(&nextId); err != nil {
				return err
			}
			c.Id = nextId
			nextId++
			nextIdSerialized2 := bytes.NewBuffer(nil)
			if err := gob.NewEncoder(nextIdSerialized2).Encode(nextId); err != nil {
				return err
			}
			if err := tx.Bucket(BUCKET_BUILDER_COMMENTS).Put(KEY_BUILDER_COMMENTS_NEXT_ID, nextIdSerialized2.Bytes()); err != nil {
				return err
			}
		} else {
			if tx.Bucket(BUCKET_BUILDER_COMMENTS).Get(key_BUILDER_COMMENTS(c.Id)) == nil {
				return fmt.Errorf("Cannot update a build that does not already exist!")
			}
		}
		key := key_BUILDER_COMMENTS(c.Id)
		var serialized bytes.Buffer
		if err := gob.NewEncoder(&serialized).Encode(c); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_BUILDER_COMMENTS).Put(key, serialized.Bytes()); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_BUILDER_COMMENTS_BY_BUILDER).Put(key_BUILDER_COMMENTS_BY_BUILDER(c.Builder, c.Id), key); err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.Id = oldId
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) DeleteBuilderComment(id int64) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		key := key_BUILDER_COMMENTS(id)
		comment, err := getBuilderComment(tx, key)
		if err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_BUILDER_COMMENTS_BY_BUILDER).Delete(key_BUILDER_COMMENTS_BY_BUILDER(comment.Builder, id)); err != nil {
			return err
		}
		return tx.Bucket(BUCKET_BUILDER_COMMENTS).Delete(key)
	})
}

// getCommitComment returns the given builder comment.
func getCommitComment(tx *bolt.Tx, id []byte) (*CommitComment, error) {
	serialized := tx.Bucket(BUCKET_COMMIT_COMMENTS).Get(id)
	if serialized == nil {
		return nil, fmt.Errorf("No such comment: %v", id)
	}
	var comment CommitComment
	if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// getCommitComments returns the comments for the given builder.
func getCommitComments(tx *bolt.Tx, commit string) ([]*CommitComment, error) {
	c := tx.Bucket(BUCKET_COMMIT_COMMENTS_BY_COMMIT).Cursor()
	ids := [][]byte{}
	for k, v := c.Seek([]byte(commit)); bytes.HasPrefix(k, []byte(fmt.Sprintf("%s|", commit))); k, v = c.Next() {
		ids = append(ids, v)
	}
	rv := make([]*CommitComment, 0, len(ids))
	for _, id := range ids {
		comment, err := getCommitComment(tx, id)
		if err != nil {
			return nil, err
		}
		rv = append(rv, comment)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetCommitComments(commit string) ([]*CommitComment, error) {
	var rv []*CommitComment
	if err := d.db.View(func(tx *bolt.Tx) error {
		comments, err := getCommitComments(tx, commit)
		if err != nil {
			return err
		}
		rv = comments
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) GetCommitsComments(commits []string) (map[string][]*CommitComment, error) {
	rv := map[string][]*CommitComment{}
	if err := d.db.View(func(tx *bolt.Tx) error {
		for _, c := range commits {
			comments, err := getCommitComments(tx, c)
			if err != nil {
				return err
			}
			rv[c] = comments
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) PutCommitComment(c *CommitComment) error {
	oldId := c.Id
	if err := d.db.Update(func(tx *bolt.Tx) error {
		if c.Id == 0 {
			// This is a new comment. Determine which ID to use, and set the next ID.
			nextIdSerialized := tx.Bucket(BUCKET_COMMIT_COMMENTS).Get(KEY_COMMIT_COMMENTS_NEXT_ID)
			var nextId int64
			if err := gob.NewDecoder(bytes.NewBuffer(nextIdSerialized)).Decode(&nextId); err != nil {
				return err
			}
			c.Id = nextId
			nextId++
			nextIdSerialized2 := bytes.NewBuffer(nil)
			if err := gob.NewEncoder(nextIdSerialized2).Encode(nextId); err != nil {
				return err
			}
			if err := tx.Bucket(BUCKET_COMMIT_COMMENTS).Put(KEY_COMMIT_COMMENTS_NEXT_ID, nextIdSerialized2.Bytes()); err != nil {
				return err
			}
		} else {
			if tx.Bucket(BUCKET_COMMIT_COMMENTS).Get(key_COMMIT_COMMENTS(c.Id)) == nil {
				return fmt.Errorf("Cannot update a build that does not already exist!")
			}
		}
		key := key_COMMIT_COMMENTS(c.Id)
		var serialized bytes.Buffer
		if err := gob.NewEncoder(&serialized).Encode(c); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_COMMIT_COMMENTS).Put(key, serialized.Bytes()); err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_COMMIT_COMMENTS_BY_COMMIT).Put(key_COMMIT_COMMENTS_BY_COMMIT(c.Commit, c.Id), key); err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.Id = oldId
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) DeleteCommitComment(id int64) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		key := key_COMMIT_COMMENTS(id)
		comment, err := getCommitComment(tx, key)
		if err != nil {
			return err
		}
		if err := tx.Bucket(BUCKET_COMMIT_COMMENTS_BY_COMMIT).Delete(key_COMMIT_COMMENTS_BY_COMMIT(comment.Commit, id)); err != nil {
			return err
		}
		return tx.Bucket(BUCKET_COMMIT_COMMENTS).Delete(key_COMMIT_COMMENTS(id))
	})
}

// RunBackupServer runs an HTTP server which provides downloadable database backups.
func RunBackupServer(db DB, port string) error {
	local, ok := db.(*localDB)
	if !ok {
		return fmt.Errorf("Cannot run a backup server for a non-local database.")
	}
	r := mux.NewRouter()
	r.HandleFunc("/backup", func(w http.ResponseWriter, r *http.Request) {
		if err := local.db.View(func(tx *bolt.Tx) error {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\"buildbot.db\"")
			w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(w)
			return err
		}); err != nil {
			util.ReportError(w, r, err, fmt.Sprintf("Failed to create DB backup: %s", err))
		}
	})
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	return http.ListenAndServe(port, nil)
}
