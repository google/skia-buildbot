// Implementation of backing up a DB to Google Cloud Storage (GS).
package recovery

import (
	"compress/gzip"
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
	"google.golang.org/api/option"
)

const (
	// DB_BACKUP_DIR, DB_FILE_NAME_BASE, and DB_FILE_NAME_EXTENSION specify the
	// object name to store DB backups in the GS bucket.
	DB_BACKUP_DIR          = "db-backup"
	DB_FILE_NAME_BASE      = "task-scheduler"
	DB_FILE_NAME_EXTENSION = "bdb"
	// DB_BACKUP_TARGET_TIME_OF_DAY is duration after midnight UTC to trigger an
	// automatic backup.
	DB_BACKUP_TARGET_TIME_OF_DAY = 5 * time.Hour
	// DB_BACKUP_MIN_DURATION_BETWEEN_BACKUPS is the minimum time difference from
	// the last backup to perform an automatic backup. This grace period allows
	// correctly triggering a daily backup even if the service is not running at
	// DB_BACKUP_TARGET_TIME_OF_DAY. The max duration is 24 hours plus this value.
	DB_BACKUP_MIN_DURATION_BETWEEN_BACKUPS = 3 * time.Hour
)

// dbBackup implements Backuper.
type dbBackup struct {
	// gsBucket specifies the GS bucket (for testing).
	gsBucket string
	// gsClient accesses gsBucket.
	gsClient *storage.Client
	// db is the DB to back up.
	db db.BackupDBCloser
	// ctx allows to stop the dbBackup.
	ctx context.Context
	// lastDBBackupLiveness records the modified time of the most recent DB
	// backup.
	lastDBBackupLiveness *metrics2.Liveness
	// maybeBackupDBLiveness records whether maybeBackupDB is being called.
	maybeBackupDBLiveness *metrics2.Liveness
	// nextBackupTime indicates the next time maybeBackupDB will trigger a backup.
	nextBackupTime time.Time
	// retryCount is the number of times we have attempted to back up at the
	// current value of nextBackupTime.
	retryCount int
}

// getLastBackup returns the Updated time and Name of the most recent DB backup,
// or the zero time and empty string if no backups exist. Does not return an
// error unless the request could not be completed.
func (b *dbBackup) getLastBackup() (time.Time, string, error) {
	lastTime := time.Time{}
	lastName := ""
	err := gs.AllFilesInDir(b.gsClient, b.gsBucket, DB_BACKUP_DIR, func(item *storage.ObjectAttrs) {
		if item.Updated.After(lastTime) {
			lastTime = item.Updated
			lastName = item.Name
		}
	})
	return lastTime, lastName, err
}

// updateLastBackupTime updates the metric for the time since last successful
// backup.
func (b *dbBackup) updateLastBackupTime() {
	last, _, err := b.getLastBackup()
	if err != nil {
		glog.Errorf("Failed to get last DB backup time: %s", err)
	}
	b.lastDBBackupLiveness.ManualReset(last)
	glog.Infof("Last DB backup was %s.", last)
}

// getNextBackupName returns the object name for a DB backup created at the
// given time, taking care not to overwrite existing backups. Returns an error
// if there are too many backups for today.
func (b *dbBackup) getNextBackupName(now time.Time) (string, error) {
	now = now.UTC()
	_, lastName, err := b.getLastBackup()
	if err != nil {
		return "", err
	}
	prefix := path.Join(DB_BACKUP_DIR, now.Format("2006/01/02"), DB_FILE_NAME_BASE)
	suffix := fmt.Sprintf(".%s.gz", DB_FILE_NAME_EXTENSION)
	if strings.HasPrefix(lastName, prefix) {
		if !strings.HasSuffix(lastName, suffix) {
			return "", fmt.Errorf("Unrecognized previous backup filename %q; expected suffix %s", lastName, suffix)
		}
		lastNameNum := lastName[len(prefix) : len(lastName)-len(suffix)]
		count := 1
		if len(lastNameNum) > 0 {
			count, err = strconv.Atoi(lastNameNum)
			if err != nil {
				return "", fmt.Errorf("Unrecognized previous backup filename %q: %s", lastName, err)
			}
		}
		count++
		// Allow writing up to 9 versions per day (normally only one per day).
		if count >= 10 {
			return "", fmt.Errorf("Too many DB backups for today (%s).", now.Format("2006/01/02"))
		}
		return fmt.Sprintf("%s%d%s", prefix, count, suffix), nil
	}
	return fmt.Sprintf("%s%s", prefix, suffix), nil
}

// writeDBBackupToFile creates filename and writes the DB to it. File may be
// written even if an error is returned.
func (b *dbBackup) writeDBBackupToFile(filename string) error {
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
func uploadFile(ctx context.Context, filename string, bucket *storage.BucketHandle, objectname string, modTime time.Time) error {
	fileR, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Unable to read temporary backup file: %s", err)
	}
	// If we are able to successfully read temp file until EOF, we don't
	// care if Close returns an error.
	defer util.Close(fileR)
	objW := bucket.Object(objectname).NewWriter(ctx)
	defer func() {
		// We set objW to nil when we manually close it below.
		if objW != nil {
			util.Close(objW)
		}
	}()
	objW.ObjectAttrs.ContentType = "application/gzip"
	objW.ObjectAttrs.ContentDisposition = fmt.Sprintf("attachment; filename=\"%s\"", path.Base(objectname))
	gzW := gzip.NewWriter(objW)
	defer func() {
		// We set gzW to nil when we manually close it below.
		if gzW != nil {
			util.Close(gzW)
		}
	}()
	gzW.Header.Name = path.Base(filename)
	gzW.Header.ModTime = modTime.UTC()
	if _, err := io.Copy(gzW, fileR); err != nil {
		return err
	}
	if err, gzW = gzW.Close(), nil; err != nil {
		return err
	}
	err, objW = objW.Close(), nil
	return err
}

// backupDB performs an immediate backup of b.db.
func (b *dbBackup) backupDB(now time.Time) (err error) {
	objectname, err := b.getNextBackupName(now)
	if err != nil {
		return err
	}
	// We expect TMPDIR to be set to a location that can store a large file at
	// high throughput.
	tempdir, err := ioutil.TempDir("", "dbbackup")
	if err != nil {
		return err
	}
	defer util.RemoveAll(tempdir)
	tempfilename := path.Join(tempdir, fmt.Sprintf("%s.%s", DB_FILE_NAME_BASE, DB_FILE_NAME_EXTENSION))

	modTime, err := b.db.GetIncrementalBackupTime()
	if err != nil {
		glog.Warningf("Error getting DB incremental backup time; using current time instead. %s", err)
		modTime = now
	}
	if err := b.writeDBBackupToFile(tempfilename); err != nil {
		return err
	}
	bucket := b.gsClient.Bucket(b.gsBucket)
	if err := uploadFile(b.ctx, tempfilename, bucket, objectname, modTime); err != nil {
		return err
	}
	// Last backup was now.
	b.resetNextBackupTime(now, now)
	return nil
}

// See documentation for Backuper.ImmediateBackup.
func (b *dbBackup) ImmediateBackup() error {
	return b.backupDB(time.Now())
}

// resetNextBackupTime sets b.nextBackupTime based on the current time and the most recent backup time. Also resets b.retryCount.
func (b *dbBackup) resetNextBackupTime(now, last time.Time) {
	now = now.UTC()
	if util.TimeIsZero(last) || now.Sub(last) > DB_BACKUP_MIN_DURATION_BETWEEN_BACKUPS+24*time.Hour {
		b.nextBackupTime = now
	} else {
		year, month, day := now.Date()
		b.nextBackupTime = time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Add(DB_BACKUP_TARGET_TIME_OF_DAY)
		for b.nextBackupTime.Sub(last) < DB_BACKUP_MIN_DURATION_BETWEEN_BACKUPS {
			b.nextBackupTime = b.nextBackupTime.Add(24 * time.Hour)
		}
	}
	if !b.nextBackupTime.After(now) {
		b.nextBackupTime = now
		glog.Infof("Next automatic DB backup will occur at the next opportunity.")
	} else {
		glog.Infof("Next automatic DB backup scheduled for %s (%s from now)", b.nextBackupTime, b.nextBackupTime.Sub(now))
	}
	b.retryCount = 0
}

// maybeBackupDB calls backupDB daily at DB_BACKUP_TARGET_TIME_OF_DAY.
func (b *dbBackup) maybeBackupDB(now time.Time) {
	b.maybeBackupDBLiveness.Reset()
	if b.nextBackupTime.After(now) {
		return
	}
	if b.retryCount == 0 {
		glog.Infof("Beginning automatic DB backup.")
	} else {
		glog.Infof("Retrying automatic DB backup -- attempt %d.", b.retryCount+1)
	}
	if err := b.backupDB(now); err != nil {
		glog.Errorf("Automatic DB backup failed: %s", err)
		b.retryCount++
		if b.retryCount >= 3 {
			glog.Errorf("Automatic DB backup failed after %d attempts.", b.retryCount)
			// Pass "now" for "last" so that we don't retry for another 24 hours.
			b.resetNextBackupTime(now, now)
		}
	} else {
		glog.Infof("Completed automatic DB backup.")
	}
}

func (b *dbBackup) Tick() {
	now := time.Now()
	// TODO(benjaminwagner): Remove this once we start doing incremental backups.
	if err := b.db.SetIncrementalBackupTime(now); err != nil {
		glog.Errorf("Unable to set incremental backup time: %s", err)
	}
	b.maybeBackupDB(now)
}

// Backuper has methods to trigger periodic and immediate backups.
type Backuper interface {
	// Tick triggers a periodic backup if one is due. This allows callers to
	// perform backups when it is less likely to conflict with other actions.
	Tick()
	// ImmediateBackup triggers a backup immediately.
	ImmediateBackup() error
}

// NewBackuper creates a Backuper.
//  - ctx can be used to stop any background processes as well as to interrupt
//    Tick or ImmediateBackup.
//  - gsBucket is the GS bucket to store backups.
//  - db is the DB to back up.
//  - authClient is a client authenticated with auth.SCOPE_READ_WRITE.
func NewBackuper(ctx context.Context, gsBucket string, db db.BackupDBCloser, authClient *http.Client) (Backuper, error) {
	gsClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(authClient))
	if err != nil {
		return nil, err
	}
	return NewBackuperWithClient(ctx, gsBucket, db, gsClient)
}

// NewBackuperWithClient is the same as NewBackuper but takes a GS client for
// testing.
func NewBackuperWithClient(ctx context.Context, gsBucket string, db db.BackupDBCloser, gsClient *storage.Client) (Backuper, error) {
	b := &dbBackup{
		gsBucket: gsBucket,
		gsClient: gsClient,
		db:       db,
		ctx:      ctx,
		lastDBBackupLiveness: metrics2.NewLiveness("last-db-backup", map[string]string{
			"database": "task_scheduler",
		}),
		maybeBackupDBLiveness: metrics2.NewLiveness("db-backup-maybe-backup-db", map[string]string{
			"database": "task_scheduler",
		}),
	}

	last, _, err := b.getLastBackup()
	if err != nil {
		return nil, err
	}
	b.resetNextBackupTime(time.Now(), last)
	go util.RepeatCtx(10*time.Minute, b.ctx, b.updateLastBackupTime)

	return b, nil
}
