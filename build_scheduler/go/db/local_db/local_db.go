package local_db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	// Tasks. Key is Task.Id, which is set to (creation time, sequence number)
	// (see formatId for detail), value is the GOB of the task. Tasks will be
	// updated in place. All repos share the same bucket.
	// TODO(benjaminwagner): May need to prefix value with metadata.
	BUCKET_TASKS = "tasks"
	// BUCKET_TASKS will be append-mostly, so use a high fill percent.
	BUCKET_TASKS_FILL_PERCENT = 0.9

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

	modTasks db.ModifiedTasks
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
	go func() {
		for _ = range time.Tick(time.Minute) {
			d.reportActiveTx()
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

	return d, nil
}

// See docs for DB interface.
func (d *localDB) Close() error {
	d.txMutex.Lock()
	defer d.txMutex.Unlock()
	if len(d.txActive) > 0 {
		return fmt.Errorf("Can not close DB when transactions are active.")
	}
	// TODO(benjaminwagner): Make this work.
	//if err := d.txCount.Delete(); err != nil {
	//	return err
	//}
	d.txActive = map[int64]string{}
	return d.db.Close()
}

// Sets t.Id either based on t.Created or now. tx must be an update transaction.
func (d *localDB) assignId(tx *bolt.Tx, t *db.Task, now time.Time) error {
	if t.Id != "" {
		return fmt.Errorf("Task Id already assigned: %v", t.Id)
	}
	ts := now
	if !util.TimeIsZero(t.Created) {
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
		serialized := tasksBucket(tx).Get([]byte(id))
		if serialized == nil {
			return nil
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
// TODO(benjaminwagner): Filter Tasks based on Task.Created rather than Task.Id.
func (d *localDB) GetTasksFromDateRange(start, end time.Time) ([]*db.Task, error) {
	min := []byte(start.UTC().Format(TIMESTAMP_FORMAT))
	max := []byte(end.UTC().Format(TIMESTAMP_FORMAT))
	decoder := db.TaskDecoder{}
	if err := d.view("GetTasksFromDateRange", func(tx *bolt.Tx) error {
		c := tasksBucket(tx).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			cpy := make([]byte, len(v))
			copy(cpy, v)
			if !decoder.Process(cpy) {
				return nil
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return decoder.Result()
}

// See documentation for DB interface.
func (d *localDB) PutTask(t *db.Task) error {
	return d.PutTasks([]*db.Task{t})
}

// validate returns an error if the task can not be inserted into the DB. Does
// not modify t.
func (d *localDB) validate(t *db.Task) error {
	// TODO(benjaminwagner): Check skew between t.Id (if assigned) and t.Created.
	return nil
}

// See documentation for DB interface.
// TODO(benjaminwagner): Figure out how to detect write/write conflicts and
// return "concurrent modification" error.
func (d *localDB) PutTasks(tasks []*db.Task) error {
	// If there is an error during the transaction, we should leave the tasks
	// unchanged. Save the old Ids since we set them below.
	oldIds := make([]string, len(tasks))
	// Validate and save current Ids.
	for _, t := range tasks {
		if err := d.validate(t); err != nil {
			return err
		}
		oldIds = append(oldIds, t.Id)
	}
	revertChanges := func() {
		for i, oldId := range oldIds {
			tasks[i].Id = oldId
		}
	}
	err := d.update("PutTasks", func(tx *bolt.Tx) error {
		// Assign Ids and encode.
		e := db.TaskEncoder{}
		now := time.Now()
		for _, t := range tasks {
			if t.Id == "" {
				if err := d.assignId(tx, t, now); err != nil {
					return err
				}
			}
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
			if err := tasksBucket(tx).Put([]byte(t.Id), serialized); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		revertChanges()
		return err
	} else {
		// TODO(benjaminwagner): pass serialized bytes.
		d.modTasks.TrackModifiedTasks(tasks)
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

// Returns the total number of tasks in the DB.
// TODO(benjaminwagner): add a metrics goroutine.
func (d *localDB) NumTasks() (int, error) {
	var n int
	if err := d.view("NumTasks", func(tx *bolt.Tx) error {
		n = tasksBucket(tx).Stats().KeyN
		return nil
	}); err != nil {
		return -1, err
	}
	return n, nil
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
