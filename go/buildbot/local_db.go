package buildbot

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
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

	// Maximum number of simultaneous GetModifiedBuilds users.
	MAX_MODIFIED_BUILDS_USERS = 10

	// Expiration for GetModifiedBuilds users.
	MODIFIED_BUILDS_TIMEOUT = 10 * time.Minute
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

func checkInterrupt(stop <-chan struct{}) error {
	select {
	case <-stop:
		sklog.Errorf("Transaction interrupted!")
		return fmt.Errorf("Transaction was interrupted.")
	default:
		return nil
	}
}

// localDB is a struct used for interacting with a local database.
type localDB struct {
	db *bolt.DB

	txCount  *metrics2.Counter
	txNextId int64
	txActive map[int64]string
	txMutex  sync.RWMutex

	modBuilds map[string]map[string][]byte
	modExpire map[string]time.Time
	modMutex  sync.RWMutex
}

// startTx monitors when a transaction starts.
func (d *localDB) startTx(name string) int64 {
	d.txMutex.Lock()
	defer d.txMutex.Unlock()
	d.txCount.Inc(1)
	id := d.txNextId
	d.txActive[id] = name
	d.txNextId++
	return id
}

// endTx monitors when a transaction ends.
func (d *localDB) endTx(id int64) {
	d.txMutex.Lock()
	defer d.txMutex.Unlock()
	d.txCount.Dec(1)
	delete(d.txActive, id)
}

// reportActiveTx prints out the list of active transactions.
func (d *localDB) reportActiveTx() {
	d.txMutex.RLock()
	defer d.txMutex.RUnlock()
	if len(d.txActive) == 0 {
		sklog.Infof("Active Transactions: (none)")
		return
	}
	txs := make([]string, 0, len(d.txActive))
	for id, name := range d.txActive {
		txs = append(txs, fmt.Sprintf("  %d\t%s", id, name))
	}
	sklog.Infof("Active Transactions:\n%s", strings.Join(txs, "\n"))
}

// tx is a wrapper for a BoltDB transaction which tracks statistics.
func (d *localDB) tx(name string, fn func(*bolt.Tx) error, update bool) error {
	txId := d.startTx(name)
	defer d.endTx(txId)
	defer metrics2.NewTimer("db-tx-duration", map[string]string{
		"database":    "buildbot",
		"transaction": name,
	}).Stop()
	if update {
		return d.db.Update(fn)
	} else {
		return d.db.View(fn)
	}
}

// view is a wrapper for the BoltDB instance's View method.
func (d *localDB) view(name string, fn func(*bolt.Tx) error) error {
	return d.tx(name, fn, false)
}

// update is a wrapper for the BoltDB instance's Update method.
func (d *localDB) update(name string, fn func(*bolt.Tx) error) error {
	return d.tx(name, fn, true)
}

// NewLocalDB returns a local DB instance.
func NewLocalDB(filename string) (DB, error) {
	d, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}
	db := &localDB{
		db: d,
		txCount: metrics2.GetCounter("db-active-tx", map[string]string{
			"database": "buildbot",
		}),
		txNextId: 0,
		txActive: map[int64]string{},
		txMutex:  sync.RWMutex{},

		modBuilds: map[string]map[string][]byte{},
		modExpire: map[string]time.Time{},
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			db.reportActiveTx()
			db.clearExpiredModifiedUsers()
		}
	}()

	if err := db.update("NewLocalDB", func(tx *bolt.Tx) error {
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

	return db, nil
}

// Close closes the db.
func (d *localDB) Close() error {
	return d.db.Close()
}

// See documentation for DB interface.
func (d *localDB) BuildExists(master, builder string, number int) (bool, error) {
	rv := false
	if err := d.view("BuildExists", func(tx *bolt.Tx) error {
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
	if err := d.view("GetBuildNumberForCommit", func(tx *bolt.Tx) error {
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
func (d *localDB) getBuildsForCommits(commits []string, ignore map[string]bool, stop <-chan struct{}) (map[string][]*Build, error) {
	rv := map[string][]*Build{}
	if err := d.view("GetBuildsForCommits", func(tx *bolt.Tx) error {
		cursor := tx.Bucket(BUCKET_BUILDS_BY_COMMIT).Cursor()
		for _, c := range commits {
			if err := checkInterrupt(stop); err != nil {
				return err
			}
			ids := []BuildID{}
			for k, v := cursor.Seek([]byte(c)); bytes.HasPrefix(k, []byte(c)); k, v = cursor.Next() {
				if err := checkInterrupt(stop); err != nil {
					return err
				}
				ids = append(ids, v)
			}
			if err := checkInterrupt(stop); err != nil {
				return err
			}
			builds, err := getBuilds(tx, ids, stop)
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
func (d *localDB) GetBuildsForCommits(commits []string, ignore map[string]bool) (map[string][]*Build, error) {
	return d.getBuildsForCommits(commits, ignore, make(chan struct{}))
}

// See documentation for DB interface.
func (d *localDB) GetBuild(id BuildID) (*Build, error) {
	var rv *Build
	if err := d.view("GetBuild", func(tx *bolt.Tx) error {
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
func getBuilds(tx *bolt.Tx, ids []BuildID, stop <-chan struct{}) ([]*Build, error) {
	rv := make([]*Build, 0, len(ids))
	for _, id := range ids {
		if err := checkInterrupt(stop); err != nil {
			return nil, err
		}
		b, err := getBuild(tx, id)
		if err != nil {
			return nil, err
		}
		if err := checkInterrupt(stop); err != nil {
			return nil, err
		}
		rv = append(rv, b)
	}
	return rv, nil
}

// insertBuild inserts the Build into the database.
func (d *localDB) insertBuild(tx *bolt.Tx, b *Build) error {
	// Insert the build into the various buckets.
	id := b.Id()
	b.fixup()

	// Builds.
	var serialized bytes.Buffer
	if err := gob.NewEncoder(&serialized).Encode(b); err != nil {
		return err
	}
	d.modify(b, serialized.Bytes())
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

// recordBuildIngestLatency pushes the latency between the time that the build
// started and the time when it was first ingested into metrics.
func recordBuildIngestLatency(b *Build) {
	// Measure the time between build start and first DB insertion.
	latency := time.Now().Sub(b.Started)
	if latency > INGEST_LATENCY_ALERT_THRESHOLD {
		// This is probably going to trigger an alert. Log the build for debugging.
		sklog.Warningf("Build start to ingestion latency is greater than %s (%s): %s %s #%d", INGEST_LATENCY_ALERT_THRESHOLD, latency, b.Master, b.Builder, b.Number)
	}
	metrics2.RawAddInt64PointAtTime("buildbot.ingest.latency", map[string]string{
		"master":  b.Master,
		"builder": b.Builder,
		"number":  strconv.Itoa(b.Number),
	}, int64(latency), time.Now())
}

// putBuild inserts the build into the database, replacing any previous version.
func (d *localDB) putBuild(tx *bolt.Tx, b *Build) error {
	id := b.Id()
	if tx.Bucket(BUCKET_BUILDS).Get(id) == nil {
		recordBuildIngestLatency(b)
	} else {
		if err := deleteBuild(tx, id); err != nil {
			return err
		}
	}
	return d.insertBuild(tx, b)
}

// putBuilds inserts the builds into the database, replacing any previous versions.
func (d *localDB) putBuilds(tx *bolt.Tx, builds []*Build) error {
	for _, b := range builds {
		id := b.Id()
		if tx.Bucket(BUCKET_BUILDS).Get(id) == nil {
			recordBuildIngestLatency(b)
		} else {
			if err := deleteBuild(tx, id); err != nil {
				return err
			}
		}
	}
	for _, b := range builds {
		if err := d.insertBuild(tx, b); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) PutBuild(b *Build) error {
	return d.update("PutBuild", func(tx *bolt.Tx) error {
		return d.putBuild(tx, b)
	})
}

// See documentation for DB interface.
func (d *localDB) PutBuilds(builds []*Build) error {
	return d.update("PutBuilds", func(tx *bolt.Tx) error {
		return d.putBuilds(tx, builds)
	})
}

func (d *localDB) getLastProcessedBuilds(m string, stop <-chan struct{}) ([]BuildID, error) {
	rv := []BuildID{}
	// Seek to the end of the bucket, grab the last build for each builder.
	if err := d.view("GetLastProcessedBuilds", func(tx *bolt.Tx) error {
		b := tx.Bucket(BUCKET_BUILDS)
		c := b.Cursor()
		k, _ := c.Last()
		if k == nil {
			// The bucket is empty.
			return nil
		}
		for k != nil {
			if err := checkInterrupt(stop); err != nil {
				return err
			}
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
func (d *localDB) GetLastProcessedBuilds(m string) ([]BuildID, error) {
	return d.getLastProcessedBuilds(m, make(chan struct{}))
}

func (d *localDB) getUnfinishedBuilds(master string, stop <-chan struct{}) ([]*Build, error) {
	prefix := []byte(fmt.Sprintf("%s|%s|", util.TimeUnixZero.Format(time.RFC3339Nano), master))
	var rv []*Build
	if err := d.view("GetUnfinishedBuilds", func(tx *bolt.Tx) error {
		ids := []BuildID{}
		cursor := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Cursor()
		for k, v := cursor.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = cursor.Next() {
			if err := checkInterrupt(stop); err != nil {
				return err
			}
			ids = append(ids, v)
		}
		builds, err := getBuilds(tx, ids, stop)
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
func (d *localDB) GetUnfinishedBuilds(master string) ([]*Build, error) {
	return d.getUnfinishedBuilds(master, make(chan struct{}))
}

func (d *localDB) getBuildsFromDateRange(start, end time.Time, stop <-chan struct{}) ([]*Build, error) {
	min := []byte(start.Format(time.RFC3339))
	max := []byte(end.Format(time.RFC3339))
	var rv []*Build
	if err := d.view("GetBuildsFromDateRange", func(tx *bolt.Tx) error {
		c := tx.Bucket(BUCKET_BUILDS_BY_FINISH_TIME).Cursor()
		ids := []BuildID{}
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			if err := checkInterrupt(stop); err != nil {
				return err
			}
			ids = append(ids, v)
		}
		builds, err := getBuilds(tx, ids, stop)
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
	return d.getBuildsFromDateRange(start, end, make(chan struct{}))
}

// See documentation for DB interface.
func (d *localDB) GetMaxBuildNumber(master, builder string) (int, error) {
	var rv int
	if err := d.view("GetMaxBuildNumber", func(tx *bolt.Tx) error {
		c := tx.Bucket(BUCKET_BUILDS).Cursor()
		maxID := MakeBuildID(master, builder, -1)
		_, _ = c.Seek(maxID)
		k, _ := c.Prev()
		if k == nil {
			rv = -1
			return nil
		}
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
func (d *localDB) GetModifiedBuilds(id string) ([]*Build, error) {
	gobs, err := d.GetModifiedBuildsGOB(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*Build, 0, len(gobs))
	for _, serialized := range gobs {
		var b Build
		if err := gob.NewDecoder(bytes.NewBuffer(serialized)).Decode(&b); err != nil {
			return nil, err
		}
		rv = append(rv, &b)
	}
	return rv, nil
}

// Like GetModifiedBuilds, but returns the GOB of each build.
func (d *localDB) GetModifiedBuildsGOB(id string) ([][]byte, error) {
	d.modMutex.Lock()
	defer d.modMutex.Unlock()
	modifiedBuilds, ok := d.modBuilds[id]
	if !ok {
		return nil, fmt.Errorf("Unknown or expired ID: %s", id)
	}
	rv := make([][]byte, 0, len(modifiedBuilds))
	for _, b := range modifiedBuilds {
		rv = append(rv, b)
	}
	d.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return rv, nil
}

// See documentation for DB interface.
func (d *localDB) StartTrackingModifiedBuilds() (string, error) {
	d.modMutex.Lock()
	defer d.modMutex.Unlock()
	if len(d.modBuilds) > MAX_MODIFIED_BUILDS_USERS {
		return "", fmt.Errorf("Exceeded maximum modified builds users.")
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	d.modBuilds[id] = map[string][]byte{}
	d.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return id, nil
}

func (d *localDB) clearExpiredModifiedUsers() {
	d.modMutex.Lock()
	defer d.modMutex.Unlock()
	for id, t := range d.modExpire {
		if time.Now().After(t) {
			delete(d.modBuilds, id)
			delete(d.modExpire, id)
		}
	}
}

func (d *localDB) modify(b *Build, gob []byte) {
	// Copy to allow the original buffer to be GC'd.
	gob = append([]byte{}, gob...)
	d.modMutex.Lock()
	defer d.modMutex.Unlock()
	for _, modBuilds := range d.modBuilds {
		modBuilds[string(b.Id())] = gob
	}
}

// See documentation for DB interface.
func (d *localDB) NumIngestedBuilds() (int, error) {
	var n int
	if err := d.view("NumIngestedBuilds", func(tx *bolt.Tx) error {
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
	if err := d.update("PutBuildComment", func(tx *bolt.Tx) error {
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
		return d.putBuild(tx, build)
	}); err != nil {
		c.Id = 0
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) DeleteBuildComment(master, builder string, number int, id int64) error {
	return d.update("DeleteBuildComment", func(tx *bolt.Tx) error {
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
		return d.putBuild(tx, build)
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
	prefix := []byte(fmt.Sprintf("%s|", builder))
	for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
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
	if err := d.view("GetBuilderComments", func(tx *bolt.Tx) error {
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
	if err := d.view("GetBuildersComments", func(tx *bolt.Tx) error {
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
	if err := d.update("PutBuilderComment", func(tx *bolt.Tx) error {
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
	return d.update("DeleteBuilderComment", func(tx *bolt.Tx) error {
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
	if err := d.view("GetCommitComments", func(tx *bolt.Tx) error {
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
	if err := d.view("GetCommitsComments", func(tx *bolt.Tx) error {
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
	if err := d.update("PutCommitComment", func(tx *bolt.Tx) error {
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
	return d.update("DeleteCommitComment", func(tx *bolt.Tx) error {
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
		if err := local.view("Backup", func(tx *bolt.Tx) error {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\"buildbot.db\"")
			w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(w)
			return err
		}); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed to create DB backup: %s", err))
		}
	})
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	return http.ListenAndServe(port, nil)
}
