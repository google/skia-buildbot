package coverageingest

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"cloud.google.com/go/storage"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/coverage/go/common"
	"go.skia.org/infra/coverage/go/db"
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

func mockProcessingSteps() *db.MockCoverageCache {
	// TODO(kjlubick): Use exec.NewContext to mock out the call to tar.
	unTar = func(ctx context.Context, tarpath, outpath string) error {
		return nil
	}
	calculateCoverage = func(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
		return common.CoverageSummary{}, nil
	}
	mcc := db.MockCoverageCache{}
	mcc.On("CheckCache", mock.Anything).Return(common.CoverageSummary{}, false)
	mcc.On("StoreToCache", mock.Anything, mock.Anything).Return(nil)
	return &mcc
}

// Tests that we download files correctly from GCS when starting from fresh.
// Doesn't test untarring, coverage calculation, or caching
func TestBlankIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()
	mcc := mockProcessingSteps()

	calculateCoverage = func(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
		return common.CoverageSummary{TotalLines: 40, MissedLines: 7}, nil
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

	i := New(tpath, mg, mcc)
	i.IngestCommits(ctx, mockLongCommits("abcdefgh", "d123045"))

	mg.AssertNumberOfCalls(t, "GetFileContents", 6)
	assertFilesExist(t, path.Join(tpath, "abcdefgh"), "Some-Config.text.tar", "Some-Config.profraw")
	assertFilesExist(t, path.Join(tpath, "d123045"), "Some-Config.text.tar", "Some-Config.profraw", "Other-Config.text.tar", "Other-Config.profraw")

	// spot check that the bytes were written appropriately
	f, err := os.Open(path.Join(tpath, "d123045", "Some-Config.profraw"))
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, contents, b)

	ir := []IngestedResults{
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "abcdefgh",
				Author: "nobody",
			},
			Jobs:          []common.CoverageSummary{{Name: "Some-Config", TotalLines: 40, MissedLines: 7}},
			TotalCoverage: common.CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "d123045",
				Author: "nobody",
			},
			Jobs: []common.CoverageSummary{
				{Name: "Other-Config", TotalLines: 40, MissedLines: 7},
				{Name: "Some-Config", TotalLines: 40, MissedLines: 7},
			},
			TotalCoverage: common.CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

// Tests that we download files correctly from GCS when some files already exist.
// Additionally tests that we re-calculate coverage for anything that is a cache miss
// (which is everything)
// Doesn't test untarring, coverage calculation, or caching
func TestPartialIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()
	mcc := mockProcessingSteps()

	calculateCoverage = func(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
		return common.CoverageSummary{TotalLines: 40, MissedLines: 7}, nil
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

	i := New(tpath, mg, mcc)
	// Old results should be purgd on new ingest
	i.results = []IngestedResults{{Commit: &vcsinfo.ShortCommit{Hash: "Should not exist"}, Jobs: []common.CoverageSummary{{Name: "Go away"}}}}
	i.IngestCommits(ctx, mockLongCommits("abcdefgh", "d123045"))

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
			Jobs:          []common.CoverageSummary{{Name: "Some-Config", TotalLines: 40, MissedLines: 7}},
			TotalCoverage: common.CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "d123045",
				Author: "nobody",
			},
			Jobs: []common.CoverageSummary{
				{Name: "Other-Config", TotalLines: 40, MissedLines: 7},
				{Name: "Some-Config", TotalLines: 40, MissedLines: 7},
			},
			TotalCoverage: common.CoverageSummary{TotalLines: 40, MissedLines: 7},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

// Tests that we correctly call the untar() function after downloading a tar file.
// Doesn't test downloading from GCS, coverage calculation, or caching.
func TestTarIngestion(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mcc := mockProcessingSteps()

	called := 0
	unTar = func(ctx context.Context, tarpath, outpath string) error {
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

	i := New(tpath, mg, mcc)
	i.IngestCommits(ctx, mockLongCommits("abcdefgh"))

	mg.AssertCalled(t, "GetFileContents", ctx, "commit/abcdefgh/Some-Config.html.tar")
	testutils.AssertDeepEqual(t, 1, called) // unTar() should be called exactly once
}

// Tests that the call to the "get combined coverage" function is well-formed.
// Doesn't test downloading from GCS, coverage calculation, untarring, or caching.
func TestCallToCombine(t *testing.T) {
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()

	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mcc := mockProcessingSteps()

	called := 1
	calculateCoverage = func(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
		someFolder := path.Join(tpath, "abcdefgh", "Some-Config", "text", "coverage")
		otherFolder := path.Join(tpath, "abcdefgh", "Other-Config", "text", "coverage")
		if len(folders) == 1 && folders[0] == someFolder {
			called *= 3
			assert.Equal(t, renderInfo{}, ri, "Expecting empty render info for anything that's not the combined coverage")
		} else if len(folders) == 1 && folders[0] == otherFolder {
			called *= 5
			assert.Equal(t, renderInfo{}, ri, "Expecting empty render info for anything that's not the combined coverage")
		} else if len(folders) == 2 {
			called *= 7
			assert.Equal(t, "Combined", ri.jobName)
			assert.Equal(t, "abcdefgh", ri.commit)
			assert.Equal(t, []string{otherFolder, someFolder}, folders)
			assert.Equal(t, path.Join(tpath, "abcdefgh", "Combined", "html"), ri.outputPath)
		} else {
			assert.Failf(t, "unexpected call", "calculateCoverage(%s)", folders)
		}
		return common.CoverageSummary{}, nil
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

	i := New(tpath, mg, mcc)
	i.IngestCommits(ctx, mockLongCommits("abcdefgh"))

	testutils.AssertDeepEqual(t, 3*5*7, called) // createSummary() should be called 3 times, once with each folder, then once with an array.
}

// Tests the interface between the ingestion and the caching layer,
// i.e. that we use the cached results when appropriate.
// Doesn't test downloading from GCS, coverage calculation, or untarring.
func TestCachingCalls(t *testing.T) {
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()
	mg := mockgcsclient.New()
	defer mg.AssertExpectations(t)

	mcc := &db.MockCoverageCache{}
	mcc.On("CheckCache", "Some-Config:abcdefgh").Return(common.CoverageSummary{MissedLines: 7, TotalLines: 77}, true)
	mcc.On("CheckCache", mock.Anything).Return(common.CoverageSummary{}, false).Twice()
	mcc.On("StoreToCache", mock.Anything, common.CoverageSummary{MissedLines: 12, TotalLines: 17}).Return(nil).Twice()
	defer mcc.AssertExpectations(t)

	mockProcessingSteps()

	called := 0
	calculateCoverage = func(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
		called++
		return common.CoverageSummary{MissedLines: 12, TotalLines: 17}, nil
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

	i := New(tpath, mg, mcc)
	i.IngestCommits(ctx, mockLongCommits("abcdefgh"))
	assert.Equal(t, 2, called)
	ir := []IngestedResults{
		{
			Commit: &vcsinfo.ShortCommit{
				Hash:   "abcdefgh",
				Author: "nobody",
			},
			Jobs: []common.CoverageSummary{
				{Name: "Other-Config", TotalLines: 17, MissedLines: 12},
				{Name: "Some-Config", TotalLines: 77, MissedLines: 7},
			},
			TotalCoverage: common.CoverageSummary{TotalLines: 17, MissedLines: 12},
		},
	}

	testutils.AssertDeepEqual(t, ir, i.GetResults())
}

// Tests the defaultUntar function, in that it behaves properly on a tar file that
// mimics what the Coverage bots produce.
func TestUntarDefaultStructure(t *testing.T) {
	// MediumTest because we write to disk
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	ctx := context.Background()

	tarpath, err := testutils.TestDataDir()
	assert.NoError(t, err, "Problem with getting TestDataDir")
	tarpath = path.Join(tarpath, "Sample-Config.html.tar")
	// The sample tar folder has the same directory structure that comes off
	// the Linux coverage bots.

	err = defaultUnTar(ctx, tarpath, path.Join(tpath, "SomeFolder"))
	assert.NoError(t, err, "Problem untarring")

	assertFilesExist(t, path.Join(tpath, "SomeFolder"), "bar.html", "foo.html")
	assertFilesExist(t, path.Join(tpath, "SomeFolder", "coverage", "mnt", "pd0", "work", "skia", "dm"), "alpha.cpp")
}

// Tests the coverageData struct and its various operations.
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
		sourceLines: map[int]string{
			1: "alpha",
			2: "beta",
			3: "gamma",
		},
	}
	assert.Equal(t, 6, c1.TotalExecutable())
	assert.Equal(t, 3, c1.MissedExecutable())
	c2 := coverageData{
		executableLines: map[int]bool{
			1: false,
			2: true,
			3: true,
			4: false,
			5: false,
			6: true,
		},
		sourceLines: map[int]string{
			1: "alpha",
			2: "beta",
			3: "gamma",
		},
	}
	assert.Equal(t, 6, c2.TotalExecutable())
	assert.Equal(t, 3, c2.MissedExecutable())
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
		sourceLines: map[int]string{
			1: "alpha",
			2: "beta",
			3: "gamma",
		},
	}
	assert.Equal(t, 8, expected.TotalExecutable())
	assert.Equal(t, 3, expected.MissedExecutable())

	testutils.AssertDeepEqual(t, expected, c1.Union(&c2))
	testutils.AssertDeepEqual(t, expected, c2.Union(&c1))
}

// Tests the parsing logic in coverageData for output produced by LLVM 5
func TestCoverageDataParsingLLVM5(t *testing.T) {
	testutils.SmallTest(t)
	contents := testutils.MustReadFile("some-config.main.cpp")
	expectedHTML := testutils.MustReadFile("someconfig.html")
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
	parsed := parseLinesCovered(contents)
	testutils.AssertDeepEqual(t, expected.executableLines, parsed.executableLines)
	actualHTML, err := parsed.ToHTMLPage(CoverageFileData{
		FileName: "test.cpp",
		Commit:   "adbde2143",
		JobName:  "Combined Report",
	})
	assert.NoError(t, err)
	assert.Equal(t, 29, parsed.TotalExecutable())
	assert.Equal(t, 7, parsed.MissedExecutable())
	assert.Equal(t, 44, parsed.TotalSource())

	assert.Equal(t, expectedHTML, actualHTML)
}

func calculateTotalCoverageSetup(t *testing.T, tpath string) {
	// Set up a directory structure like what the coverage data looks like
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
}

// Tests that coverage of two folders (one per commit) is properly joined together
// and analyzed. In this example, the two folders share one file (main.cpp) and have
// two different headers that were "run".
func TestCalculateTotalCoverage(t *testing.T) {
	testutils.MediumTest(t)
	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	calculateTotalCoverageSetup(t, tpath)

	tc, err := defaultCalculateTotalCoverage(renderInfo{}, path.Join(tpath, "Some-Config", "coverage"), path.Join(tpath, "Other-Config", "coverage"))
	assert.NoError(t, err)

	expected := common.CoverageSummary{
		TotalLines:  36, // Computed by hand
		MissedLines: 7,
	}

	testutils.AssertDeepEqual(t, expected, tc)
}

func read(t *testing.T, path string) string {
	contents, err := ioutil.ReadFile(path)
	assert.NoError(t, err)
	return string(contents)
}

func TestCalculateTotalCoverageOutputHTML(t *testing.T) {
	testutils.MediumTest(t)
	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	calculateTotalCoverageSetup(t, tpath)

	ri := renderInfo{
		outputPath: path.Join(tpath, "Combined"),
		commit:     "98776ab3",
		jobName:    "Combined Unit Test",
	}
	_, err := defaultCalculateTotalCoverage(ri, path.Join(tpath, "Some-Config", "coverage"), path.Join(tpath, "Other-Config", "coverage"))
	assert.NoError(t, err)

	assert.Equal(t, testutils.MustReadFile("combined.main.cpp.html"),
		read(t, path.Join(ri.outputPath, "coverage", "foo", "main.cpp.html")), "main.cpp.html differs")
	assert.Equal(t, testutils.MustReadFile("combined.one-header.h.html"),
		read(t, path.Join(ri.outputPath, "coverage", "foo", "one-header.h.html")), "one-header.h.html differs")
	assert.Equal(t, testutils.MustReadFile("combined.two-header.h.html"),
		read(t, path.Join(ri.outputPath, "coverage", "foo", "two-header.h.html")), "two-header.h.html differs")
	assert.Equal(t, testutils.MustReadFile("combined.index.html"),
		read(t, path.Join(ri.outputPath, "index.html")), "index.html differs")
}
