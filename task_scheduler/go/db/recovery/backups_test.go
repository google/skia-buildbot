package recovery

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
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
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/option"

	"github.com/gorilla/mux"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
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
	injectGetTSError error
	injectWriteError error
}

func (tdb *testDB) Close() error {
	closer, ok := tdb.content.(io.Closer)
	if ok {
		return closer.Close()
	}
	return nil
}

func (tdb *testDB) WriteBackup(w io.Writer) error {
	defer util.Close(tdb) // close tdb.content
	if tdb.injectWriteError != nil {
		return tdb.injectWriteError
	}
	_, err := io.Copy(w, tdb.content)
	return err
}

func (*testDB) SetIncrementalBackupTime(time.Time) error {
	return nil
}

func (tdb *testDB) GetIncrementalBackupTime() (time.Time, error) {
	if tdb.injectGetTSError != nil {
		return time.Time{}, tdb.injectGetTSError
	}
	return time.Unix(TEST_DB_TIME, 0).UTC(), nil
}

// gsRoute returns the mux.Route for the GS server.
func gsRoute(mockMux *mux.Router) *mux.Route {
	return mockMux.Schemes("https").Host("www.googleapis.com")
}

// getMockedDBBackup returns a dbBackup that handles GS requests with mockMux.
// WriteBackup will write TEST_DB_CONTENT.
func getMockedDBBackup(t *testing.T, mockMux *mux.Router) (*dbBackup, context.CancelFunc) {
	return getMockedDBBackupWithContent(t, mockMux, bytes.NewReader([]byte(TEST_DB_CONTENT)))
}

// getMockedDBBackupWithContent is like getMockedDBBackup but WriteBackup will
// copy the given content.
func getMockedDBBackupWithContent(t *testing.T, mockMux *mux.Router, content io.Reader) (*dbBackup, context.CancelFunc) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(mockhttpclient.NewMuxClient(mockMux)))
	assert.NoError(t, err)

	db := &testDB{
		DB:      db.NewInMemoryDB(),
		content: content,
	}
	backuper, err := NewBackuperWithClient(ctx, TEST_BUCKET, db, gsClient)
	assert.NoError(t, err)
	b := backuper.(*dbBackup)
	return b, ctxCancel
}

// object represents a GS object for makeObjectResponse and makeObjectsResponse.
type object struct {
	bucket string
	name   string
	time   time.Time
}

// makeObjectResponse generates the JSON representation of a GS object.
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

// makeObjectsResponse generates the JSON representation of an array of GS objects.
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

// addNoFilesHandler causes r to respond to a request to list objects in
// TEST_BUCKET with prefix DB_BACKUP_DIR with an empty list of objects.
func addNoFilesHandler(r *mux.Router) {
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", DB_BACKUP_DIR).
		Handler(mockhttpclient.MockGetDialogue([]byte(makeObjectsResponse([]object{}))))
}

// getLastBackup should return zero time and empty name when there are no
// existing backups.
func TestGetLastBackupNoFiles(t *testing.T) {
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, name, err := b.getLastBackup()
	assert.NoError(t, err)
	assert.True(t, ts.IsZero())
	assert.Equal(t, "", name)
}

// getLastBackup should return info on the latest object when there are
// multiple.
func TestGetLastBackupTwoFiles(t *testing.T) {
	r := mux.NewRouter()
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", DB_BACKUP_DIR).
		Handler(mockhttpclient.MockGetDialogue([]byte(makeObjectsResponse([]object{
			{TEST_BUCKET, "a", time.Date(2016, 10, 5, 4, 0, 0, 0, time.UTC)},
			{TEST_BUCKET, "b", time.Date(2016, 10, 5, 5, 0, 0, 0, time.UTC)},
		}))))
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts, name, err := b.getLastBackup()
	assert.NoError(t, err)
	assert.True(t, ts.Equal(time.Date(2016, 10, 5, 5, 0, 0, 0, time.UTC)))
	assert.Equal(t, "b", name)
}

// getNextBackupName should return task-scheduler.bdb.gz for today's date when
// there are no existing backups.
func TestGetNextBackupNameNoneExisting(t *testing.T) {
	now := time.Now()
	r := mux.NewRouter()
	addNoFilesHandler(r)

	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	actualName, err := b.getNextBackupName(now)
	assert.NoError(t, err)
	assert.Equal(t, name, actualName)
}

// getNextBackupName should return task-schedulerN.bdb.gz for N=[2, 9] when there are existing
// backups for today.
func TestGetNextBackupNameSomeExisting(t *testing.T) {
	now := time.Now()
	r := mux.NewRouter()

	name1 := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"
	ts1 := time.Date(2016, 10, 5, 4, 1, 0, 0, time.UTC)
	objectResponses := []object{
		object{TEST_BUCKET, name1, ts1},
	}
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", DB_BACKUP_DIR).
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(makeObjectsResponse(objectResponses)))
		})
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	for i := 2; i < 10; i++ {
		expectedNameN := fmt.Sprintf("%s/%s/task-scheduler%d.bdb.gz", DB_BACKUP_DIR, now.UTC().Format("2006/01/02"), i)
		actualNameN, err := b.getNextBackupName(now)
		assert.NoError(t, err, "Got error for count %d", i)
		assert.Equal(t, expectedNameN, actualNameN)

		tsN := time.Date(2016, 10, 5, 4, i, 0, 0, time.UTC)
		objectResponses = append(objectResponses, object{TEST_BUCKET, expectedNameN, tsN})
	}

	// After 9 backups, we should get an error.
	_, err := b.getNextBackupName(now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Too many DB backups")
}

// getNextBackupName may return an error when it sees names it doesn't recognize.
func TestGetNextBackupNameInvalidExisting(t *testing.T) {
	now := time.Now()
	r := mux.NewRouter()

	namePrefix := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler"
	objectResponses := []object{}
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", DB_BACKUP_DIR).
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(makeObjectsResponse(objectResponses)))
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	ts1 := time.Date(2016, 10, 5, 4, 1, 0, 0, time.UTC)
	objectResponses = append(objectResponses, object{TEST_BUCKET, namePrefix + ".bdb", ts1})
	_, err := b.getNextBackupName(now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected suffix")

	ts2 := time.Date(2016, 10, 5, 4, 2, 0, 0, time.UTC)
	objectResponses = append(objectResponses, object{TEST_BUCKET, namePrefix + "x.bdb.gz", ts2})
	_, err = b.getNextBackupName(now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid syntax")

	// Invalid names should only cause errors for the current date.
	now = now.Add(24 * time.Hour)
	expectedName := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"
	actualName, err := b.getNextBackupName(now)
	assert.Equal(t, expectedName, actualName)
}

// writeDBBackupToFile should produce a file with contents equal to what
// WriteBackup wrote.
func TestWriteDBBackupToFile(t *testing.T) {
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
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
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
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
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
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
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
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

// addMultipartHandler causes r to respond to a request to add an object with
// the given name to TEST_BUCKET with a successful response and sets
// actualBytesGzip to the object contents. Also performs assertions on the
// request.
func addMultipartHandler(t *testing.T, r *mux.Router, name string, actualBytesGzip *[]byte) {
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
			assert.Equal(t, TEST_BUCKET, data["bucket"])
			assert.Equal(t, name, data["name"])
			assert.Equal(t, "application/gzip", data["contentType"])
			assert.Equal(t, fmt.Sprintf("attachment; filename=\"%s\"", path.Base(name)), data["contentDisposition"])
			dataPart, err := mr.NextPart()
			assert.NoError(t, err)
			*actualBytesGzip, err = ioutil.ReadAll(dataPart)
			assert.NoError(t, err)
			_, _ = w.Write([]byte(makeObjectResponse(object{TEST_BUCKET, name, time.Now()})))
		})
}

// uploadFile should upload a file to GS.
func TestUploadFile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "myfile.txt")
	assert.NoError(t, ioutil.WriteFile(filename, []byte(TEST_DB_CONTENT), os.ModePerm))

	now := time.Now().Round(time.Second)
	r := mux.NewRouter()
	addNoFilesHandler(r)
	name := "path/to/gsfile.txt.gz"
	var actualBytesGzip []byte
	addMultipartHandler(t, r, name, &actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	err = uploadFile(b.ctx, filename, b.gsClient.Bucket(b.gsBucket), name, now)
	assert.NoError(t, err)

	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip))
	assert.NoError(t, err)
	assert.True(t, now.Equal(gzR.Header.ModTime))
	assert.Equal(t, "myfile.txt", gzR.Header.Name)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, TEST_DB_CONTENT, string(actualBytes))
}

// uploadFile may fail if the file doesn't exist.
func TestUploadFileNoFile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "myfile.txt")

	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	now := time.Now()
	name := "path/to/gsfile.txt.gz"
	err = uploadFile(b.ctx, filename, b.gsClient.Bucket(b.gsBucket), name, now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to read temporary backup file")
}

// uploadFile may fail if the GS request fails.
func TestUploadFileUploadError(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "backups_test")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tempdir)

	filename := path.Join(tempdir, "myfile.txt")
	assert.NoError(t, ioutil.WriteFile(filename, []byte(TEST_DB_CONTENT), os.ModePerm))

	now := time.Now()
	r := mux.NewRouter()
	name := "path/to/gsfile.txt.gz"

	addNoFilesHandler(r)
	gsRoute(r).Methods("POST").
		Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	err = uploadFile(b.ctx, filename, b.gsClient.Bucket(b.gsBucket), name, now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "got HTTP response code 418 with body: I don't like your poem.")
}

// backupDB should create a GS object with the gzipped contents of the DB.
func TestBackupDB(t *testing.T) {
	var expectedBytes []byte
	{
		// Get expectedBytes from writeDBBackupToFile.
		r := mux.NewRouter()
		addNoFilesHandler(r)
		b, cancel := getMockedDBBackup(t, r)
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
	addNoFilesHandler(r)
	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o/%s", TEST_BUCKET, name)).
		Handler(mockhttpclient.MockGetError("Not Found", http.StatusNotFound))

	var actualBytesGzip []byte
	addMultipartHandler(t, r, name, &actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()
	b.nextBackupTime = now

	err := b.backupDB(now)
	assert.NoError(t, err)
	gzR, err := gzip.NewReader(bytes.NewReader(actualBytesGzip))
	assert.NoError(t, err)
	actualBytes, err := ioutil.ReadAll(gzR)
	assert.NoError(t, err)
	assert.NoError(t, gzR.Close())
	assert.Equal(t, expectedBytes, actualBytes)

	assert.True(t, b.nextBackupTime.After(now))
}

// backupDB should create a second object for the current date if a backup
// already exists.
func TestBackupDBAlreadyExists(t *testing.T) {
	now := time.Now()
	r := mux.NewRouter()

	name1 := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"
	ts1 := time.Date(2016, 10, 5, 4, 1, 0, 0, time.UTC)
	gsRoute(r).Methods("GET").
		Path(fmt.Sprintf("/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("prefix", DB_BACKUP_DIR).
		Handler(mockhttpclient.MockGetDialogue([]byte(makeObjectsResponse([]object{
			object{TEST_BUCKET, name1, ts1},
		}))))

	name2 := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler2.bdb.gz"

	var ignored []byte
	addMultipartHandler(t, r, name2, &ignored)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	err := b.backupDB(now)
	assert.NoError(t, err)
}

// testBackupDBLarge tests backupDB for DB contents larger than 8MB.
func testBackupDBLarge(t *testing.T, contentSize int64) {
	now := time.Now()
	r := mux.NewRouter()
	addNoFilesHandler(r)
	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"

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
			assert.Equal(t, "application/gzip", data["contentType"])
			assert.Equal(t, "attachment; filename=\"task-scheduler.bdb.gz\"", data["contentDisposition"])
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
		Headers("Content-Type", "application/gzip").
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
				w.Header().Set("Range", fmt.Sprintf("0-%d", recvBytes))
				w.WriteHeader(308)
			}
		})

	b, cancel := getMockedDBBackupWithContent(t, r, makeLargeDBContent(contentSize))
	defer cancel()

	// Check available disk space.
	output, err := exec.RunCommand(&exec.Command{
		Name: "df",
		Args: []string{"--block-size=1", "--output=avail", os.TempDir()},
	})
	assert.NoError(t, err, "df failed: %s", output)
	// Output looks like:
	//       Avail
	// 13704458240
	availSize, err := strconv.ParseInt(strings.TrimSpace(strings.Split(output, "\n")[1]), 10, 64)
	assert.NoError(t, err, "Unable to parse df output: %s", output)
	assert.True(t, availSize > contentSize, "Insufficient disk space to run test; need %d bytes, have %d bytes for %s", contentSize, availSize, os.TempDir())

	err = b.backupDB(now)
	assert.NoError(t, err)
	assert.True(t, complete)
}

// backupDB should work for a large-ish DB.
func TestBackupDBLarge(t *testing.T) {
	testutils.SkipIfShort(t)
	// Send 128MB. Add 1 so it's not a multiple of 8MB.
	var contentSize int64 = 128*1024*1024 + 1
	testBackupDBLarge(t, contentSize)
}

// backupDB should work for a 16GB DB.
func TestBackupDBHuge(t *testing.T) {
	testutils.SkipIfShort(t)
	// Normally, just skip this test. But if we've set a high timeout, we can run
	// it.
	// TODO(benjaminwagner): Use one of the new flags in
	// https://skia-review.googlesource.com/c/4041/
	fTimeout := flag.Lookup("test.timeout")
	if fTimeout == nil || fTimeout.Value.(flag.Getter).Get().(time.Duration) < 15*time.Minute {
		t.Skip("Must specify -timeout=15m (or more) to run TestBackupDBHuge.")
	}

	// Send 16GB. Add 1 so it's not a multiple of 8MB.
	var contentSize int64 = 16*1024*1024*1024 + 1
	testBackupDBLarge(t, contentSize)
}

// resetNextBackupTime should set nextBackupTime based on the current time and
// previous backup time.
func TestResetNextBackupTime(t *testing.T) {
	r := mux.NewRouter()
	addNoFilesHandler(r)
	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	test := func(now, last, expected time.Time) {
		b.resetNextBackupTime(now, last)
		assert.True(t, expected.Equal(b.nextBackupTime), "Expected %s, got %s", expected, b.nextBackupTime)
		assert.Equal(t, 0, b.retryCount)
	}

	testASAP := func(now, last time.Time) {
		test(now, last, now)
	}

	// No previous backup.
	testASAP(time.Date(2016, 10, 26, 0, 0, 0, 0, time.UTC), time.Time{})
	testASAP(time.Date(2016, 10, 26, 1, 2, 3, 4, time.UTC), time.Time{})
	testASAP(time.Date(2016, 10, 26, 5, 6, 7, 8, time.UTC), time.Time{})
	testASAP(time.Date(2016, 10, 26, 13, 14, 15, 16, time.UTC), time.Time{})
	testASAP(time.Date(2016, 10, 26, 23, 59, 59, 999999999, time.UTC), time.Time{})

	// Previous backup very old.
	lastBackup := time.Date(2016, 10, 24, 5, 0, 0, 0, time.UTC)
	testASAP(time.Date(2016, 10, 26, 0, 0, 0, 0, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 1, 2, 3, 4, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 5, 6, 7, 8, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 13, 14, 15, 16, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 23, 59, 59, 999999999, time.UTC), lastBackup)

	// Previous backup yesterday.
	lastBackup = time.Date(2016, 10, 25, 5, 0, 0, 0, time.UTC)
	expected := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	test(time.Date(2016, 10, 26, 0, 0, 0, 0, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 1, 2, 3, 4, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 4, 59, 59, 999999999, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC), lastBackup, expected)
	testASAP(time.Date(2016, 10, 26, 5, 0, 0, 1, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 5, 6, 7, 8, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 13, 14, 15, 16, time.UTC), lastBackup)
	testASAP(time.Date(2016, 10, 26, 23, 59, 59, 999999999, time.UTC), lastBackup)

	// Previous backup today.
	lastBackup = time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	expected = time.Date(2016, 10, 27, 5, 0, 0, 0, time.UTC)
	test(time.Date(2016, 10, 26, 5, 0, 0, 1, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 5, 6, 7, 8, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 13, 14, 15, 16, time.UTC), lastBackup, expected)
	test(time.Date(2016, 10, 26, 23, 59, 59, 999999999, time.UTC), lastBackup, expected)

	// Retry count reset.
	b.retryCount = 3
	test(time.Date(2016, 10, 26, 5, 0, 0, 1, time.UTC), lastBackup, expected)
}

// maybeBackupDB should do nothing if nextBackupTime is in the future.
func TestMaybeBackupDBNotYet(t *testing.T) {
	now := time.Now()
	r := mux.NewRouter()
	addNoFilesHandler(r)
	called := false
	gsRoute(r).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		util.Close(r.Body)
	})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	b.nextBackupTime = now.Add(time.Second)

	b.maybeBackupDB(now)
	assert.False(t, called)
	assert.True(t, now.Add(time.Second).Equal(b.nextBackupTime))
}

// maybeBackupDB should perform a backup and reset nextBackupTime if
// nextBackupTime is in the past.
func TestMaybeBackupDBSuccess(t *testing.T) {
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()
	addNoFilesHandler(r)
	name := DB_BACKUP_DIR + "/" + now.UTC().Format("2006/01/02") + "/task-scheduler.bdb.gz"

	var actualBytesGzip []byte
	addMultipartHandler(t, r, name, &actualBytesGzip)

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	b.nextBackupTime = now.Add(-time.Second)

	b.maybeBackupDB(now)

	assert.True(t, len(actualBytesGzip) > 0)
	assert.Equal(t, 0, b.retryCount)
	assert.True(t, b.nextBackupTime.Equal(time.Date(2016, 10, 27, 5, 0, 0, 0, time.UTC)))
}

// maybeBackupDB should not change nextBackupTime if the backup fails.
func TestMaybeBackupDBFail(t *testing.T) {
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()
	addNoFilesHandler(r)

	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	b.nextBackupTime = now.Add(-time.Second)

	b.maybeBackupDB(now)

	assert.Equal(t, 1, b.retryCount)
	assert.True(t, b.nextBackupTime.Equal(now.Add(-time.Second)))
}

// maybeBackupDB should reset nextBackupTime and retryCount if retries are
// exhausted.
func TestMaybeBackupDBRetriesExhausted(t *testing.T) {
	now := time.Date(2016, 10, 26, 5, 0, 0, 0, time.UTC)
	r := mux.NewRouter()
	addNoFilesHandler(r)

	gsRoute(r).Methods("POST").Path(fmt.Sprintf("/upload/storage/v1/b/%s/o", TEST_BUCKET)).
		Queries("uploadType", "multipart").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			util.Close(r.Body)
			http.Error(w, "I don't like your poem.", http.StatusTeapot)
		})

	b, cancel := getMockedDBBackup(t, r)
	defer cancel()

	b.nextBackupTime = now.Add(-time.Second)
	b.retryCount = 2

	b.maybeBackupDB(now)

	assert.Equal(t, 0, b.retryCount)
	assert.True(t, b.nextBackupTime.Equal(time.Date(2016, 10, 27, 5, 0, 0, 0, time.UTC)))
}
