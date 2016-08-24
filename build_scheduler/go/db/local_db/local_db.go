package local_db

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	// BUCKET_TASKS is the name of the Tasks bucket. Key is Task.Id, which is set
	// to (creation time, sequence number) (see formatId for detail), value is
	// described in docs for BUCKET_TASKS_VERSION. Tasks will be updated in place.
	// All repos share the same bucket.
	BUCKET_TASKS = "tasks"
	// BUCKET_TASKS_FILL_PERCENT is the value to set for bolt.Bucket.FillPercent
	// for BUCKET_TASKS. BUCKET_TASKS will be append-mostly, so use a high fill
	// percent.
	BUCKET_TASKS_FILL_PERCENT = 0.9
	// BUCKET_TASKS_VERSION indicates the format of the value of BUCKET_TASKS
	// written by PutTasks. Retrieving Tasks from the DB must support all previous
	// versions. For all versions, the first byte is the version number.
	//   Version 1: v[0] = 1; v[1:9] is the modified time as UnixNano encoded as
	//     big endian; v[9:] is the GOB of the Task.
	BUCKET_TASKS_VERSION = 1

	// TIMESTAMP_FORMAT is a format string passed to Time.Format and time.Parse to
	// format/parse the timestamp in the Task ID. It is similar to
	// util.RFC3339NanoZeroPad, but since Task.Id can not contain colons, we omit
	// most of the punctuation. This timestamp can only be used to format and
	// parse times in UTC.
	TIMESTAMP_FORMAT = "20060102T150405.000000000Z"
	// SEQUENCE_NUMBER_FORMAT is a format string passed to fmt.Sprintf or
	// fmt.Sscanf to format/parse the sequence number in the Task ID. It is a
	// 16-digit zero-padded lowercase hexidecimal number.
	SEQUENCE_NUMBER_FORMAT = "%016x"

	// MAX_CREATED_TIME_SKEW is the maximum difference between the timestamp in a
	// Task's Id field and that Task's Created field. This allows AssignId to be
	// called before creating the Swarming task so that the Id can be included in
	// the Swarming task tags. GetTasksFromDateRange accounts for this skew when
	// retrieving tasks. This value can be increased in the future, but can never
	// be decreased.
	//
	// 6 minutes is based on httputils.DIAL_TIMEOUT + httputils.REQUEST_TIMEOUT,
	// which is assumed to be the approximate maximum duration of a successful
	// swarming.ApiClient.TriggerTask() call.
	MAX_CREATED_TIME_SKEW = 6 * time.Minute
)

// formatId returns the timestamp and sequence number formatted for a Task ID.
// Format is "<timestamp>_<sequence_num>", where the timestamp is formatted
// using TIMESTAMP_FORMAT and sequence_num is formatted using
// SEQUENCE_NUMBER_FORMAT.
func formatId(t time.Time, seq uint64) string {
	t = t.UTC()
	return fmt.Sprintf("%s_"+SEQUENCE_NUMBER_FORMAT, t.Format(TIMESTAMP_FORMAT), seq)
}

// parseId returns the timestamp and sequence number stored in a Task ID.
func parseId(id string) (time.Time, uint64, error) {
	parts := strings.Split(id, "_")
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("Unparsable ID: %q", id)
	}
	t, err := time.Parse(TIMESTAMP_FORMAT, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("Unparsable ID: %q; %s", id, err)
	}
	var seq uint64
	// Add newlines to force Sscanf to match the entire string. Otherwise
	// "123hello" will be parsed as 123. Note that Sscanf does not require 16
	// digits even though SEQUENCE_NUMBER_FORMAT specifies padding to 16 digits.
	i, err := fmt.Sscanf(parts[1]+"\n", SEQUENCE_NUMBER_FORMAT+"\n", &seq)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("Unparsable ID: %q; %s", id, err)
	} else if i != 1 {
		return time.Time{}, 0, fmt.Errorf("Unparsable ID: %q; Expected one hex number in %s, got %d", id, parts[1], i)
	}
	return t, seq, nil
}

// packV1 creates a value as described for BUCKET_TASKS_VERSION = 1. t is the
// modified time and serialized is the GOB of the Task.
func packV1(t time.Time, serialized []byte) []byte {
	rv := make([]byte, len(serialized)+9)
	rv[0] = 1
	binary.BigEndian.PutUint64(rv[1:9], uint64(t.UnixNano()))
	copy(rv[9:], serialized)
	return rv
}

// unpackV1 gets the modified time and GOB of the Task from a value as described
// by BUCKET_TASKS_VERSION = 1. The returned GOB shares structure with value.
func unpackV1(value []byte) (time.Time, []byte, error) {
	if len(value) < 9 {
		return time.Time{}, nil, fmt.Errorf("unpackV1 value is too short (%d bytes)", len(value))
	}
	if value[0] != 1 {
		return time.Time{}, nil, fmt.Errorf("unpackV1 called for value with version %d", value[0])
	}
	t := time.Unix(0, int64(binary.BigEndian.Uint64(value[1:9]))).UTC()
	return t, value[9:], nil
}

// localDB accesses a local BoltDB database containing tasks.
type localDB struct {
	// name is used in logging and metrics to identify this DB.
	name string

	// db is the underlying BoltDB.
	db *bolt.DB

	// tx fields contain metrics on the number of active transactions. Protected
	// by txMutex.
	txCount  *metrics2.Counter
	txNextId int64
	txActive map[int64]string
	txMutex  sync.RWMutex

	dbMetric *boltutil.DbMetric

	modTasks db.ModifiedTasks

	// Close will send on each of these channels to indicate goroutines should
	// stop.
	notifyOnClose []chan bool
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
		glog.Infof("%s Active Transactions: (none)", d.name)
		return
	}
	txs := make([]string, 0, len(d.txActive))
	for id, name := range d.txActive {
		txs = append(txs, fmt.Sprintf("  %d\t%s", id, name))
	}
	glog.Infof("%s Active Transactions:\n%s", d.name, strings.Join(txs, "\n"))
}

// tx is a wrapper for a BoltDB transaction which tracks statistics.
func (d *localDB) tx(name string, fn func(*bolt.Tx) error, update bool) error {
	txId := d.startTx(name)
	defer d.endTx(txId)
	defer metrics2.NewTimer("db-tx-duration", map[string]string{
		"database":    d.name,
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

// Returns the tasks bucket with FillPercent set.
func tasksBucket(tx *bolt.Tx) *bolt.Bucket {
	b := tx.Bucket([]byte(BUCKET_TASKS))
	b.FillPercent = BUCKET_TASKS_FILL_PERCENT
	return b
}

// NewDB returns a local DB instance.
func NewDB(name, filename string) (db.DB, error) {
	boltdb, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}
	d := &localDB{
		name: name,
		db:   boltdb,
		txCount: metrics2.GetCounter("db-active-tx", map[string]string{
			"database": name,
		}),
		txNextId: 0,
		txActive: map[int64]string{},
	}

	stopReportActiveTx := make(chan bool)
	d.notifyOnClose = append(d.notifyOnClose, stopReportActiveTx)
	go func() {
		t := time.NewTicker(time.Minute)
		for {
			select {
			case <-stopReportActiveTx:
				t.Stop()
				return
			case <-t.C:
				d.reportActiveTx()
			}
		}
	}()

	if err := d.update("NewDB", func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_TASKS)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if dbMetric, err := boltutil.NewDbMetric(boltdb, []string{BUCKET_TASKS}, map[string]string{"database": name}); err != nil {
		return nil, err
	} else {
		d.dbMetric = dbMetric
	}

	return d, nil
}

// See docs for DB interface.
func (d *localDB) Close() error {
	d.txMutex.Lock()
	defer d.txMutex.Unlock()
	if len(d.txActive) > 0 {
		return fmt.Errorf("Can not close DB when transactions are active.")
	}
	for _, c := range d.notifyOnClose {
		c <- true
	}
	d.txActive = map[int64]string{}
	if err := d.dbMetric.Delete(); err != nil {
		return err
	}
	d.dbMetric = nil
	if err := d.txCount.Delete(); err != nil {
		return err
	}
	d.txCount = nil
	return d.db.Close()
}

// Sets t.Id either based on t.Created or now. tx must be an update transaction.
func (d *localDB) assignId(tx *bolt.Tx, t *db.Task, now time.Time) error {
	if t.Id != "" {
		return fmt.Errorf("Task Id already assigned: %v", t.Id)
	}
	ts := now
	if !util.TimeIsZero(t.Created) {
		// TODO(benjaminwagner): Disallow assigning IDs based on t.Created; or
		// ensure t.Created is > any ID ts in the DB.
		ts = t.Created
	}
	seq, err := tasksBucket(tx).NextSequence()
	if err != nil {
		return err
	}
	t.Id = formatId(ts, seq)
	return nil
}

// See docs for DB interface.
func (d *localDB) AssignId(t *db.Task) error {
	oldId := t.Id
	err := d.update("AssignId", func(tx *bolt.Tx) error {
		return d.assignId(tx, t, time.Now())
	})
	if err != nil {
		t.Id = oldId
	}
	return err
}

// See docs for DB interface.
func (d *localDB) GetTaskById(id string) (*db.Task, error) {
	var rv *db.Task
	if err := d.view("GetTaskById", func(tx *bolt.Tx) error {
		value := tasksBucket(tx).Get([]byte(id))
		if value == nil {
			return nil
		}
		// Only BUCKET_TASKS_VERSION = 1 is implemented right now.
		// TODO(benjaminwagner): Add functions "pack" and "unpack" that determine
		// which version to use.
		_, serialized, err := unpackV1(value)
		if err != nil {
			return err
		}
		var t db.Task
		if err := gob.NewDecoder(bytes.NewReader(serialized)).Decode(&t); err != nil {
			return err
		}
		rv = &t
		return nil
	}); err != nil {
		return nil, err
	}
	if rv == nil {
		// Return an error if id is invalid.
		if _, _, err := parseId(id); err != nil {
			return nil, err
		}
	}
	return rv, nil
}

// See docs for DB interface.
func (d *localDB) GetTasksFromDateRange(start, end time.Time) ([]*db.Task, error) {
	min := []byte(start.Add(-MAX_CREATED_TIME_SKEW).UTC().Format(TIMESTAMP_FORMAT))
	max := []byte(end.UTC().Format(TIMESTAMP_FORMAT))
	decoder := db.TaskDecoder{}
	if err := d.view("GetTasksFromDateRange", func(tx *bolt.Tx) error {
		c := tasksBucket(tx).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			// Only BUCKET_TASKS_VERSION = 1 is implemented right now.
			_, serialized, err := unpackV1(v)
			if err != nil {
				return err
			}
			cpy := make([]byte, len(serialized))
			copy(cpy, serialized)
			if !decoder.Process(cpy) {
				return nil
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result, err := decoder.Result()
	if err != nil {
		return nil, err
	}
	sort.Sort(db.TaskSlice(result))
	// The Tasks retrieved based on Id timestamp may include Tasks with Created
	// time before/after the desired range.
	// TODO(benjaminwagner): Biased binary search might be faster.
	startIdx := 0
	for startIdx < len(result) && result[startIdx].Created.Before(start) {
		startIdx++
	}
	endIdx := len(result)
	for endIdx > 0 && !result[endIdx-1].Created.Before(end) {
		endIdx--
	}
	return result[startIdx:endIdx], nil
}

// See documentation for DB interface.
func (d *localDB) PutTask(t *db.Task) error {
	return d.PutTasks([]*db.Task{t})
}

// validate returns an error if the task can not be inserted into the DB. Does
// not modify t.
func (d *localDB) validate(t *db.Task) error {
	if util.TimeIsZero(t.Created) {
		return fmt.Errorf("Created not set. Task %s created time is %s. %v", t.Id, t.Created, t)
	}
	if t.Id != "" {
		idTs, _, err := parseId(t.Id)
		if err != nil {
			return err
		}
		if t.Created.Sub(idTs) > MAX_CREATED_TIME_SKEW {
			return fmt.Errorf("Created too late. Task %s was assigned Id at %s which is %s before Created time %s, more than MAX_CREATED_TIME_SKEW = %s.", t.Id, idTs, t.Created.Sub(idTs), t.Created, MAX_CREATED_TIME_SKEW)
		}
		if t.Created.Before(idTs) {
			return fmt.Errorf("Created too early. Task %s Created time was changed or set to %s after Id assigned at %s.", t.Id, t.Created, idTs)
		}
	}
	return nil
}

// See documentation for DB interface.
func (d *localDB) PutTasks(tasks []*db.Task) error {
	// If there is an error during the transaction, we should leave the tasks
	// unchanged. Save the old Ids and DbModified times since we set them below.
	type savedData struct {
		Id         string
		DbModified time.Time
	}
	oldData := make([]savedData, 0, len(tasks))
	// Validate and save current data.
	for _, t := range tasks {
		if err := d.validate(t); err != nil {
			return err
		}
		oldData = append(oldData, savedData{
			Id:         t.Id,
			DbModified: t.DbModified,
		})
	}
	revertChanges := func() {
		for i, data := range oldData {
			tasks[i].Id = data.Id
			tasks[i].DbModified = data.DbModified
		}
	}
	gobs := make(map[string][]byte, len(tasks))
	err := d.update("PutTasks", func(tx *bolt.Tx) error {
		bucket := tasksBucket(tx)
		// Assign Ids and encode.
		e := db.TaskEncoder{}
		now := time.Now().UTC()
		for _, t := range tasks {
			if t.Id == "" {
				if err := d.assignId(tx, t, now); err != nil {
					return err
				}
			} else {
				if value := bucket.Get([]byte(t.Id)); value != nil {
					modTs, serialized, err := unpackV1(value)
					if err != nil {
						return err
					}
					if !modTs.Equal(t.DbModified) {
						var existing db.Task
						if err := gob.NewDecoder(bytes.NewReader(serialized)).Decode(&existing); err != nil {
							return err
						}
						glog.Warningf("Cached Task has been modified in the DB. Current:\n%#v\nCached:\n%#v", existing, t)
						return db.ErrConcurrentUpdate
					}
				}
			}
			t.DbModified = now
			e.Process(t)
		}
		// Insert/update.
		for {
			t, serialized, err := e.Next()
			if err != nil {
				return err
			}
			if t == nil {
				break
			}
			gobs[t.Id] = serialized
			// BUCKET_TASKS_VERSION = 1
			value := packV1(now, serialized)
			if err := bucket.Put([]byte(t.Id), value); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		revertChanges()
		return err
	} else {
		d.modTasks.TrackModifiedTasksGOB(gobs)
	}
	return nil
}

// See docs for DB interface.
func (d *localDB) GetModifiedTasks(id string) ([]*db.Task, error) {
	return d.modTasks.GetModifiedTasks(id)
}

// See docs for DB interface.
func (d *localDB) StartTrackingModifiedTasks() (string, error) {
	return d.modTasks.StartTrackingModifiedTasks()
}

// See docs for DB interface.
func (d *localDB) StopTrackingModifiedTasks(id string) {
	d.modTasks.StopTrackingModifiedTasks(id)
}

// RunBackupServer runs an HTTP server which provides downloadable database
// backups.
func (d *localDB) RunBackupServer(port string) error {
	r := mux.NewRouter()
	r.HandleFunc("/backup", func(w http.ResponseWriter, r *http.Request) {
		if err := d.view("Backup", func(tx *bolt.Tx) error {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\"tasks.db\"")
			w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(w)
			return err
		}); err != nil {
			glog.Errorf("Failed to create DB backup: %s", err)
			httputils.ReportError(w, r, err, "Failed to create DB backup")
		}
	})
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	return http.ListenAndServe(port, nil)
}
