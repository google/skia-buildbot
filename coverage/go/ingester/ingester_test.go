package ingester

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"cloud.google.com/go/storage"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/mockgcsclient"
	"go.skia.org/infra/go/testutils"
)

var ctx = mock.AnythingOfType("*context.emptyCtx")
var callback = mock.AnythingOfType("func(*storage.ObjectAttrs)")

func assertFilesExist(t *testing.T, basePath string, files ...string) {
	for _, f := range files {
		fullPath := path.Join(basePath, f)
		if fi, err := os.Stat(fullPath); os.IsNotExist(err) {
			assert.Fail(t, "File should have existed and it does not", fullPath)
		} else if err != nil {
			assert.FailNow(t, "Unexpected error", err.Error())
		} else {
			assert.Falsef(t, fi.IsDir(), "File %s should be a real file, not a directory", fullPath)
		}
	}
}

func TestBlankIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	unTar = func(tarpath, outpath string) error {
		assert.FailNow(t, "unTar should not be called")
		return nil
	}

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "Some-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Some-Config.profraw"})
	}).Return(nil).Once()

	mg.On("AllFilesInDirectory", ctx, "commit/d123045/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "Some-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Some-Config.profraw"})
		f(&storage.ObjectAttrs{Name: "Other-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Other-Config.profraw"})
	}).Return(nil).Once()

	contents := []byte("Filler")
	mg.On("GetFileContents", ctx, mock.Anything).Return(contents, nil)

	i := New(tpath, mg)
	i.IngestCommits([]string{"abcdefgh", "d123045"})

	mg.AssertNumberOfCalls(t, "GetFileContents", 6)
	assertFilesExist(t, path.Join(tpath, "abcdefgh"), "Some-Config.profdata", "Some-Config.profraw")
	assertFilesExist(t, path.Join(tpath, "d123045"), "Some-Config.profdata", "Some-Config.profraw", "Other-Config.profdata", "Other-Config.profraw")

	// spot check that the bytes were written appropriately
	f, err := os.Open(path.Join(tpath, "d123045", "Some-Config.profraw"))
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, contents, b)
	assert.NoError(t, f.Close())

	ir := []IngestedResults{
		{
			Commit: "abcdefgh",
			Jobs:   []JobSummary{{Name: "Some-Config", TotalLines: 100, MissedLines: 50}},
		},
		{
			Commit: "d123045",
			Jobs: []JobSummary{
				{Name: "Other-Config", TotalLines: 100, MissedLines: 50},
				{Name: "Some-Config", TotalLines: 100, MissedLines: 50},
			},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

func TestPartialIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	unTar = func(tarpath, outpath string) error {
		assert.FailNow(t, "unTar should not be called")
		return nil
	}

	// Write the files so it appears they have already been ingested
	if _, err := fileutil.EnsureDirExists(path.Join(tpath, "abcdefgh")); err != nil {
		assert.Fail(t, "Could not make dir abcdefgh", err)
	}
	if _, err := fileutil.EnsureDirExists(path.Join(tpath, "d123045")); err != nil {
		assert.Fail(t, "Could not make dir abcdefgh", err)
	}
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Some-Config.profdata"), "")
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Some-Config.profraw"), "")
	testutils.WriteFile(t, path.Join(tpath, "d123045", "Some-Config.profdata"), "")
	testutils.WriteFile(t, path.Join(tpath, "d123045", "Some-Config.text.tar"), "")

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "Some-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Some-Config.profraw"})
	}).Return(nil).Once()

	mg.On("AllFilesInDirectory", ctx, "commit/d123045/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "Some-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "Other-Config.profdata"})
		f(&storage.ObjectAttrs{Name: "Other-Config.profraw.tar.gz"})
	}).Return(nil).Once()

	contents := []byte("Filler")
	mg.On("GetFileContents", ctx, mock.Anything).Return(contents, nil)

	i := New(tpath, mg)
	// Old results should be purgd on new ingest
	i.results = []IngestedResults{{Commit: "Should not exist", Jobs: []JobSummary{{Name: "Go away"}}}}
	i.IngestCommits([]string{"abcdefgh", "d123045"})

	mg.AssertNumberOfCalls(t, "GetFileContents", 1)
	mg.AssertCalled(t, "GetFileContents", ctx, "commit/d123045/Other-Config.profdata")
	// Don't download .tar.gz files

	assertFilesExist(t, path.Join(tpath, "abcdefgh"), "Some-Config.profdata", "Some-Config.profraw")
	assertFilesExist(t, path.Join(tpath, "d123045"), "Some-Config.profdata", "Some-Config.text.tar", "Other-Config.profdata")

	ir := []IngestedResults{
		{
			Commit: "abcdefgh",
			Jobs:   []JobSummary{{Name: "Some-Config", TotalLines: 100, MissedLines: 50}},
		},
		{
			Commit: "d123045",
			Jobs: []JobSummary{
				{Name: "Other-Config", TotalLines: 100, MissedLines: 50},
				{Name: "Some-Config", TotalLines: 100, MissedLines: 50},
			},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

func TestTarIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	called := 0
	unTar = func(tarpath, outpath string) error {
		called++
		testutils.AssertDeepEqual(t, path.Join(tpath, "abcdefgh", "Some-Config.html.tar"), tarpath)
		testutils.AssertDeepEqual(t, path.Join(tpath, "abcdefgh", "Some-Config", "html"), outpath)
		return nil
	}

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "Some-Config.html.tar"})
	}).Return(nil).Once()

	mg.On("GetFileContents", ctx, mock.Anything).Return([]byte("Filler"), nil)

	i := New(tpath, mg)
	i.IngestCommits([]string{"abcdefgh"})

	mg.AssertCalled(t, "GetFileContents", ctx, "commit/abcdefgh/Some-Config.html.tar")
	testutils.AssertDeepEqual(t, 1, called) // unTar() should be called exactly once
}

func TestUntarDefaultStructure(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	tarpath, err := testutils.TestDataDir()
	assert.NoError(t, err, "Problem with getting TestDataDir")
	tarpath = path.Join(tarpath, "Sample-Config.html.tar")
	// The sample tar folder has the same directory structure that comes off
	// the Linux coverage bots.

	err = defaultUnTar(tarpath, path.Join(tpath, "SomeFolder"))
	assert.NoError(t, err, "Problem untarring")

	assertFilesExist(t, path.Join(tpath, "SomeFolder"), "bar.html", "foo.html")
	assertFilesExist(t, path.Join(tpath, "SomeFolder", "coverage", "mnt", "pd0", "work", "skia", "dm"), "alpha.cpp")
}
