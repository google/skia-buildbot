package recovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/exec"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"google.golang.org/api/option"
)

const (
	TEST_BUCKET     = "skia-test"
	TEST_DB_CONTENT = `
I'm a little database
Short and stout!
Here is my file handle
Here is my timeout.
When I get all locked up,
Hear me shout!
Make a backup
And write it out!
`
	TEST_DB_TIME         = 1477000000
	TEST_DB_CONTENT_SEED = 299792

	// If metrics2.Liveness.Get returns a value longer than
	// MAX_TEST_TIME_SECONDS, we know it wasn't reset during the test.
	MAX_TEST_TIME_SECONDS = 15 * 60
)

// Create an io.Reader that returns the given number of bytes.
func makeLargeDBContent(bytes int64) io.Reader {
	r := rand.New(rand.NewSource(TEST_DB_CONTENT_SEED))
	return &io.LimitedReader{
		R: r,
		N: bytes,
	}
}

// testDB implements db.BackupDBCloser.
type testDB struct {
	db.DB
	content          io.Reader
	ts               time.Time
	injectSetTSError error
	injectGetTSError error
	injectWriteError error
}

// Closes content if necessary.
func (tdb *testDB) Close() error {
	closer, ok := tdb.content.(io.Closer)
	if ok {
		return closer.Close()
	}
	return nil
}

// Implements BackupDBCloser.WriteBackup.
func (tdb *testDB) WriteBackup(w io.Writer) error {
	defer util.Close(tdb) // close tdb.content
	if tdb.injectWriteError != nil {
		return tdb.injectWriteError
	}
	_, err := io.Copy(w, tdb.content)
	return err
}

// Implements BackupDBCloser.SetIncrementalBackupTime.
func (tdb *testDB) SetIncrementalBackupTime(ts time.Time) error {
	if tdb.injectSetTSError != nil {
		return tdb.injectSetTSError
	}
	tdb.ts = ts
	return nil
}

// Implements BackupDBCloser.GetIncrementalBackupTime.
func (tdb *testDB) GetIncrementalBackupTime() (time.Time, error) {
	if tdb.injectGetTSError != nil {
		return time.Time{}, tdb.injectGetTSError
	}
	return tdb.ts.UTC(), nil
}

// getMockedDBBackup returns a gsDBBackup that handles GCS requests with mockMux.
// If mockMux is nil, an empty mux.Router is used. WriteBackup will write
// TEST_DB_CONTENT.
func getMockedDBBackup(t *testing.T, mockMux *mux.Router) (*gsDBBackup, context.CancelFunc) {
	return getMockedDBBackupWithContent(t, mockMux, bytes.NewReader([]byte(TEST_DB_CONTENT)))
}

// getMockedDBBackupWithContent is like getMockedDBBackup but WriteBackup will
// copy the given content.
func getMockedDBBackupWithContent(t *testing.T, mockMux *mux.Router, content io.Reader) (*gsDBBackup, context.CancelFunc) {
	if mockMux == nil {
		mockMux = mux.NewRouter()
	}
	ctx, ctxCancel := context.WithCancel(context.Background())
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(mockhttpclient.NewMuxClient(mockMux)))
	assert.NoError(t, err)

	dir, err := ioutil.TempDir("", "getMockedDBBackupWithContent")
	assert.NoError(t, err)
	assert.NoError(t, os.MkdirAll(path.Join(dir, TRIGGER_DIRNAME), os.ModePerm))

	db := &testDB{
		DB:      db.NewInMemoryDB(),
		content: content,
		ts:      time.Unix(TEST_DB_TIME, 0),
	}
	b, err := newGsDbBackupWithClient(ctx, TEST_BUCKET, db, "task_scheduler_db", dir, gsClient)
	assert.NoError(t, err)
	return b, func() {
		ctxCancel()
		testutils.RemoveAll(t, dir)
	}
}

// object represents a GCS object for makeObjectResponse and makeObjectsResponse.
type object struct {
	bucket string
	name   string
	time   time.Time
}

// makeObjectResponse generates the JSON representation of a GCS object.
func makeObjectResponse(obj object) string {
	timeStr := obj.time.UTC().Format(time.RFC3339)
	return fmt.Sprintf(`{
  "kind": "storage#object",
  "id": "%s/%s",
  "name": "%s",
  "bucket": "%s",
  "generation": "1",
  "metageneration": "1",
  "timeCreated": "%s",
  "updated": "%s",
  "storageClass": "STANDARD",
  "size": "15",
  "md5Hash": "d8dh5MIGdPoMfh/owveXhA==",
  "crc32c": "Oz54cA==",
  "etag": "CLD56dvBp8oCEAE="
}`, obj.bucket, obj.name, obj.name, obj.bucket, timeStr, timeStr)
}

// makeObjectsResponse generates the JSON representation of an array of GCS
// objects.
func makeObjectsResponse(objs []object) string {
	jsObjs := make([]string, 0, len(objs))
	for _, o := range objs {
		jsObjs = append(jsObjs, makeObjectResponse(o))
	}
	return fmt.Sprintf(`{
  "kind": "storage#objects",
  "items": [
%s
  ]
}`, strings.Join(jsObjs, ",\n"))
}

// gsRoute returns the mux.Route for the GCS server.
func gsRoute(mockMux *mux.Router) *mux.Route {
	return mockMux.Schemes("https").Host("www.googleapis.com")
}

// addListObjectsHandler causes r to respond to a request to list objects in
// TEST_BUCKET/prefix with the given objects, formatted with
// makeObjectsResponse.
func addListObjectsHandler(t *testing.T, r *mux.Router, prefix string, objs []object) {
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", prefix).
		Handler(mockhttpclient.MockGetDialogue([]byte(makeObjectsResponse(objs))))
}

// getBackupMetrics should return zero time and zero count when there are no
// existing backups.
func TestGetBackupMetricsNoFiles(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	r := mux.NewRouter()
	addListObjectsHandler(t, r, DB_BACKUP_DIR, []object{})
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, count, err := b.getBackupMetrics(now)
	assert.NoError(t, err)
	assert.True(t, ts.IsZero())
	assert.Equal(t, int64(0), count)
}

// getBackupMetrics should return the time of the latest object when there are
// multiple.
func TestGetBackupMetricsTwoFiles(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	addListObjectsHandler(t, r, DB_BACKUP_DIR, []object{
		{TEST_BUCKET, "a", now.Add(-1 * time.Hour).UTC()},
		{TEST_BUCKET, "b", now.Add(-2 * time.Hour).UTC()},
	})
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, count, err := b.getBackupMetrics(now)
	assert.NoError(t, err)
	assert.True(t, ts.Equal(now.Add(-1*time.Hour)), "Expected %s, got %s", now.Add(-1*time.Hour), ts)
	assert.Equal(t, int64(2), count)
}

// getBackupMetrics should not count objects that were not modified recently.
func TestGetBackupMetricsSeveralDays(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	addListObjectsHandler(t, r, DB_BACKUP_DIR, []object{
		{TEST_BUCKET, "a", now.Add(-49 * time.Hour).UTC()},
		{TEST_BUCKET, "b", now.Add(-25 * time.Hour).UTC()},
		{TEST_BUCKET, "c", now.Add(-1 * time.Hour).UTC()},
	})
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, count, err := b.getBackupMetrics(now)
	assert.NoError(t, err)
	assert.True(t, ts.Equal(now.Add(-1*time.Hour)))
	assert.Equal(t, int64(1), count)
}

// getBackupMetrics should return the latest backup time even if it is far in
// the past.
func TestGetBackupMetricsOld(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	addListObjectsHandler(t, r, DB_BACKUP_DIR, []object{
		{TEST_BUCKET, "a", now.Add(-49 * time.Hour).UTC()},
		{TEST_BUCKET, "b", now.Add(-128 * time.Hour).UTC()},
		{TEST_BUCKET, "c", now.Add(-762 * time.Hour).UTC()},
	})
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, count, err := b.getBackupMetrics(now)
	assert.NoError(t, err)
	assert.True(t, ts.Equal(now.Add(-49*time.Hour)))
	assert.Equal(t, int64(0), count)
}

// writeDBBackupToFile should produce a file with contents equal to what
// WriteBackup wrote.
func TestWriteDBBackupToFile(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "foo.bdb")
	err = b.writeDBBackupToFile(filename)
	assert.NoError(t, err)

	actualContents, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, TEST_DB_CONTENT, string(actualContents))
}

// writeDBBackupToFile should succeed even if GetIncrementalBackupTime returns
// an error.
func TestWriteDBBackupToFileGetIncrementalBackupTimeError(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	injectedError := fmt.Errorf("Not giving you the time of day!")
	// This should not prevent DB from being backed up.
	b.db.(*testDB).injectGetTSError = injectedError
	filename := path.Join(tempdir, "foo.bdb")
	err = b.writeDBBackupToFile(filename)
	assert.NoError(t, err)

	actualContents, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, TEST_DB_CONTENT, string(actualContents))
}

// writeDBBackupToFile could fail due to disk error.
func TestWriteDBBackupToFileCreateError(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "nonexistant_dir", "foo.bdb")
	err = b.writeDBBackupToFile(filename)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not create temp file to write DB backup")
}

// writeDBBackupToFile could fail due to DB error.
func TestWriteDBBackupToFileDBError(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	injectedError := fmt.Errorf("Can't back up: unable to shift to reverse.")
	b.db.(*testDB).injectWriteError = injectedError
	filename := path.Join(tempdir, "foo.bdb")
	err = b.writeDBBackupToFile(filename)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), injectedError.Error())
}

// addMultipartHandler causes r to respond to a request to add an object to
// TEST_BUCKET with a successful response and sets actualBytesGzip[object_name]
// to the object contents. Also performs assertions on the request.
func addMultipartHandler(t *testing.T, r *mux.Router, actualBytesGzip map[string][]byte) {
	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := mockhttpclient.MuxSafeT(t)
			mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			assert.NoError(t, err)
			assert.Equal(t, "multipart/related", mediaType)
			mr := multipart.NewReader(r.Body, params["boundary"])
			jsonPart, err := mr.NextPart()
			assert.NoError(t, err)
			data := map[string]string{}
			assert.NoError(t, json.NewDecoder(jsonPart).Decode(&data))
			name := data["name"]
			assert.Equal(t, TEST_BUCKET, data["bucket"])
			assert.Equal(t, "application/octet-stream", data["contentType"])
			assert.Equal(t, "gzip", data["contentEncoding"])
			assert.Equal(t, fmt.Sprintf("attachment; filename=\"%s\"", path.Base(name)), data["contentDisposition"])
			dataPart, err := mr.NextPart()
			assert.NoError(t, err)
			actualBytesGzip[name], err = ioutil.ReadAll(dataPart)
			assert.NoError(t, err)
			_, _ = w.Write([]byte(makeObjectResponse(object{TEST_BUCKET, name, time.Now()})))
		})
}

// upload should upload data to GCS.
func TestUpload(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	name := "path/to/gsfile.txt"
	err := upload(b.ctx, strings.NewReader(TEST_DB_CONTENT), b.gsClient.Bucket(b.gsBucket), name, now)
	assert.NoError(t, err)

	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name]))
	assert.NoError(t, err)
	assert.True(t, now.Equal(gzR.Header.ModTime))
	assert.Equal(t, "gsfile.txt", gzR.Header.Name)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, TEST_DB_CONTENT, string(actualBytes))
}

// upload may fail if the GCS request fails.
func TestUploadError(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	r := mux.NewRouter()
	name := "path/to/gsfile.txt"

	gsRoute(r).Methods("POST").
		Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	err := upload(b.ctx, strings.NewReader(TEST_DB_CONTENT), b.gsClient.Bucket(b.gsBucket), name, now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "got HTTP response code 418 with body: I don't like your poem.")
}

// uploadFile should upload a file to GCS.
func TestUploadFile(t *testing.T) {
	testutils.SmallTest(t)
	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "myfile.txt")
	assert.NoError(t, ioutil.WriteFile(filename, []byte(TEST_DB_CONTENT), os.ModePerm))

	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	name := "path/to/gsfile.txt"
	err = uploadFile(b.ctx, filename, b.gsClient.Bucket(b.gsBucket), name, now)
	assert.NoError(t, err)

	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name]))
	assert.NoError(t, err)
	assert.True(t, now.Equal(gzR.Header.ModTime))
	assert.Equal(t, "gsfile.txt", gzR.Header.Name)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, TEST_DB_CONTENT, string(actualBytes))
}

// uploadFile may fail if the file doesn't exist.
func TestUploadFileNoFile(t *testing.T) {
	testutils.SmallTest(t)
	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "myfile.txt")

	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	now := time.Now()
	name := "path/to/gsfile.txt"
	err = uploadFile(b.ctx, filename, b.gsClient.Bucket(b.gsBucket), name, now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to read temporary backup file")
}

// backupDB should create a GCS object with the gzipped contents of the DB.
func TestBackupDB(t *testing.T) {
	testutils.SmallTest(t)
	var expectedBytes []byte
	{
		// Get expectedBytes from writeDBBackupToFile.
		b, cancel := getMockedDBBackup(t, nil)
		defer cancel()

		tempdir, err := ioutil.TempDir("", "backups_test")
		assert.NoError(t, err)
		defer testutils.RemoveAll(t, tempdir)

		filename := path.Join(tempdir, "expected.bdb")
		err = b.writeDBBackupToFile(filename)
		assert.NoError(t, err)
		expectedBytes, err = ioutil.ReadFile(filename)
		assert.NoError(t, err)
	}

	now := time.Now()
	r := mux.NewRouter()

	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Test resetting incrementalBackupResetCount.
	b.incrementalBackupResetCount.Inc(1)

	err := b.backupDB(now, "task-scheduler")
	assert.NoError(t, err)
	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb"
	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name]))
	assert.NoError(t, err)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, expectedBytes, actualBytes)

	// incrementalBackupResetCount should be reset.
	assert.Equal(t, int64(0), b.incrementalBackupResetCount.Get())
}

// testBackupDBLarge tests backupDB for DB contents larger than 8MB.
func testBackupDBLarge(t *testing.T, contentSize int64) {
	now := time.Now()
	r := mux.NewRouter()
	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb"

	// https://cloud.google.com/storage/docs/json_api/v1/how-tos/resumable-upload
	uploadId := "resume_me_please"
	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "resumable").
		Headers("Content-Type", "application/json").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := mockhttpclient.MuxSafeT(t)
			data := map[string]string{}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&data))
			assert.Equal(t, TEST_BUCKET, data["bucket"])
			assert.Equal(t, name, data["name"])
			assert.Equal(t, "application/octet-stream", data["contentType"])
			assert.Equal(t, "gzip", data["contentEncoding"])
			assert.Equal(t, "attachment; filename=\"task-scheduler.bdb\"", data["contentDisposition"])
			uploadUrl, err := url.Parse(r.URL.String())
			assert.NoError(t, err)
			query := uploadUrl.Query()
			query.Set("upload_id", uploadId)
			uploadUrl.RawQuery = query.Encode()
			w.Header().Set("Location", uploadUrl.String())
		})

	rangeRegexp := regexp.MustCompile("bytes ([0-9]+|\\*)-?([0-9]+)?/([0-9]+|\\*)")

	var recvBytes int64 = 0
	complete := false

	// Despite what the documentation says, the Go client uses POST, not PUT.
	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "resumable", "upload_id", uploadId).
		Headers("Content-Type", "application/octet-stream").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := mockhttpclient.MuxSafeT(t)

			byteRange := rangeRegexp.FindStringSubmatch(r.Header.Get("Content-Range"))
			assert.Equal(t, 4, len(byteRange), "Unexpected request %v %s", r.Header, r.URL.String())

			assert.NotEqual(t, "*", byteRange[1], "Test does not support upload size that is a multiple of 8MB.")

			begin, err := strconv.ParseInt(byteRange[1], 10, 64)
			assert.NoError(t, err)
			assert.Equal(t, recvBytes, begin)

			end, err := strconv.ParseInt(byteRange[2], 10, 64)
			assert.NoError(t, err)

			finalChunk := false
			if byteRange[3] != "*" {
				size, err := strconv.ParseInt(byteRange[3], 10, 64)
				assert.NoError(t, err)
				finalChunk = size == end+1
			}

			recvBytes += end - begin + 1
			if finalChunk {
				complete = true
				_, _ = w.Write([]byte(makeObjectResponse(object{TEST_BUCKET, name, time.Now()})))
			} else {
				w.Header().Set("Range", fmt.Sprintf("0-%d", recvBytes-1))
				// https://github.com/google/google-api-go-client/commit/612451d2aabbf88084e4f1c48c0781073c0d5583
				w.Header().Set("X-HTTP-Status-Code-Override", "308")
				w.WriteHeader(200)
			}
		})

	b, cancel := getMockedDBBackupWithContent(t, r, makeLargeDBContent(contentSize))
	defer cancel()

	// Check available disk space.
	output, err := exec.RunCommand(context.Background(), &exec.Command{
		Name: "df",
		Args: []string{"--block-size=1", "--output=avail", os.TempDir()},
	})
	assert.NoError(t, err, "df failed: %s", output)
	// Output looks like:
	//       Avail
	// 13704458240
	availSize, err := strconv.ParseInt(strings.TrimSpace(strings.Split(output, "\n")[1]), 10, 64)
	assert.NoError(t, err, "Unable to parse df output: %s", output)
	assert.True(t, availSize > contentSize, "Insufficient disk space to run test; need %d bytes, have %d bytes for %s. Please set TMPDIR.", contentSize, availSize, os.TempDir())

	err = b.backupDB(now, "task-scheduler")
	assert.NoError(t, err)
	assert.True(t, complete)
}

// backupDB should work for a large-ish DB.
func TestBackupDBLarge(t *testing.T) {
	testutils.LargeTest(t)
	// Send 128MB. Add 1 so it's not a multiple of 8MB.
	var contentSize int64 = 128*1024*1024 + 1
	testBackupDBLarge(t, contentSize)
}

// backupDB should work for a 16GB DB.
func TestBackupDBHuge(t *testing.T) {
	t.Skipf("TODO(benjaminwagner): change TMPDIR to make this work.")
	testutils.LargeTest(t)
	// Send 16GB. Add 1 so it's not a multiple of 8MB.
	var contentSize int64 = 16*1024*1024*1024 + 1
	testBackupDBLarge(t, contentSize)
}

// immediateBackupBasename should return a name based on the time of day.
func TestImmediateBackupBasename(t *testing.T) {
	testutils.SmallTest(t)
	test := func(expected string, input time.Time) {
		assert.Equal(t, expected, immediateBackupBasename(input))
	}
	test("task-scheduler-00:00:00", time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC))
	test("task-scheduler-01:02:03", time.Date(2016, 2, 29, 1, 2, 3, 0, time.UTC))
	test("task-scheduler-13:14:15", time.Date(2016, 10, 27, 13, 14, 15, 16171819, time.UTC))
	test("task-scheduler-23:59:59", time.Date(2016, 12, 31, 23, 59, 59, 999999999, time.UTC))
}

// findAndParseTriggerFile should return an error when the directory doesn't
// exist.
func TestFindAndParseTriggerFileNoDir(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	testutils.RemoveAll(t, b.triggerDir)
	_, _, err := b.findAndParseTriggerFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to read trigger directory")
}

// findAndParseTriggerFile should return empty for an empty dir.
func TestFindAndParseTriggerFileNoFile(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	file, attempts, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "", file)
	assert.Equal(t, 0, attempts)
}

// findAndParseTriggerFile should return the filename and indicate no attempts
// for an empty file.
func TestFindAndParseTriggerFileNewFile(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	exec_testutils.Run(t, context.Background(), b.triggerDir, "touch", "foo")
	file, attempts, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "foo", file)
	assert.Equal(t, 0, attempts)
}

// findAndParseTriggerFile should choose one of the files when multiple are
// present.
func TestFindAndParseTriggerFileTwoFiles(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	ctx := context.Background()
	exec_testutils.Run(t, ctx, b.triggerDir, "touch", "foo")
	exec_testutils.Run(t, ctx, b.triggerDir, "touch", "bar")
	file, attempts, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.True(t, file == "foo" || file == "bar")
	assert.Equal(t, 0, attempts)
}

// writeTriggerFile followed by findAndParseTriggerFile should return the same
// values.
func TestWriteFindAndParseTriggerFileWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	for i := 1; i < 3; i++ {
		assert.NoError(t, b.writeTriggerFile("foo", i))
		file, attempts, err := b.findAndParseTriggerFile()
		assert.NoError(t, err)
		assert.Equal(t, "foo", file)
		assert.Equal(t, i, attempts)
	}
}

// writeTriggerFile could fail if permissions are incorrect.
func TestWriteTriggerFileReadOnly(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	assert.NoError(t, ioutil.WriteFile(path.Join(b.triggerDir, "foo"), []byte{}, 0444))
	err := b.writeTriggerFile("foo", 1)
	assert.Error(t, err)
	assert.Regexp(t, `Unable to write new retry count \(1\) to trigger file .*/foo: .*permission denied`, err.Error())
}

// findAndParseTriggerFile should return an error when the file can't be parsed.
func TestFindAndParseTriggerFileInvalidContents(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	assert.NoError(t, ioutil.WriteFile(path.Join(b.triggerDir, "foo"), []byte("Hi Mom!"), 0666))
	_, _, err := b.findAndParseTriggerFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to parse trigger file")
}

// deleteTriggerFile followed by findAndParseTriggerFile should return empty.
func TestDeleteTriggerFile(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	assert.NoError(t, b.writeTriggerFile("foo", 1))
	file, attempts, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "foo", file)
	assert.Equal(t, 1, attempts)

	assert.NoError(t, b.deleteTriggerFile("foo"))
	file, attempts, err = b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "", file)
	assert.Equal(t, 0, attempts)

	files, err := ioutil.ReadDir(b.triggerDir)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(files))
}

// deleteTriggerFile could fail if file has already been deleted.
func TestDeleteTriggerFileAlreadyDeleted(t *testing.T) {
	testutils.SmallTest(t)
	b, cancel := getMockedDBBackup(t, nil)
	defer cancel()

	err := b.deleteTriggerFile("foo")
	assert.Error(t, err)
	assert.Regexp(t, "Unable to remove trigger file .*/foo: .*no such file", err.Error())
}

// maybeBackupDB should do nothing if there is no trigger file.
func TestMaybeBackupDBNotYet(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	r := mux.NewRouter()
	called := false
	gsRoute(r).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	b.maybeBackupDB(now)
	assert.False(t, called)
}

// maybeBackupDB should find the trigger file and perform a backup, then delete
// the trigger file if successful.
func TestMaybeBackupDBSuccess(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()

	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	assert.NoError(t, b.writeTriggerFile("task-scheduler", 0))

	b.maybeBackupDB(now)

	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb"
	assert.True(t, len(actualBytesGzip[name]) > 0)

	file, _, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "", file)
}

// maybeBackupDB should write the number of attempts to the trigger file if the
// backup fails.
func TestMaybeBackupDBFail(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()

	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	assert.NoError(t, b.writeTriggerFile("task-scheduler", 0))

	b.maybeBackupDB(now)

	file, attempts, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "task-scheduler", file)
	assert.Equal(t, 1, attempts)
}

// maybeBackupDB should delete the trigger file if retries are exhausted.
func TestMaybeBackupDBRetriesExhausted(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()

	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	assert.NoError(t, b.writeTriggerFile("task-scheduler", 2))

	b.maybeBackupDB(now)

	file, _, err := b.findAndParseTriggerFile()
	assert.NoError(t, err)
	assert.Equal(t, "", file)
}

func TestFormatJobObjectName(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "job-backup/2016/01/01/police-officer.gob", formatJobObjectName(time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC), "police-officer"))
	assert.Equal(t, "job-backup/2016/02/29/nurse.gob", formatJobObjectName(time.Date(2016, 2, 29, 1, 2, 3, 0, time.UTC), "nurse"))
	assert.Equal(t, "job-backup/2008/08/08/scientist.gob", formatJobObjectName(time.Date(2008, 8, 8, 8, 8, 8, 8, time.UTC), "scientist"))
}

func TestParseIdFromJobObjectName(t *testing.T) {
	testutils.SmallTest(t)
	test := func(id string) {
		assert.Equal(t, id, parseIdFromJobObjectName(formatJobObjectName(time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC), id)))
	}
	test("police-officer")
	test("name.with.internal.dots")
	test("20161116T220425.634818978Z_0000000000001e88")
}

// makeJob returns a dummy Job without Id and DbModified set.
func makeJob(now time.Time) *db.Job {
	return &db.Job{
		Created:      now.UTC(),
		Dependencies: map[string][]string{},
		RepoState: db.RepoState{
			Repo: db.DEFAULT_TEST_REPO,
		},
		Name:  "Test-Job",
		Tasks: map[string][]*db.TaskSummary{},
	}
}

// makeExistingJob returns a dummy Job with Id and DbModified set to the given
// values.
func makeExistingJob(now time.Time, id string) *db.Job {
	job := makeJob(now)
	job.Id = id
	job.DbModified = now.UTC()
	return job
}

// backupJob should create a GCS object with the gzipped bytes.
func TestBackupJob(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()

	j := makeJob(now)
	var buf bytes.Buffer
	assert.NoError(t, gob.NewEncoder(&buf).Encode(j))
	jobgob := buf.Bytes()

	r := mux.NewRouter()

	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	err := b.backupJob(now, "myjob", jobgob)
	assert.NoError(t, err)
	name := JOB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/myjob.gob"
	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name]))
	assert.NoError(t, err)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, jobgob, actualBytes)
}

// incrementalBackupStep should just update the incremental backup time when
// there are no jobs.
func TestIncrementalBackupStepNoJobs(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Metrics occasionally fail to be deleted, so we might have a leftover from a
	// previous test.
	beforeCount := b.jobBackupCount.Get()

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	now := time.Now()
	assert.NoError(t, b.incrementalBackupStep(now))

	newTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, now.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() < MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount, b.jobBackupCount.Get())
}

// incrementalBackupStep should back up each added or modified job.
func TestIncrementalBackupStep(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	namePrefix := JOB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/"

	r := mux.NewRouter()
	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Metrics occasionally fail to be deleted, so we might have a leftover from a
	// previous test.
	beforeCount := b.jobBackupCount.Get()

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	// Add a job.
	j1 := makeJob(now)
	assert.NoError(t, b.db.PutJob(j1))
	name1 := namePrefix + j1.Id + ".gob"

	assert.NoError(t, b.incrementalBackupStep(now))

	// Check the uploaded data.
	{
		gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name1]))
		assert.NoError(t, err)
		var j1Copy *db.Job
		assert.NoError(t, gob.NewDecoder(gzR).Decode(&j1Copy))
		assert.NoError(t, gzR.Close())
		deepequal.AssertDeepEqual(t, j1, j1Copy)
	}

	newTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, now.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() < MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount+1, b.jobBackupCount.Get())

	// Modify j1 and add j2.
	j1.Status = db.JOB_STATUS_CANCELED
	j2 := makeJob(now.Add(time.Second))
	assert.NoError(t, b.db.PutJobs([]*db.Job{j1, j2}))
	name2 := namePrefix + j2.Id + ".gob"

	assert.NoError(t, b.incrementalBackupStep(now))

	// Check the uploaded data.
	{
		gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name1]))
		assert.NoError(t, err)
		var j1Copy *db.Job
		assert.NoError(t, gob.NewDecoder(gzR).Decode(&j1Copy))
		assert.NoError(t, gzR.Close())
		deepequal.AssertDeepEqual(t, j1, j1Copy)
	}

	{
		gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip[name2]))
		assert.NoError(t, err)
		var j2Copy *db.Job
		assert.NoError(t, gob.NewDecoder(gzR).Decode(&j2Copy))
		assert.NoError(t, gzR.Close())
		deepequal.AssertDeepEqual(t, j2, j2Copy)
	}

	assert.Equal(t, beforeCount+3, b.jobBackupCount.Get())
}

// incrementalBackupStep should continue when one job can not be uploaded.
func TestIncrementalBackupStepSingleUploadError(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()

	r := mux.NewRouter()
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Metrics occasionally fail to be deleted, so we might have a leftover from a
	// previous test.
	beforeCount := b.jobBackupCount.Get()

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	// Add two jobs.
	j1 := makeJob(now)
	j2 := makeJob(now.Add(time.Second))
	assert.NoError(t, b.db.PutJobs([]*db.Job{j1, j2}))

	count := 0
	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			if count == 1 {
				util.Close(r.Body)
				http.Error(w, "No one wants this job.", http.StatusTeapot)
				return
			}
			t := mockhttpclient.MuxSafeT(t)
			mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			assert.NoError(t, err)
			assert.Equal(t, "multipart/related", mediaType)
			mr := multipart.NewReader(r.Body, params["boundary"])
			jsonPart, err := mr.NextPart()
			assert.NoError(t, err)
			_, err = io.Copy(w, jsonPart)
			assert.NoError(t, err)
		})

	oldTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)

	err = b.incrementalBackupStep(now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "got HTTP response code 418 with body: No one wants this job.")

	assert.Equal(t, 2, count)

	newTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, oldTs.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() > MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount+1, b.jobBackupCount.Get())
}

// incrementalBackupStep should report multiple errors when they occur.
func TestIncrementalBackupStepMultipleUploadError(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()

	r := mux.NewRouter()
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Metrics occasionally fail to be deleted, so we might have a leftover from a
	// previous test.
	beforeCount := b.jobBackupCount.Get()

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	// Add two jobs.
	j1 := makeJob(now)
	j2 := makeJob(now.Add(time.Second))
	assert.NoError(t, b.db.PutJobs([]*db.Job{j1, j2}))

	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "No one wants this job.", http.StatusTeapot)
		})

	oldTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)

	err = b.incrementalBackupStep(now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple errors performing incremental Job backups")

	newTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, oldTs.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() > MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount, b.jobBackupCount.Get())
}

// incrementalBackupStep should restart modified job tracking on ErrUnknownId.
func TestIncrementalBackupStepReset(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()

	actualBytesGzip := map[string][]byte{}
	addMultipartHandler(t, r, actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	// Metrics occasionally fail to be deleted, so we might have a leftover from a
	// previous test.
	beforeCount := b.jobBackupCount.Get()

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	// Invalidate the ID.
	b.db.StopTrackingModifiedJobs(b.modifiedJobsId)

	oldTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)

	now := time.Now()
	err = b.incrementalBackupStep(now)
	assert.True(t, db.IsUnknownId(err))

	assert.Equal(t, int64(1), b.incrementalBackupResetCount.Get())

	newTs, err := b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, oldTs.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() > MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount, b.jobBackupCount.Get())
	assert.Equal(t, 0, len(actualBytesGzip))

	// Ensure next round succeeds.
	now = now.Add(10 * time.Second)
	j1 := makeJob(now)
	assert.NoError(t, b.db.PutJob(j1))
	name1 := JOB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/" + j1.Id + ".gob"

	assert.NoError(t, b.incrementalBackupStep(now))

	newTs, err = b.db.GetIncrementalBackupTime()
	assert.NoError(t, err)
	assert.True(t, now.Equal(newTs))
	assert.True(t, b.incrementalBackupLiveness.Get() < MAX_TEST_TIME_SECONDS)

	assert.Equal(t, beforeCount+1, b.jobBackupCount.Get())
	assert.Equal(t, 1, len(actualBytesGzip))
	assert.True(t, len(actualBytesGzip[name1]) > 0)
}

// incrementalBackupStep should return an error if unable to set the incremental
// backup time in the DB.
func TestIncrementalBackupStepSetTSError(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	injectedError := fmt.Errorf("It's too late. Self-destruct sequence has been initiated.")
	b.db.(*testDB).injectSetTSError = injectedError

	b.incrementalBackupLiveness.ManualReset(time.Time{})

	now := time.Now()
	err := b.incrementalBackupStep(now)
	assert.Equal(t, injectedError, err)

	assert.True(t, b.incrementalBackupLiveness.Get() > MAX_TEST_TIME_SECONDS)
}

// addGetObjectHandler causes r to respond to a request for the contents of
// TEST_BUCKET/name with the given contents.
func addGetObjectHandler(t *testing.T, r *mux.Router, name string, contents []byte) {
	// URI does not match documentation at https://cloud.google.com/storage/docs/json_api/v1/objects/get
	r.Schemes("https").Host("storage.googleapis.com").Methods("GET").
		Path(fmt.Sprintf("/%s/%s", TEST_BUCKET, name)).
		Handler(mockhttpclient.MockGetDialogue(contents))
}

// addGetJobGOBHandler causes r to respond to a request for the given job (in
// TEST_BUCKET with name given by formatJobObjectName) with the GOB-encoded Job.
func addGetJobGOBHandler(t *testing.T, r *mux.Router, job *db.Job) {
	buf := &bytes.Buffer{}
	assert.NoError(t, gob.NewEncoder(buf).Encode(job))
	addGetObjectHandler(t, r, formatJobObjectName(job.DbModified, job.Id), buf.Bytes())
}

// downloadGOB should download data from GCS.
func TestDownloadGOB(t *testing.T) {
	testutils.SmallTest(t)

	now := time.Now()
	job := makeExistingJob(now, "j1")

	r := mux.NewRouter()
	addGetJobGOBHandler(t, r, job)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	var jobCopy db.Job
	name := formatJobObjectName(job.DbModified, job.Id)
	err := downloadGOB(b.ctx, b.gsClient.Bucket(b.gsBucket), name, &jobCopy)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, job, &jobCopy)
}

// downloadGOB should return a sensible error if the object doesn't exist.
func TestDownloadGOBNotFound(t *testing.T) {
	testutils.SmallTest(t)

	r := mux.NewRouter()
	name := "foo/bar/baz.gob"
	r.Schemes("https").Host("storage.googleapis.com").Methods("GET").
		Path(fmt.Sprintf("/%s/%s", TEST_BUCKET, name)).
		Handler(mockhttpclient.MockGetError("Not Found", http.StatusNotFound))

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	var dummy db.Job
	err := downloadGOB(b.ctx, b.gsClient.Bucket(b.gsBucket), name, &dummy)
	assert.Error(t, err)
	assert.Regexp(t, "object doesn't exist", err.Error())
	deepequal.AssertDeepEqual(t, db.Job{}, dummy)
}

// downloadGOB should return an error if the data is not GOB-encoded.
func TestDownloadGOBNotGOB(t *testing.T) {
	testutils.SmallTest(t)

	r := mux.NewRouter()

	name := "poem.txt"
	addGetObjectHandler(t, r, name, []byte(TEST_DB_CONTENT))

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	var dummy db.Job
	err := downloadGOB(b.ctx, b.gsClient.Bucket(b.gsBucket), name, &dummy)
	assert.Error(t, err)
	assert.Regexp(t, "Error decoding GOB data", err.Error())
	deepequal.AssertDeepEqual(t, db.Job{}, dummy)
}

// addGetJobGOBsHandlers causes r to respond to list and get requests for the
// given Jobs. Calls addGetJobGOBHandler for each Job. Calls
// addListObjectsHandler for each dir.
func addGetJobGOBsHandlers(t *testing.T, r *mux.Router, jobsByDir map[string][]*db.Job) {
	for dir, jobs := range jobsByDir {
		objs := make([]object, len(jobs), len(jobs))
		for i, job := range jobs {
			name := formatJobObjectName(job.DbModified, job.Id)
			addGetJobGOBHandler(t, r, job)
			objs[i] = object{TEST_BUCKET, name, job.DbModified}
		}
		addListObjectsHandler(t, r, dir+"/", objs)
	}
}

// assertJobMapsEqual asserts expected and actual are deep equal. If not,
// provides a useful indication of their differences to FailNow.
func assertJobMapsEqual(t *testing.T, expected map[string]*db.Job, actual map[string]*db.Job) {
	msg := &bytes.Buffer{}
	for id, eJob := range expected {
		if aJob, ok := actual[id]; ok {
			if !reflect.DeepEqual(eJob, aJob) {
				if _, err := fmt.Fprintf(msg, "Job %q differs:\n\tExpected: %v\n\tActual:   %v\n", id, eJob, aJob); err != nil {
					sklog.Fatal(err)
				}
			}
		} else {
			if _, err := fmt.Fprintf(msg, "Missing job %q: %v\n", id, eJob); err != nil {
				sklog.Fatal(err)
			}
		}
	}
	for id, aJob := range actual {
		if _, ok := expected[id]; !ok {
			if _, err := fmt.Fprintf(msg, "Extra job %q: %v\n", id, aJob); err != nil {
				sklog.Fatal(err)
			}
		}
	}
	if msg.Len() > 0 {
		assert.FailNow(t, msg.String())
	}
}

// RetrieveJobs should download Jobs for the requested period from GCS.
func TestRetrieveJobsSimple(t *testing.T) {
	testutils.SmallTest(t)

	now := time.Now().Round(time.Second)
	since := now.Add(-1 * time.Hour)
	expectedJobs := map[string]*db.Job{}

	job1 := makeExistingJob(since.Add(-10*time.Minute), "before")
	job1dir := path.Dir(formatJobObjectName(job1.DbModified, job1.Id))

	job2 := makeExistingJob(since.Add(10*time.Minute), "after")
	job2dir := path.Dir(formatJobObjectName(job2.DbModified, job2.Id))
	expectedJobs[job2.Id] = job2.Copy()

	r := mux.NewRouter()
	allJobsByDir := map[string][]*db.Job{}
	if job1dir == job2dir {
		allJobsByDir[job1dir] = []*db.Job{job1, job2}
	} else {
		allJobsByDir[job1dir] = []*db.Job{job1}
		allJobsByDir[job2dir] = []*db.Job{job2}
	}
	nowdir := path.Dir(formatJobObjectName(time.Now(), "dummy"))
	if job2dir != nowdir {
		allJobsByDir[nowdir] = []*db.Job{}
	}
	addGetJobGOBsHandlers(t, r, allJobsByDir)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	actualJobs, err := b.RetrieveJobs(since)
	assert.NoError(t, err)
	assertJobMapsEqual(t, expectedJobs, actualJobs)
}

// RetrieveJobs should download Jobs for the requested period from GCS where the
// Jobs span multiple directories.
func TestRetrieveJobsMultipleDirs(t *testing.T) {
	testutils.MediumTest(t) // GOB encoding and decoding takes time.

	now := time.Now().Round(time.Second)
	since := now.Add(-26 * time.Hour)
	allJobsByDir := map[string][]*db.Job{}
	expectedJobs := map[string]*db.Job{}
	// Add jobs before since. Not expected from RetrieveJobs.
	for i := -26 * time.Hour; i < 0; i += time.Hour {
		ts := since.Add(i)
		job := makeExistingJob(ts, fmt.Sprintf("%s", i))
		dir := path.Dir(formatJobObjectName(job.DbModified, job.Id))
		allJobsByDir[dir] = append(allJobsByDir[dir], job)
	}
	// Add jobs at and after since. Expected from RetrieveJobs.
	for i := time.Duration(0); i <= 26*time.Hour; i += time.Hour {
		ts := since.Add(i)
		job := makeExistingJob(ts, fmt.Sprintf("%s", i))
		dir := path.Dir(formatJobObjectName(job.DbModified, job.Id))
		allJobsByDir[dir] = append(allJobsByDir[dir], job)
		expectedJobs[job.Id] = job.Copy()
	}

	r := mux.NewRouter()
	addGetJobGOBsHandlers(t, r, allJobsByDir)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	actualJobs, err := b.RetrieveJobs(since)
	assert.NoError(t, err)
	assertJobMapsEqual(t, expectedJobs, actualJobs)
}

// RetrieveJobs should download Jobs for the requested period from GCS when there
// are older versions for the same Job.
func TestRetrieveJobsMultipleVersions(t *testing.T) {
	testutils.MediumTest(t) // GOB encoding and decoding takes time.

	now := time.Now().Round(time.Second)
	since := now.Add(-26 * time.Hour)
	allJobsByDir := map[string][]*db.Job{}
	expectedJobs := map[string]*db.Job{}
	// Add and modify jobs before since. Not expected from RetrieveJobs.
	for i := -26 * time.Hour; i < -time.Hour; i += time.Hour {
		ts := since.Add(i)
		origjob := makeExistingJob(ts, fmt.Sprintf("before-mod-before-%s", i))
		origdir := path.Dir(formatJobObjectName(origjob.DbModified, origjob.Id))
		modjob := origjob.Copy()
		modjob.Status = db.JOB_STATUS_CANCELED
		modjob.DbModified = ts.Add(time.Hour).UTC()
		moddir := path.Dir(formatJobObjectName(modjob.DbModified, modjob.Id))
		allJobsByDir[moddir] = append(allJobsByDir[moddir], modjob)
		if origdir != moddir {
			allJobsByDir[origdir] = append(allJobsByDir[origdir], origjob)
		}
	}
	// Add jobs created before since and modified after since. Expected from
	// RetrieveJobs.
	for i := time.Hour; i < 26*time.Hour; i += time.Hour {
		ts := since.Add(-i)
		origjob := makeExistingJob(ts, fmt.Sprintf("before-mod-after-%s", i))
		origdir := path.Dir(formatJobObjectName(origjob.DbModified, origjob.Id))
		modjob := origjob.Copy()
		modjob.Status = db.JOB_STATUS_CANCELED
		modjob.DbModified = since.Add(i).UTC()
		moddir := path.Dir(formatJobObjectName(modjob.DbModified, modjob.Id))
		allJobsByDir[moddir] = append(allJobsByDir[moddir], modjob)
		if origdir != moddir {
			allJobsByDir[origdir] = append(allJobsByDir[origdir], origjob)
		}
		expectedJobs[modjob.Id] = modjob.Copy()
	}
	// Add jobs created and modified after since. Expected from RetrieveJobs.
	for i := time.Duration(0); i < 26*time.Hour; i += time.Hour {
		ts := since.Add(i)
		origjob := makeExistingJob(ts, fmt.Sprintf("after-mod-after-%s", i))
		origdir := path.Dir(formatJobObjectName(origjob.DbModified, origjob.Id))
		modjob := origjob.Copy()
		modjob.Status = db.JOB_STATUS_SUCCESS
		modjob.DbModified = ts.Add(time.Hour).UTC()
		moddir := path.Dir(formatJobObjectName(modjob.DbModified, modjob.Id))
		allJobsByDir[moddir] = append(allJobsByDir[moddir], modjob)
		if origdir != moddir {
			allJobsByDir[origdir] = append(allJobsByDir[origdir], origjob)
		}
		expectedJobs[modjob.Id] = modjob.Copy()
	}

	r := mux.NewRouter()
	addGetJobGOBsHandlers(t, r, allJobsByDir)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	actualJobs, err := b.RetrieveJobs(since)
	assert.NoError(t, err)
	assertJobMapsEqual(t, expectedJobs, actualJobs)
}

// RetrieveJobs should give a sensible error if unable to list Jobs in GCS.
func TestRetrieveJobsErrorListingJobs(t *testing.T) {
	testutils.SmallTest(t)

	r := mux.NewRouter()
	now := time.Now().Round(time.Second)
	prefix := JOB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/"
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", prefix).
		Handler(mockhttpclient.MockGetError("No jobs today", http.StatusTeapot))
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	_, err := b.RetrieveJobs(now)
	assert.Error(t, err)
	assert.Regexp(t, "Unable to list jobs in "+TEST_BUCKET+"/"+prefix, err)
}

// RetrieveJobs should give a sensible error if unable to download a Job from
// GCS.
func TestRetrieveJobsErrorDownloading(t *testing.T) {
	testutils.SmallTest(t)

	r := mux.NewRouter()
	now := time.Now().Round(time.Second)
	prefix := JOB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/"
	name := prefix + "j1.gob"
	addListObjectsHandler(t, r, prefix, []object{
		{TEST_BUCKET, name, now.UTC()},
	})
	addGetObjectHandler(t, r, name, []byte("Hi Mom!"))
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	_, err := b.RetrieveJobs(now)
	assert.Error(t, err)
	assert.Regexp(t, `Unable to read .*/j1\.gob`, err)
}
