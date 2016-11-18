// Implementation of backing up a DB to Google Cloud Storage (GS).
package recovery

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	// DB_BACKUP_DIR is the prefix of the object name to store DB backups in the
	// GS bucket.
	DB_BACKUP_DIR = "db-backup"
	// DB_FILE_NAME_EXTENSION is added to the base filename.
	DB_FILE_NAME_EXTENSION = "bdb"
	// TRIGGER_DIRNAME is the name of the directory containing files indicating
	// that an automatic backup should occur. These files are created by the
	// systemd task-scheduler-db-backup.service.
	TRIGGER_DIRNAME = "trigger-backup"
	// RETRY_COUNT is the number of times to attempt a DB backup when failures
	// occur.
	RETRY_COUNT = 3

	// JOB_BACKUP_DIR is the prefix of the object name to store incremental Job
	// backups in the GS bucket.
	JOB_BACKUP_DIR = "job-backup"
	// JOB_FILE_NAME_EXTENSION is added to the base filename.
	JOB_FILE_NAME_EXTENSION = "gob"
)

// DBBackup has methods to trigger periodic and immediate backups.
type DBBackup interface {
	// Tick triggers a periodic backup if one is due. This allows callers to
	// perform backups when it is less likely to conflict with other actions.
	Tick()
	// ImmediateBackup triggers a backup immediately.
	ImmediateBackup() error
	// RetrieveJobs returns all backed-up Jobs created or modified since the given
	// time, as a map[Job.Id]*Job. Only the most recent backup is returned.
	RetrieveJobs(since time.Time) (map[string]*db.Job, error)
}

// gsDBBackup implements DBBackup.
type gsDBBackup struct {
	// gsBucket specifies the GS bucket (for testing).
	gsBucket string
	// gsClient accesses gsBucket.
	gsClient *storage.Client
	// db is the DB to back up.
	db db.BackupDBCloser
	// ctx allows to stop the gsDBBackup.
	ctx context.Context
	// triggerDir is the directory that task-scheduler-db-backup.service will
	// write files to trigger an automatic backup.
	triggerDir string
	// modifiedJobsId is the return value of StartTrackingModifiedJobs.
	modifiedJobsId string
	// lastDBBackupLiveness records the modified time of the most recent DB
	// backup.
	lastDBBackupLiveness *metrics2.Liveness
	// recentDBBackupCount records the number of DB backups in the last 24 hours.
	recentDBBackupCount *metrics2.Int64Metric
	// maybeBackupDBLiveness records whether maybeBackupDB is being called.
	maybeBackupDBLiveness *metrics2.Liveness
	// jobBackupCount records the number of jobs backed up since the gsDBBackup
	// was created.
	jobBackupCount *metrics2.Counter
	// incrementalBackupLiveness tracks whether incrementalBackupStep is running
	// successfully.
	incrementalBackupLiveness *metrics2.Liveness
	// incrementalBackupResetCount records the number of times GetModifiedJobsGOB
	// returned ErrUnknownId since the last successful DB backup.
	incrementalBackupResetCount *metrics2.Counter
}

// NewDBBackup creates a DBBackup.
//  - ctx can be used to stop any background processes as well as to interrupt
//    Tick or ImmediateBackup.
//  - gsBucket is the GS bucket to store backups.
//  - db is the DB to back up.
//  - authClient is a client authenticated with auth.SCOPE_READ_WRITE.
func NewDBBackup(ctx context.Context, gsBucket string, db db.BackupDBCloser, name string, workdir string, authClient *http.Client) (DBBackup, error) {
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, err
	}
	b, err := newGsDbBackupWithClient(ctx, gsBucket, db, name, workdir, gsClient)
	if err != nil {
		return nil, err
	}

	go util.RepeatCtx(10*time.Minute, b.ctx, b.updateMetrics)
	go util.RepeatCtx(10*time.Second, b.ctx, func() {
		if err := b.incrementalBackupStep(time.Now()); err != nil {
			glog.Errorf("Incremental Job backup failed: %s", err)
		}
	})

	return b, nil
}

// newGsDbBackupWithClient is the same as NewDBBackup but takes a GS client for
// testing and does not start the metrics goroutine or the incremental backup
// goroutine.
func newGsDbBackupWithClient(ctx context.Context, gsBucket string, db db.BackupDBCloser, name string, workdir string, gsClient *storage.Client) (*gsDBBackup, error) {
	modJobsId, err := db.StartTrackingModifiedJobs()
	if err != nil {
		return nil, err
	}
	metricTags := map[string]string{
		"database": name,
	}
	b := &gsDBBackup{
		gsBucket:                    gsBucket,
		gsClient:                    gsClient,
		db:                          db,
		ctx:                         ctx,
		triggerDir:                  path.Join(workdir, TRIGGER_DIRNAME),
		modifiedJobsId:              modJobsId,
		lastDBBackupLiveness:        metrics2.NewLiveness("last-db-backup", metricTags),
		recentDBBackupCount:         metrics2.GetInt64Metric("recent-db-backup-count", metricTags),
		maybeBackupDBLiveness:       metrics2.NewLiveness("db-backup-maybe-backup-db", metricTags),
		jobBackupCount:              metrics2.GetCounter("incremental-job-backup", metricTags),
		incrementalBackupLiveness:   metrics2.NewLiveness("incremental-backup", metricTags),
		incrementalBackupResetCount: metrics2.GetCounter("incremental-backup-reset", metricTags),
	}
	// Release resources when done.
	go func() {
		<-ctx.Done()
		b.db.StopTrackingModifiedTasks(b.modifiedJobsId)
		// TODO(benjaminwagner): Liveness doesn't have a Delete method.
		//if err := b.lastDBBackupLiveness.Delete(); err != nil {
		//	glog.Error(err)
		//}
		if err := b.recentDBBackupCount.Delete(); err != nil {
			glog.Error(err)
		}
		// TODO(benjaminwagner): Liveness doesn't have a Delete method.
		//if err := b.maybeBackupDBLiveness.Delete(); err != nil {
		//	glog.Error(err)
		//}
		if err := b.jobBackupCount.Delete(); err != nil {
			glog.Error(err)
		}
		// TODO(benjaminwagner): Liveness doesn't have a Delete method.
		//if err := b.incrementalBackupLiveness.Delete(); err != nil {
		//	glog.Error(err)
		//}
		if err := b.incrementalBackupResetCount.Delete(); err != nil {
			glog.Error(err)
		}
	}()
	return b, nil
}

// getBackupMetrics returns the Updated time of the most recent DB backup and
// the number of backups in the last 24 hours, or the zero time if no backups
// exist. Does not return an error unless the request could not be completed.
func (b *gsDBBackup) getBackupMetrics(now time.Time) (time.Time, int64, error) {
	lastTime := time.Time{}
	var count int64 = 0
	countAfter := now.Add(-24 * time.Hour)
	err := gs.AllFilesInDir(b.gsClient, b.gsBucket, DB_BACKUP_DIR, func(item *storage.ObjectAttrs) {
		if item.Updated.After(lastTime) {
			lastTime = item.Updated
		}
		if item.Updated.After(countAfter) {
			count++
		}
	})
	return lastTime, count, err
}

// updateMetrics updates the metrics for the time since last successful backup
// and number of backups in the last 24 hours.
func (b *gsDBBackup) updateMetrics() {
	last, count, err := b.getBackupMetrics(time.Now())
	if err != nil {
		glog.Errorf("Failed to get DB backup metrics: %s", err)
	}
	b.lastDBBackupLiveness.ManualReset(last)
	glog.Infof("Last DB backup was %s.", last)
	b.recentDBBackupCount.Update(count)
}

// writeDBBackupToFile creates filename and writes the DB to it. File may be
// written even if an error is returned.
func (b *gsDBBackup) writeDBBackupToFile(filename string) error {
	fileW, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Could not create temp file to write DB backup: %s", err)
	}
	defer func() {
		// We set fileW to nil when we manually close it below.
		if fileW != nil {
			util.Close(fileW)
		}
	}()
	// TODO(benjaminwagner): Start WriteBackup in a goroutine, close fileW on
	// b.ctx.Done().
	if err := b.db.WriteBackup(fileW); err != nil {
		return err
	}
	err, fileW = fileW.Close(), nil
	return err
}

// uploadFile gzips and writes the given file as the given object name to GS.
func uploadFile(ctx context.Context, filename string, bucket *storage.BucketHandle, objectname string, modTime time.Time) (err error) {
	fileR, openErr := os.Open(filename)
	if openErr != nil {
		return fmt.Errorf("Unable to read temporary backup file: %s", err)
	}
	// If we are able to successfully read temp file until EOF, we don't
	// care if Close returns an error.
	defer util.Close(fileR)
	return upload(ctx, fileR, bucket, objectname, modTime)
}

// upload gzips and writes the given content as the given object name to GS.
func upload(ctx context.Context, content io.Reader, bucket *storage.BucketHandle, objectname string, modTime time.Time) (err error) {
	objW := bucket.Object(objectname).NewWriter(ctx)
	defer func() {
		// We set objW to nil when we manually close it below.
		if objW != nil {
			_ = objW.CloseWithError(err)
		}
	}()
	basename := path.Base(objectname)
	objW.ObjectAttrs.ContentType = "application/octet-stream"
	objW.ObjectAttrs.ContentDisposition = fmt.Sprintf("attachment; filename=\"%s\"", basename)
	objW.ObjectAttrs.ContentEncoding = "gzip"
	gzW := gzip.NewWriter(objW)
	defer func() {
		// We set gzW to nil when we manually close it below.
		if gzW != nil {
			util.Close(gzW)
		}
	}()
	gzW.Header.Name = basename
	gzW.Header.ModTime = modTime.UTC()
	if _, err = io.Copy(gzW, content); err != nil {
		return err
	}
	if err, gzW = gzW.Close(), nil; err != nil {
		return err
	}
	err, objW = objW.Close(), nil
	return err
}

// backupDB performs an immediate backup of b.db, using the given name as the
// base filename.
func (b *gsDBBackup) backupDB(now time.Time, basename string) (err error) {
	// We expect TMPDIR to be set to a location that can store a large file at
	// high throughput.
	tempdir, err := ioutil.TempDir("", "dbbackup")
	if err != nil {
		return err
	}
	defer util.RemoveAll(tempdir)
	tempfilename := path.Join(tempdir, fmt.Sprintf("%s.%s", basename, DB_FILE_NAME_EXTENSION))

	modTime, err := b.db.GetIncrementalBackupTime()
	if err != nil {
		glog.Warningf("Error getting DB incremental backup time; using current time instead. %s", err)
		modTime = now
	}
	if err := b.writeDBBackupToFile(tempfilename); err != nil {
		return err
	}
	bucket := b.gsClient.Bucket(b.gsBucket)
	objectname := fmt.Sprintf("%s/%s/%s.%s", DB_BACKUP_DIR, now.UTC().Format("2006/01/02"), basename, DB_FILE_NAME_EXTENSION)
	if err := uploadFile(b.ctx, tempfilename, bucket, objectname, modTime); err != nil {
		return err
	}
	b.incrementalBackupResetCount.Reset()
	return nil
}

// immediateBackupBasename creates a base filename for backupDB that is unlikely
// to conflict with other backups.
func immediateBackupBasename(now time.Time) string {
	return "task-scheduler-" + now.UTC().Format("15:04:05")
}

// See documentation for DBBackup.ImmediateBackup.
func (b *gsDBBackup) ImmediateBackup() error {
	glog.Infof("Beginning manual DB backup.")
	now := time.Now()
	return b.backupDB(now, immediateBackupBasename(now))
}

// findAndParseTriggerFile returns the base filename for the first file in
// triggerDir and the number of times backupDB has failed for this trigger. For
// empty files written by the systemd service task-scheduler-db-backup.service,
// returns the filename and 0.
func (b *gsDBBackup) findAndParseTriggerFile() (string, int, error) {
	dir, err := os.Open(b.triggerDir)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to read trigger directory %s: %s", b.triggerDir, err)
	}
	defer util.Close(dir)
	files, err := dir.Readdirnames(1)
	if err == io.EOF {
		return "", 0, nil
	} else if err != nil {
		return "", 0, fmt.Errorf("Unable to list trigger directory %s: %s", b.triggerDir, err)
	}
	basename := files[0]
	filename := path.Join(b.triggerDir, basename)
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to read trigger file %s: %s", filename, err)
	}
	trimmed := bytes.TrimSpace(content)
	retries := 0
	if len(trimmed) > 0 {
		retries, err = strconv.Atoi(string(trimmed))
		if err != nil {
			return "", 0, fmt.Errorf("Unable to parse trigger file %s: %s. Full content: %q", filename, err, string(content))
		}
	}
	return basename, retries, nil
}

// writeTriggerFile writes to the given trigger file indicating that the given
// number of backupDB attempts have failed.
func (b *gsDBBackup) writeTriggerFile(basename string, retries int) error {
	filename := path.Join(b.triggerDir, basename)
	content := []byte(strconv.Itoa(retries))
	if err := ioutil.WriteFile(filename, content, 0666); err != nil {
		return fmt.Errorf("Unable to write new retry count (%d) to trigger file %s: %s", retries, filename, err)
	}
	return nil
}

// deleteTriggerFile removes the given trigger file indicating that the backup
// succeeded or retries are exhausted.
func (b *gsDBBackup) deleteTriggerFile(basename string) error {
	filename := path.Join(b.triggerDir, basename)
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("Unable to remove trigger file %s: %s", filename, err)
	}
	return nil
}

// maybeBackupDB calls backupDB if TRIGGER_DIRNAME contains a file.
func (b *gsDBBackup) maybeBackupDB(now time.Time) {
	b.maybeBackupDBLiveness.Reset()
	// Look for a trigger file written by task-scheduler-db-backup.service
	// or a previous automatic backup attempt.
	basename, attemptCount, err := b.findAndParseTriggerFile()
	if err != nil {
		glog.Error(err)
	}
	if basename == "" {
		return
	}
	attemptCount++
	if attemptCount == 1 {
		glog.Infof("Beginning automatic DB backup.")
	} else {
		glog.Infof("Retrying automatic DB backup -- attempt %d.", attemptCount)
	}
	if err := b.backupDB(now, basename); err != nil {
		glog.Errorf("Automatic DB backup failed: %s", err)
		if attemptCount >= RETRY_COUNT {
			glog.Errorf("Automatic DB backup failed after %d attempts. Retries exhausted.", attemptCount)
			if err := b.deleteTriggerFile(basename); err != nil {
				glog.Error(err)
			}
		} else {
			if err := b.writeTriggerFile(basename, attemptCount); err != nil {
				glog.Error(err)
			}
		}
	} else {
		glog.Infof("Completed automatic DB backup.")
		if err := b.deleteTriggerFile(basename); err != nil {
			glog.Error(err)
		}
	}
}

// See documentation for DBBackup.Tick.
func (b *gsDBBackup) Tick() {
	now := time.Now()
	// TODO(benjaminwagner): Tick should return as soon as the DB file is written.
	b.maybeBackupDB(now)
}

// formatJobObjectName returns the GS object name for a Job with the given id
// being uploaded at the given time.
func formatJobObjectName(ts time.Time, id string) string {
	return fmt.Sprintf("%s/%s/%s.%s", JOB_BACKUP_DIR, ts.UTC().Format("2006/01/02"), id, JOB_FILE_NAME_EXTENSION)
}

// parseIdFromJobObjectName returns the Job ID from a GS object name formatted
// with formatJobObjectName.
func parseIdFromJobObjectName(name string) string {
	return strings.TrimSuffix(path.Base(name), "."+JOB_FILE_NAME_EXTENSION)
}

// backupJob writes the given bytes to GS under the given Job id.
func (b *gsDBBackup) backupJob(now time.Time, id string, jobGob []byte) error {
	bucket := b.gsClient.Bucket(b.gsBucket)
	return upload(b.ctx, bytes.NewReader(jobGob), bucket, formatJobObjectName(now, id), now)
}

// incrementalBackupStep writes all recently modified Jobs to GS.
func (b *gsDBBackup) incrementalBackupStep(now time.Time) error {
	jobs, err := b.db.GetModifiedJobsGOB(b.modifiedJobsId)
	if db.IsUnknownId(err) {
		glog.Errorf("incrementalBackupStep too slow; GetModifiedJobsGOB expired id: %s", b.modifiedJobsId)
		b.incrementalBackupResetCount.Inc(1)
		id, startErr := b.db.StartTrackingModifiedJobs()
		if startErr != nil {
			return startErr
		}
		b.modifiedJobsId = id
		// Since we just started tracking, there's nothing to do.
		// TODO(benjaminwagner): Ideally, we should scan the JobCache for Jobs whose
		// DbModified time is after b.db.GetIncrementalBackupTime() and call
		// backupJob for each of them.
		return err
	} else if err != nil {
		return err
	}
	errs := []error{}
	for id, jobGob := range jobs {
		// TODO(benjaminwagner): Use goroutines.
		if err := b.backupJob(now, id, jobGob); err != nil {
			// We still want to process the remaining jobs.
			errs = append(errs, err)
			continue
		}
		b.jobBackupCount.Inc(1)
	}
	if len(errs) == 0 {
		if err := b.db.SetIncrementalBackupTime(now); err != nil {
			return err
		}
		b.incrementalBackupLiveness.Reset()
		return nil
	} else if len(errs) == 1 {
		return errs[0]
	} else {
		errStr := &bytes.Buffer{}
		fmt.Fprint(errStr, "Multiple errors performing incremental Job backups:")
		for _, err := range errs {
			fmt.Fprint(errStr, "\n", err.Error())
		}
		return errors.New(errStr.String())
	}
}

// downloadGOB reads, ungzips, and GOB-decodes the given object from GS.
func downloadGOB(ctx context.Context, bucket *storage.BucketHandle, objectname string, dst interface{}) error {
	objR, err := bucket.Object(objectname).NewReader(ctx)
	if err != nil {
		return err
	}
	// As long as we can decode the object, we don't care if Close returns an
	// error.
	defer util.Close(objR)
	gzR, err := gzip.NewReader(objR)
	if err != nil {
		return err
	}
	defer util.Close(gzR)
	if err := gob.NewDecoder(gzR).Decode(dst); err != nil {
		return fmt.Errorf("Error decoding GOB data: %s", err)
	}
	return nil
}

// See docs for DBBackup interface.
func (b *gsDBBackup) RetrieveJobs(since time.Time) (map[string]*db.Job, error) {
	sinceDir := path.Dir(formatJobObjectName(since, "dummy")) + "/"
	bucket := b.gsClient.Bucket(b.gsBucket)
	rv := map[string]*db.Job{}
	// Iterate from today backwards to sinceDir.
	for t := time.Now(); ; t = t.Add(-24 * time.Hour) {
		curDir := path.Dir(formatJobObjectName(t, "dummy")) + "/"
		if curDir < sinceDir {
			break
		}

		q := &storage.Query{Prefix: curDir, Versions: false}
		it := bucket.Objects(b.ctx, q)
		for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
			if err != nil {
				return nil, fmt.Errorf("Unable to list jobs in %s/%s: %s", b.gsBucket, curDir, err)
			}
			if obj.Updated.Before(since) {
				continue
			}

			// If rv already contains this Job, it is newer than this version, so
			// skip.
			id := parseIdFromJobObjectName(obj.Name)
			if _, ok := rv[id]; ok {
				continue
			}

			// TODO(benjaminwagner): Download and decode in parallel.
			var job db.Job
			if err := downloadGOB(b.ctx, bucket, obj.Name, &job); err != nil {
				return nil, fmt.Errorf("Unable to read %s/%s: %s", b.gsBucket, obj.Name, err)
			}
			rv[job.Id] = &job
		}
	}
	return rv, nil
}
