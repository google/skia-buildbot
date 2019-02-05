package local_db

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/db/modified"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// DB_NAME is the name of the database.
	DB_NAME = "task_scheduler_db"

	// DB_FILENAME is the name of the file in which the database is stored.
	DB_FILENAME = "task_scheduler.bdb"

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

	// BUCKET_JOBS is the name of the Jobs bucket. Key is Job.Id, which is set to
	// (creation time, sequence number) (see formatId for detail), value is
	// described in docs for BUCKET_JOBS_VERSION. Jobs will be updated in place.
	// All repos share the same bucket.
	BUCKET_JOBS = "jobs"
	// BUCKET_JOBS_FILL_PERCENT is the value to set for bolt.Bucket.FillPercent
	// for BUCKET_JOBS. BUCKET_JOBS will be append-mostly, so use a high fill
	// percent.
	BUCKET_JOBS_FILL_PERCENT = 0.9
	// BUCKET_JOBS_VERSION indicates the format of the value of BUCKET_JOBS
	// written by PutJobs. Retrieving Jobs from the DB must support all previous
	// versions. For all versions, the first byte is the version number.
	//   Version 1: v[0] = 1; v[1:9] is the modified time as UnixNano encoded as
	//     big endian; v[9:] is the GOB of the Job.
	BUCKET_JOBS_VERSION = 1

	// BUCKET_COMMENTS is the name of the comments bucket. Key is KEY_COMMENT_MAP,
	// value is the GOB of the map provided by db.CommentBox. The comment map will
	// be updated in place. All repos share the same bucket.
	BUCKET_COMMENTS = "comments"
	KEY_COMMENT_MAP = "comment-map"

	// BUCKET_BACKUP is the name of the backup bucket. Key is
	// KEY_INCREMENTAL_BACKUP_TIME, value is time.Time.MarshalBinary. The value
	// will be updated in place.
	BUCKET_BACKUP               = "backup"
	KEY_INCREMENTAL_BACKUP_TIME = "inc-backup-ts"

	// TIMESTAMP_FORMAT is a format string passed to Time.Format and time.Parse to
	// format/parse the timestamp in the Task ID. It is similar to
	// util.RFC3339NanoZeroPad, but since Task.Id can not contain colons, we omit
	// most of the punctuation. This timestamp can only be used to format and
	// parse times in UTC.
	TIMESTAMP_FORMAT = util.SAFE_TIMESTAMP_FORMAT
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

// formatId returns the timestamp and sequence number formatted for a Task or
// Job ID. Format is "<timestamp>_<sequence_num>", where the timestamp is
// formatted using TIMESTAMP_FORMAT and sequence_num is formatted using
// SEQUENCE_NUMBER_FORMAT.
func formatId(t time.Time, seq uint64) string {
	t = t.UTC()
	return fmt.Sprintf("%s_"+SEQUENCE_NUMBER_FORMAT, t.Format(TIMESTAMP_FORMAT), seq)
}

// ParseId returns the timestamp and sequence number stored in a Task or Job ID.
func ParseId(id string) (time.Time, uint64, error) {
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

// packV1 creates a value as described for BUCKET_TASKS_VERSION = 1 or
// BUCKET_JOBS_VERSION = 1. t is the modified time and serialized is the GOB of
// the Task or Job.
func packV1(t time.Time, serialized []byte) []byte {
	rv := make([]byte, len(serialized)+9)
	rv[0] = 1
	binary.BigEndian.PutUint64(rv[1:9], uint64(t.UnixNano()))
	copy(rv[9:], serialized)
	return rv
}

// unpackV1 gets the modified time and GOB of the Task/Job from a value as
// described by BUCKET_TASKS_VERSION = 1 or BUCKET_JOBS_VERSION = 1. The
// returned GOB shares structure with value.
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

// packTask creates a value for the current value of BUCKET_TASKS_VERSION. t is
// the modified time and serialized is the GOB of the Task.
func packTask(t time.Time, serialized []byte) []byte {
	if BUCKET_TASKS_VERSION != 1 {
		panic(BUCKET_TASKS_VERSION)
	}
	return packV1(t, serialized)
}

// unpackTask gets the modified time and GOB of the Task from a value for any
// supported version. The returned GOB shares structure with value.
func unpackTask(value []byte) (time.Time, []byte, error) {
	if len(value) < 1 {
		return time.Time{}, nil, fmt.Errorf("unpackTask value is empty")
	}
	// Only one version currently supported.
	if value[0] != 1 {
		return time.Time{}, nil, fmt.Errorf("unpackTask unrecognized version %d", value[0])
	}
	return unpackV1(value)
}

// packJob creates a value for the current value of BUCKET_JOBS_VERSION. t is
// the modified time and serialized is the GOB of the Job.
func packJob(t time.Time, serialized []byte) []byte {
	if BUCKET_JOBS_VERSION != 1 {
		panic(BUCKET_JOBS_VERSION)
	}
	return packV1(t, serialized)
}

// unpackJob gets the modified time and GOB of the Job from a value for any
// supported version. The returned GOB shares structure with value.
func unpackJob(value []byte) (time.Time, []byte, error) {
	if len(value) < 1 {
		return time.Time{}, nil, fmt.Errorf("unpackJob value is empty")
	}
	// Only one version currently supported.
	if value[0] != 1 {
		return time.Time{}, nil, fmt.Errorf("unpackJob unrecognized version %d", value[0])
	}
	return unpackV1(value)
}

// localDB accesses a local BoltDB database containing tasks, jobs, and
// comments.
type localDB struct {
	// name is used in logging and metrics to identify this DB.
	name string
	// filename is used when serving the database backup file.
	filename string

	// db is the underlying BoltDB.
	db *bolt.DB

	// tx fields contain metrics on the number of active transactions. Protected
	// by txMutex.
	txCount  metrics2.Counter
	txNextId int64
	txActive map[int64]string
	txMutex  sync.RWMutex

	// Count queries and results to get QPS metrics.
	metricReadTaskQueries  metrics2.Counter
	metricReadTaskRows     metrics2.Counter
	metricWriteTaskQueries metrics2.Counter
	metricWriteTaskRows    metrics2.Counter
	metricReadJobQueries   metrics2.Counter
	metricReadJobRows      metrics2.Counter
	metricWriteJobQueries  metrics2.Counter
	metricWriteJobRows     metrics2.Counter

	// ModifiedTasksImpl and ModifiedJobsImpl are embedded in order to
	// implement db.ModifiedTasksReader and db.ModifiedJobsReader.
	db.ModifiedData

	// CommentBox is embedded in order to implement db.CommentDB. CommentBox uses
	// this localDB to persist the comments.
	*memory.CommentBox

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
		sklog.Infof("%s Active Transactions: (none)", d.name)
		return
	}
	txs := make([]string, 0, len(d.txActive))
	for id, name := range d.txActive {
		txs = append(txs, fmt.Sprintf("  %d\t%s", id, name))
	}
	sklog.Infof("%s Active Transactions:\n%s", d.name, strings.Join(txs, "\n"))
}

// tx is a wrapper for a BoltDB transaction which tracks statistics.
func (d *localDB) tx(name string, fn func(*bolt.Tx) error, update bool) error {
	txId := d.startTx(name)
	defer d.endTx(txId)
	defer metrics2.NewTimer("db_tx_duration", map[string]string{
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

// Returns the jobs bucket with FillPercent set.
func jobsBucket(tx *bolt.Tx) *bolt.Bucket {
	b := tx.Bucket([]byte(BUCKET_JOBS))
	b.FillPercent = BUCKET_JOBS_FILL_PERCENT
	return b
}

// NewDB returns a local DB instance.
func NewDB(name, filename string, mod db.ModifiedData) (db.BackupDBCloser, error) {
	boltdb, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}
	if mod == nil {
		mod = modified.NewModifiedData()
	}
	d := &localDB{
		name:     name,
		filename: path.Base(filename),
		db:       boltdb,
		txCount: metrics2.GetCounter("db_active_tx", map[string]string{
			"database": name,
		}),
		txNextId: 0,
		txActive: map[int64]string{},
		metricReadTaskQueries: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "read",
			"bucket":   BUCKET_TASKS,
			"count":    "queries",
		}),
		metricReadTaskRows: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "read",
			"bucket":   BUCKET_TASKS,
			"count":    "rows",
		}),
		metricWriteTaskQueries: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "write",
			"bucket":   BUCKET_TASKS,
			"count":    "queries",
		}),
		metricWriteTaskRows: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "write",
			"bucket":   BUCKET_TASKS,
			"count":    "rows",
		}),
		metricReadJobQueries: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "read",
			"bucket":   BUCKET_JOBS,
			"count":    "queries",
		}),
		metricReadJobRows: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "read",
			"bucket":   BUCKET_JOBS,
			"count":    "rows",
		}),
		metricWriteJobQueries: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "write",
			"bucket":   BUCKET_JOBS,
			"count":    "queries",
		}),
		metricWriteJobRows: metrics2.GetCounter("db_op_count", map[string]string{
			"database": name,
			"op":       "write",
			"bucket":   BUCKET_JOBS,
			"count":    "rows",
		}),
		ModifiedData: mod,
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

	comments := map[string]*types.RepoComments{}

	if err := d.update("NewDB", func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_TASKS)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_JOBS)); err != nil {
			return err
		}
		commentsBucket, err := tx.CreateBucketIfNotExists([]byte(BUCKET_COMMENTS))
		if err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_BACKUP)); err != nil {
			return err
		}

		serializedCommentMap := commentsBucket.Get([]byte(KEY_COMMENT_MAP))
		if serializedCommentMap != nil {
			if err := gob.NewDecoder(bytes.NewReader(serializedCommentMap)).Decode(&comments); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	d.CommentBox = memory.NewCommentBoxWithPersistence(mod, comments, d.writeCommentsMap)

	return d, nil
}

// See docs for io.Closer interface.
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
	if err := d.txCount.Delete(); err != nil {
		return err
	}
	d.txCount = nil
	return d.db.Close()
}

// Sets t.Id either based on t.Created or now. tx must be an update transaction.
func (d *localDB) assignTaskId(tx *bolt.Tx, t *types.Task, now time.Time) error {
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

// See docs for TaskDB interface.
func (d *localDB) AssignId(t *types.Task) error {
	oldId := t.Id
	err := d.update("AssignId", func(tx *bolt.Tx) error {
		return d.assignTaskId(tx, t, time.Now())
	})
	if err != nil {
		t.Id = oldId
	}
	return err
}

// See docs for TaskDB interface.
func (d *localDB) GetTaskById(id string) (*types.Task, error) {
	d.metricReadTaskQueries.Inc(1)
	var rv *types.Task
	if err := d.view("GetTaskById", func(tx *bolt.Tx) error {
		value := tasksBucket(tx).Get([]byte(id))
		if value == nil {
			return nil
		}
		_, serialized, err := unpackTask(value)
		if err != nil {
			return err
		}
		var t types.Task
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
		if _, _, err := ParseId(id); err != nil {
			return nil, err
		}
	}
	d.metricReadTaskRows.Inc(1)
	return rv, nil
}

// See docs for TaskDB interface.
func (d *localDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*types.Task, error) {
	d.metricReadTaskQueries.Inc(1)
	min := []byte(start.Add(-MAX_CREATED_TIME_SKEW).UTC().Format(TIMESTAMP_FORMAT))
	max := []byte(end.Add(MAX_CREATED_TIME_SKEW).UTC().Format(TIMESTAMP_FORMAT))
	decoder := types.NewTaskDecoder()
	if err := d.view("GetTasksFromDateRange", func(tx *bolt.Tx) error {
		c := tasksBucket(tx).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			_, serialized, err := unpackTask(v)
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
	sort.Sort(types.TaskSlice(result))
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
	if repo == "" {
		d.metricReadTaskRows.Inc(int64(endIdx - startIdx))
		return result[startIdx:endIdx], nil
	}
	rv := make([]*types.Task, 0, len(result[startIdx:endIdx]))
	for _, t := range result[startIdx:endIdx] {
		if t.Repo == repo {
			rv = append(rv, t)
		}
	}
	d.metricReadTaskRows.Inc(int64(len(rv)))
	return rv, nil
}

// See documentation for TaskDB interface.
func (d *localDB) PutTask(t *types.Task) error {
	return d.PutTasks([]*types.Task{t})
}

// validateTask returns an error if the task can not be inserted into the DB.
// Does not modify task.
func (d *localDB) validateTask(task *types.Task) error {
	if util.TimeIsZero(task.Created) {
		return fmt.Errorf("Created not set. Task %s created time is %s. %v", task.Id, task.Created, task)
	}
	if task.Id != "" {
		idTs, _, err := ParseId(task.Id)
		if err != nil {
			return err
		}
		if task.Created.Sub(idTs) > MAX_CREATED_TIME_SKEW {
			return fmt.Errorf("Created too late. Task %s was assigned Id at %s which is %s before Created time %s, more than MAX_CREATED_TIME_SKEW = %s.", task.Id, idTs, task.Created.Sub(idTs), task.Created, MAX_CREATED_TIME_SKEW)
		}
		if idTs.Sub(task.Created) > MAX_CREATED_TIME_SKEW {
			return fmt.Errorf("Created too early. Task %s Created time was changed or set to %s which is %s after Id assigned at %s, more than MAX_CREATED_TIME_SKEW = %s.", task.Id, task.Created, idTs.Sub(task.Created), idTs, MAX_CREATED_TIME_SKEW)
		}
	}
	return nil
}

// See documentation for TaskDB interface.
func (d *localDB) PutTasks(tasks []*types.Task) error {
	if len(tasks) > firestore.MAX_TRANSACTION_DOCS {
		sklog.Errorf("Inserting %d tasks, which is more than the Firestore maximum of %d; consider switching to PutTasksInChunks.", len(tasks), firestore.MAX_TRANSACTION_DOCS)
	}
	d.metricWriteTaskQueries.Inc(1)
	// If there is an error during the transaction, we should leave the tasks
	// unchanged. Save the old Ids and DbModified times since we set them below.
	type savedData struct {
		Id         string
		DbModified time.Time
	}
	oldData := make([]savedData, 0, len(tasks))
	// Validate and save current data.
	for _, t := range tasks {
		if err := d.validateTask(t); err != nil {
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
	var now time.Time
	gobs := make(map[string][]byte, len(tasks))
	err := d.update("PutTasks", func(tx *bolt.Tx) error {
		bucket := tasksBucket(tx)
		// Assign Ids and encode.
		e := types.TaskEncoder{}
		now = time.Now().UTC()
		for _, t := range tasks {
			if t.Id == "" {
				if err := d.assignTaskId(tx, t, now); err != nil {
					return err
				}
			} else {
				if value := bucket.Get([]byte(t.Id)); value != nil {
					modTs, _, err := unpackTask(value)
					if err != nil {
						return err
					}
					if !modTs.Equal(t.DbModified) {
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
			value := packTask(t.DbModified, serialized)
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
		d.metricWriteTaskRows.Inc(int64(len(gobs)))
		d.TrackModifiedTasksGOB(now, gobs)
	}
	return nil
}

// See documentation for TaskDB interface.
func (d *localDB) PutTasksInChunks(tasks []*types.Task) error {
	return util.ChunkIter(len(tasks), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutTasks(tasks[i:j])
	})
}

// Sets job.Id based on job.Created. tx must be an update transaction.
func (d *localDB) assignJobId(tx *bolt.Tx, job *types.Job) error {
	if job.Id != "" {
		return fmt.Errorf("Job Id already assigned: %v", job.Id)
	}
	if util.TimeIsZero(job.Created) {
		// TODO(benjaminwagner): Ensure job.Created is > any ID ts in the DB.
		return fmt.Errorf("Job Created time is not set: %s", job.Created)
	}
	seq, err := jobsBucket(tx).NextSequence()
	if err != nil {
		return err
	}
	job.Id = formatId(job.Created, seq)
	return nil
}

// See docs for JobDB interface.
func (d *localDB) GetJobById(id string) (*types.Job, error) {
	d.metricReadJobQueries.Inc(1)
	var rv *types.Job
	if err := d.view("GetJobById", func(tx *bolt.Tx) error {
		value := jobsBucket(tx).Get([]byte(id))
		if value == nil {
			return nil
		}
		_, serialized, err := unpackJob(value)
		if err != nil {
			return err
		}
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(serialized)).Decode(&job); err != nil {
			return err
		}
		rv = &job
		return nil
	}); err != nil {
		return nil, err
	}
	if rv == nil {
		// Return an error if id is invalid.
		if _, _, err := ParseId(id); err != nil {
			return nil, err
		}
	}
	d.metricReadJobRows.Inc(1)
	return rv, nil
}

// See docs for JobDB interface.
func (d *localDB) GetJobsFromDateRange(start, end time.Time) ([]*types.Job, error) {
	d.metricReadJobQueries.Inc(1)
	min := []byte(start.UTC().Format(TIMESTAMP_FORMAT))
	max := []byte(end.UTC().Format(TIMESTAMP_FORMAT))
	decoder := types.NewJobDecoder()
	if err := d.view("GetJobsFromDateRange", func(tx *bolt.Tx) error {
		c := jobsBucket(tx).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			_, serialized, err := unpackJob(v)
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
	sort.Sort(types.JobSlice(result))
	d.metricReadJobRows.Inc(int64(len(result)))
	return result, nil
}

// See documentation for JobDB interface.
func (d *localDB) PutJob(job *types.Job) error {
	return d.PutJobs([]*types.Job{job})
}

// validateJob returns an error if the job can not be inserted into the DB. Does not
// modify job.
func (d *localDB) validateJob(job *types.Job) error {
	if util.TimeIsZero(job.Created) {
		return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
	}
	if job.Id != "" {
		idTs, _, err := ParseId(job.Id)
		if err != nil {
			return err
		}
		if !idTs.Equal(job.Created) {
			return fmt.Errorf("Created time has changed since Job ID assigned. Job %s was assigned Id for Created time %s but Created time is now %s.", job.Id, idTs, job.Created)
		}
	}
	return nil
}

// See documentation for JobDB interface.
func (d *localDB) PutJobs(jobs []*types.Job) error {
	if len(jobs) > firestore.MAX_TRANSACTION_DOCS {
		sklog.Errorf("Inserting %d jobs, which is more than the Firestore maximum of %d; consider switching to PutJobsInChunks.", len(jobs), firestore.MAX_TRANSACTION_DOCS)
	}
	d.metricWriteJobQueries.Inc(1)
	// If there is an error during the transaction, we should leave the jobs
	// unchanged. Save the old Ids and DbModified times since we set them below.
	type savedData struct {
		Id         string
		DbModified time.Time
	}
	oldData := make([]savedData, len(jobs))
	// Validate and save current data.
	for i, job := range jobs {
		if err := d.validateJob(job); err != nil {
			return err
		}
		oldData[i].Id = job.Id
		oldData[i].DbModified = job.DbModified
	}
	revertChanges := func() {
		for i, data := range oldData {
			jobs[i].Id = data.Id
			jobs[i].DbModified = data.DbModified
		}
	}
	var now time.Time
	gobs := make(map[string][]byte, len(jobs))
	err := d.update("PutJobs", func(tx *bolt.Tx) error {
		bucket := jobsBucket(tx)
		// Assign Ids and encode.
		e := types.JobEncoder{}
		now = time.Now().UTC()
		for _, job := range jobs {
			if job.Id == "" {
				if err := d.assignJobId(tx, job); err != nil {
					return err
				}
			} else {
				if value := bucket.Get([]byte(job.Id)); value != nil {
					modTs, _, err := unpackJob(value)
					if err != nil {
						return err
					}
					if !modTs.Equal(job.DbModified) {
						return db.ErrConcurrentUpdate
					}
				}
			}
			job.DbModified = now
			e.Process(job)
		}
		// Insert/update.
		for {
			job, serialized, err := e.Next()
			if err != nil {
				return err
			}
			if job == nil {
				break
			}
			gobs[job.Id] = serialized
			value := packJob(job.DbModified, serialized)
			if err := bucket.Put([]byte(job.Id), value); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		revertChanges()
		return err
	} else {
		d.metricWriteJobRows.Inc(int64(len(gobs)))
		d.TrackModifiedJobsGOB(now, gobs)
	}
	return nil
}

// See documentation for JobDB interface.
func (d *localDB) PutJobsInChunks(jobs []*types.Job) error {
	return util.ChunkIter(len(jobs), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutJobs(jobs[i:j])
	})
}

// writeCommentsMap is passed to db.NewCommentBoxWithPersistence to persist
// comments after every change. Updates the value stored in BUCKET_COMMENTS.
func (d *localDB) writeCommentsMap(comments map[string]*types.RepoComments) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(comments); err != nil {
		return err
	}
	return d.update("writeCommentsMap", func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(BUCKET_COMMENTS)).Put([]byte(KEY_COMMENT_MAP), buf.Bytes())
	})
}

// See docs for BackupDBCloser interface.
func (d *localDB) WriteBackup(w io.Writer) error {
	return d.view("WriteBackup", func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// See docs for BackupDBCloser interface.
func (d *localDB) SetIncrementalBackupTime(t time.Time) error {
	t = t.UTC()
	val, err := t.MarshalBinary()
	if err != nil {
		return err
	}
	err = d.update("SetIncrementalBackupTime", func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(BUCKET_BACKUP)).Put([]byte(KEY_INCREMENTAL_BACKUP_TIME), val)
	})
	if err != nil {
		return err
	}
	return nil
}

// See docs for BackupDBCloser interface.
func (d *localDB) GetIncrementalBackupTime() (time.Time, error) {
	incBackupTime := time.Time{}
	err := d.view("GetIncrementalBackupTime", func(tx *bolt.Tx) error {
		commentsBucket := tx.Bucket([]byte(BUCKET_BACKUP))
		serializedIncBackupTime := commentsBucket.Get([]byte(KEY_INCREMENTAL_BACKUP_TIME))
		if serializedIncBackupTime == nil {
			return nil
		}
		return incBackupTime.UnmarshalBinary(serializedIncBackupTime)
	})
	return incBackupTime.UTC(), err
}
