package coverageingest

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
	"go.skia.org/infra/go/vcsinfo"
)

var ctx = mock.AnythingOfType("*context.emptyCtx")
var callback = mock.AnythingOfType("func(*storage.ObjectAttrs)")

const FILE_CONTENT = "Filler"

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

func mockLongCommits(hashes ...string) []*vcsinfo.LongCommit {
	retval := []*vcsinfo.LongCommit{}
	for _, h := range hashes {

		retval = append(retval, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:   h,
				Author: "nobody",
			},
		})
	}
	return retval
}

func mockProcessingSteps() {
	unTar = func(tarpath, outpath string) error {
		return nil
	}
	calculateTotalCoverage = func(folders ...string) (CoverageSummary, error) {
		return CoverageSummary{}, nil
	}
}

func TestBlankIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	mockProcessingSteps()

	calculateTotalCoverage = func(folders ...string) (CoverageSummary, error) {
		return CoverageSummary{TotalLines: 40, MissedLines: 7}, nil
	}

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.profraw"})
	}).Return(nil).Once()

	mg.On("AllFilesInDirectory", ctx, "commit/d123045/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/d123045/Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Some-Config.profraw"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Other-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Other-Config.profraw"})
	}).Return(nil).Once()

	contents := []byte(FILE_CONTENT)
	mg.On("GetFileContents", ctx, mock.Anything).Return(contents, nil)

	i := New(tpath, mg)
	i.IngestCommits(mockLongCommits("abcdefgh", "d123045"))

	mg.AssertNumberOfCalls(t, "GetFileContents", 6)
	assertFilesExist(t, path.Join(tpath, "abcdefgh"), "Some-Config.text.tar", "Some-Config.profraw")
	assertFilesExist(t, path.Join(tpath, "d123045"), "Some-Config.text.tar", "Some-Config.profraw", "Other-Config.text.tar", "Other-Config.profraw")

	// spot check that the bytes were written appropriately
	f, err := os.Open(path.Join(tpath, "d123045", "Some-Config.profraw"))
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, contents, b)
	assert.NoError(t, f.Close())

	ir := []IngestedResults{
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "abcdefgh",
				Author: "nobody",
			},
			Jobs:          []CoverageSummary{{Name: "Some-Config", TotalLines: 40, MissedLines: 7}},
			TotalCoverage: CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "d123045",
				Author: "nobody",
			},
			Jobs: []CoverageSummary{
				{Name: "Other-Config", TotalLines: 40, MissedLines: 7},
				{Name: "Some-Config", TotalLines: 40, MissedLines: 7},
			},
			TotalCoverage: CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

func TestPartialIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	mockProcessingSteps()

	calculateTotalCoverage = func(folders ...string) (CoverageSummary, error) {
		return CoverageSummary{TotalLines: 40, MissedLines: 7}, nil
	}

	// Write the files so it appears they have already been ingested
	if _, err := fileutil.EnsureDirExists(path.Join(tpath, "abcdefgh")); err != nil {
		assert.Fail(t, "Could not make dir abcdefgh", err)
	}
	if _, err := fileutil.EnsureDirExists(path.Join(tpath, "d123045")); err != nil {
		assert.Fail(t, "Could not make dir abcdefgh", err)
	}
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Some-Config.text.tar"), FILE_CONTENT)
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Some-Config.profraw"), FILE_CONTENT)
	testutils.WriteFile(t, path.Join(tpath, "d123045", "Some-Config.text.tar"), FILE_CONTENT)
	testutils.WriteFile(t, path.Join(tpath, "d123045", "Some-Config.html.tar"), FILE_CONTENT)

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.profraw"})
	}).Return(nil).Once()

	mg.On("AllFilesInDirectory", ctx, "commit/d123045/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/d123045/Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Some-Config.html.tar"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Other-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/d123045/Other-Config.profraw.tar.gz"})
	}).Return(nil).Once()

	contents := []byte(FILE_CONTENT)
	mg.On("GetFileContents", ctx, mock.Anything).Return(contents, nil)

	i := New(tpath, mg)
	// Old results should be purgd on new ingest
	i.results = []IngestedResults{{Commit: &vcsinfo.ShortCommit{Hash: "Should not exist"}, Jobs: []CoverageSummary{{Name: "Go away"}}}}
	i.IngestCommits(mockLongCommits("abcdefgh", "d123045"))

	mg.AssertNumberOfCalls(t, "GetFileContents", 1)
	mg.AssertCalled(t, "GetFileContents", ctx, "commit/d123045/Other-Config.text.tar")
	// Don't download .tar.gz files

	assertFilesExist(t, path.Join(tpath, "abcdefgh"), "Some-Config.text.tar", "Some-Config.profraw")
	assertFilesExist(t, path.Join(tpath, "d123045"), "Some-Config.text.tar", "Some-Config.html.tar", "Other-Config.text.tar")

	ir := []IngestedResults{
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "abcdefgh",
				Author: "nobody",
			},
			Jobs:          []CoverageSummary{{Name: "Some-Config", TotalLines: 40, MissedLines: 7}},
			TotalCoverage: CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "d123045",
				Author: "nobody",
			},
			Jobs: []CoverageSummary{
				{Name: "Other-Config", TotalLines: 40, MissedLines: 7},
				{Name: "Some-Config", TotalLines: 40, MissedLines: 7},
			},
			TotalCoverage: CoverageSummary{TotalLines: 40, MissedLines: 7},
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

	mockProcessingSteps()

	called := 0
	unTar = func(tarpath, outpath string) error {
		called++
		testutils.AssertDeepEqual(t, path.Join(tpath, "abcdefgh", "Some-Config.html.tar"), tarpath)
		testutils.AssertDeepEqual(t, path.Join(tpath, "abcdefgh", "Some-Config", "html"), outpath)
		return nil
	}

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.html.tar"})
	}).Return(nil).Once()

	mg.On("GetFileContents", ctx, mock.Anything).Return([]byte(FILE_CONTENT), nil)

	i := New(tpath, mg)
	i.IngestCommits(mockLongCommits("abcdefgh"))

	mg.AssertCalled(t, "GetFileContents", ctx, "commit/abcdefgh/Some-Config.html.tar")
	testutils.AssertDeepEqual(t, 1, called) // unTar() should be called exactly once
}

func TestCallToCombine(t *testing.T) {
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mockProcessingSteps()

	called := 1
	calculateTotalCoverage = func(folders ...string) (CoverageSummary, error) {
		someFolder := path.Join(tpath, "abcdefgh", "Some-Config", "text", "coverage")
		otherFolder := path.Join(tpath, "abcdefgh", "Other-Config", "text", "coverage")
		if len(folders) == 1 && folders[0] == someFolder {
			called *= 3
		} else if len(folders) == 1 && folders[0] == otherFolder {
			called *= 5
		} else if len(folders) == 2 {
			assert.Equal(t, folders, []string{someFolder, otherFolder})
			called *= 7
		} else {
			assert.Failf(t, "unexpected call", "calculateTotalCoverage(%s)", folders)
		}
		return CoverageSummary{}, nil
	}

	// Assume they've been downloaded
	if _, err := fileutil.EnsureDirExists(path.Join(tpath, "abcdefgh")); err != nil {
		assert.Fail(t, "Could not make dir abcdefgh", err)
	}
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Some-Config.text.tar"), FILE_CONTENT)
	testutils.WriteFile(t, path.Join(tpath, "abcdefgh", "Other-Config.text.tar"), FILE_CONTENT)

	mg.On("AllFilesInDirectory", ctx, "commit/abcdefgh/", callback).Run(func(args mock.Arguments) {
		f := args.Get(2).(func(item *storage.ObjectAttrs))
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Some-Config.text.tar"})
		f(&storage.ObjectAttrs{Name: "commit/abcdefgh/Other-Config.text.tar"})
	}).Return(nil).Once()

	i := New(tpath, mg)
	i.IngestCommits(mockLongCommits("abcdefgh"))

	testutils.AssertDeepEqual(t, 3*5*7, called) // createSummary() should be called 3 times, once with each folder, then once with an array.
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

func TestCoverageDataOperations(t *testing.T) {
	testutils.SmallTest(t)
	c1 := coverageData{
		executableLines: map[int]bool{
			1: true,
			2: true,
			5: false,
			6: false,
			7: true,
			8: false,
		},
	}
	assert.Equal(t, 6, c1.Total())
	assert.Equal(t, 3, c1.Missed())
	c2 := coverageData{
		executableLines: map[int]bool{
			1: false,
			2: true,
			3: true,
			4: false,
			5: false,
			6: true,
		},
	}
	assert.Equal(t, 6, c2.Total())
	assert.Equal(t, 3, c2.Missed())
	expected := &coverageData{
		executableLines: map[int]bool{
			1: true,
			2: true,
			3: true,
			4: false,
			5: false,
			6: true,
			7: true,
			8: false,
		},
	}
	assert.Equal(t, 8, expected.Total())
	assert.Equal(t, 3, expected.Missed())

	testutils.AssertDeepEqual(t, expected, c1.Union(&c2))
	testutils.AssertDeepEqual(t, expected, c2.Union(&c1))
}

func TestCoverageDataParsingLLVM5(t *testing.T) {
	testutils.SmallTest(t)
	contents := testutils.MustReadFile("some-config.main.cpp")
	expected := &coverageData{
		executableLines: map[int]bool{
			6:  true,
			7:  true,
			8:  true,
			9:  true,
			10: true,
			11: true,
			12: true,
			13: true,
			14: true,
			15: true,
			16: true,
			17: true,
			18: true,
			19: true,
			25: false,
			26: false,
			27: false,
			29: true,
			30: true,
			31: true,
			32: true,
			33: true,
			34: true,
			35: true,
			36: true,
			39: false,
			40: false,
			41: false,
			42: false,
		},
	}
	assert.Equal(t, 29, expected.Total())
	assert.Equal(t, 7, expected.Missed())
	testutils.AssertDeepEqual(t, expected, parseLinesCovered(contents))

}

func TestCalculateTotalCoverage(t *testing.T) {
	testutils.MediumTest(t)

	// Set up a directory structure like what the coverage data looks like

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	somepath := path.Join(tpath, "Some-Config", "coverage", "mnt", "pd0", "work", "skia", "foo")
	otherpath := path.Join(tpath, "Other-Config", "coverage", "mnt", "pd0", "work", "skia", "foo")

	if _, err := fileutil.EnsureDirExists(somepath); err != nil {
		assert.Fail(t, "Could not make dir"+tpath, err)
	}
	if _, err := fileutil.EnsureDirExists(otherpath); err != nil {
		assert.Fail(t, "Could not make dir"+tpath, err)
	}

	contents := testutils.MustReadFile("some-config.main.cpp")
	testutils.WriteFile(t, path.Join(somepath, "main.cpp"), contents)
	contents = testutils.MustReadFile("some-config.header.h")
	testutils.WriteFile(t, path.Join(somepath, "one-header.h"), contents)
	contents = testutils.MustReadFile("other-config.main.cpp")
	testutils.WriteFile(t, path.Join(otherpath, "main.cpp"), contents)
	contents = testutils.MustReadFile("other-config.header.h")
	testutils.WriteFile(t, path.Join(otherpath, "two-header.h"), contents)

	tc, err := defaultCalculateTotalCoverage(path.Join(tpath, "Some-Config", "coverage"), path.Join(tpath, "Other-Config", "coverage"))
	assert.NoError(t, err)

	expected := CoverageSummary{
		TotalLines:  36, // Computed by hand
		MissedLines: 7,
	}

	testutils.AssertDeepEqual(t, expected, tc)

}
