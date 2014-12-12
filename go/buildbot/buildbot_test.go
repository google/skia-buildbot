package buildbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/go/testutils"
	"skia.googlesource.com/buildbot.git/go/util"
)

var (
	// testJsonInput is raw JSON data as returned from the build master.
	testJsonInput = testutils.MustReadFile("default_build.json")

	// testIncompleteBuild is JSON data for a not-yet-finished build.
	testIncompleteBuild = testutils.MustReadFile("unfinished_build.json")

	// Results for /json/builders on various masters.
	buildersAndroid = testutils.MustReadFile("builders_android.json")
	buildersCompile = "{}"
	buildersFYI     = testutils.MustReadFile("builders_fyi.json")
	buildersSkia    = "{}"

	// More build data.
	venue464        = testutils.MustReadFile("venue464.json")
	venue465        = testutils.MustReadFile("venue465.json")
	venue466        = testutils.MustReadFile("venue466.json")
	housekeeper1035 = testutils.MustReadFile("housekeeper1035.json")

	// urlMap is a map of URLs to data returned from that URL, used for
	// mocking http.Get.
	urlMap = map[string][]byte{
		"http://build.chromium.org/p/client.skia/json/builders":                                                                    []byte(buildersSkia),
		"http://build.chromium.org/p/client.skia.android/json/builders":                                                            []byte(buildersAndroid),
		"http://build.chromium.org/p/client.skia.compile/json/builders":                                                            []byte(buildersCompile),
		"http://build.chromium.org/p/client.skia.fyi/json/builders":                                                                []byte(buildersFYI),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/721":               []byte(testJsonInput),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind/builds/152": []byte(testIncompleteBuild),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/464":         []byte(venue464),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/465":         []byte(venue465),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/466":         []byte(venue466),
		"http://build.chromium.org/p/client.skia.fyi/json/builders/Housekeeper-PerCommit/builds/1035":                              []byte(housekeeper1035),
	}
)

// clearDB initializes the database, upgrading it if needed, and removes all
// data to ensure that the test begins with a clean slate.
func clearDB(t *testing.T, conf *database.DatabaseConfig) error {
	failMsg := "Database initialization failed. Do you have the test database set up properly?  Details: %v"
	if err := InitDB(conf); err != nil {
		t.Fatalf(failMsg, err)
	}
	tables := []string{
		TABLE_BUILD_REVISIONS,
		TABLE_BUILD_STEPS,
		TABLE_BUILDS,
	}
	// Delete the data.
	for _, table := range tables {
		_, err := DB.Exec(fmt.Sprintf("DELETE FROM %s;", table))
		if err != nil {
			t.Fatalf(failMsg, err)
		}
	}
	return nil
}

// respBodyCloser is a wrapper which lets us pretend to implement io.ReadCloser
// by wrapping a bytes.Reader.
type respBodyCloser struct {
	io.Reader
}

// Close is a stub method which lets us pretend to implement io.ReadCloser.
func (r respBodyCloser) Close() error {
	return nil
}

// testGet is a mocked version of http.Get which returns data stored in the
// urlMap.
func testGet(url string) (*http.Response, error) {
	if data, ok := urlMap[url]; ok {
		return &http.Response{
			Body: &respBodyCloser{bytes.NewReader(data)},
		}, nil
	}
	return nil, fmt.Errorf("No such URL in urlMap!")
}

// testGetBuild is a helper function which pretends to load JSON data from a
// build master and decodes it into a Build object.
func testGetBuildFromMaster(repo *gitinfo.GitInfo) (*Build, error) {
	httpGet = testGet
	return getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX660-x86-Release", 721, repo)
}

// TestGetBuildFromMaster verifies that we can load JSON data from the build master and
// decode it into a Build object.
func TestGetBuildFromMaster(t *testing.T) {
	clearDB(t, ProdDatabaseConfig(true))

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// Default, complete build.
	if _, err := testGetBuildFromMaster(repo); err != nil {
		t.Fatal(err)
	}
	// Incomplete build.
	_, err = getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repo)
	if err != nil {
		t.Fatal(err)
	}
}

// TestBuildJsonSerialization verifies that we can serialize a build to JSON
// and back without losing or corrupting the data.
func TestBuildJsonSerialization(t *testing.T) {
	if err := clearDB(t, ProdDatabaseConfig(true)); err != nil {
		t.Fatal(err)
	}

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	b1, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := json.Marshal(b1)
	if err != nil {
		t.Fatal(err)
	}
	b2 := &Build{}
	if err := json.Unmarshal(bytes, b2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b1, b2) {
		t.Fatalf("Serialization diff:\nIn:  %v\nOut: %v", b1, b2)
	}
}

// TestFindCommitsForBuild verifies that findCommitsForBuild correctly obtains
// the list of commits which were newly built in a given build.
func TestFindCommitsForBuild(t *testing.T) {
	if err := clearDB(t, ProdDatabaseConfig(true)); err != nil {
		t.Fatal(err)
	}

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// The test repo is laid out like this:
	//
	// *   06eb2a58139d3ff764f10232d5c8f9362d55e20f I (HEAD, master, Build #4)
	// *   ecb424466a4f3b040586a062c15ed58356f6590e F (Build #3)
	// |\
	// | * d30286d2254716d396073c177a754f9e152bbb52 H
	// | * 8d2d1247ef5d2b8a8d3394543df6c12a85881296 G (Build #2)
	// * | 67635e7015d74b06c00154f7061987f426349d9f E
	// * | 6d4811eddfa637fac0852c3a0801b773be1f260d D (Build #1)
	// * | d74dfd42a48325ab2f3d4a97278fc283036e0ea4 C
	// |/
	// *   4b822ebb7cedd90acbac6a45b897438746973a87 B (Build #0)
	// *   051955c355eb742550ddde4eccc3e90b6dc5b887 A
	//
	hashes := map[rune]string{
		'A': "051955c355eb742550ddde4eccc3e90b6dc5b887",
		'B': "4b822ebb7cedd90acbac6a45b897438746973a87",
		'C': "d74dfd42a48325ab2f3d4a97278fc283036e0ea4",
		'D': "6d4811eddfa637fac0852c3a0801b773be1f260d",
		'E': "67635e7015d74b06c00154f7061987f426349d9f",
		'F': "ecb424466a4f3b040586a062c15ed58356f6590e",
		'G': "8d2d1247ef5d2b8a8d3394543df6c12a85881296",
		'H': "d30286d2254716d396073c177a754f9e152bbb52",
		'I': "06eb2a58139d3ff764f10232d5c8f9362d55e20f",
	}

	// Test cases. Each test case builds on the previous cases.
	testCases := []struct {
		GotRevision string
		Expected    []string
	}{
		// 0. The first build.
		{
			GotRevision: hashes['B'],
			Expected:    []string{hashes['B'], hashes['A']},
		},
		// 1. On a linear set of commits, with at least one previous build.
		{
			GotRevision: hashes['D'],
			Expected:    []string{hashes['D'], hashes['C']},
		},
		// 2. The first build on a new branch.
		{
			GotRevision: hashes['G'],
			Expected:    []string{hashes['G']},
		},
		// 3. After a merge.
		{
			GotRevision: hashes['F'],
			Expected:    []string{hashes['F'], hashes['E'], hashes['H']},
		},
		// 4. One last "normal" build.
		{
			GotRevision: hashes['I'],
			Expected:    []string{hashes['I']},
		},
		// 5. No GotRevision.
		{
			GotRevision: "",
			Expected:    []string{},
		},
	}
	for buildNum, tc := range testCases {
		b, err := testGetBuildFromMaster(repo)
		if err != nil {
			t.Fatal(err)
		}
		b.GotRevision = tc.GotRevision
		b.Number = buildNum
		c, err := findCommitsForBuild(b, repo)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c, tc.Expected) {
			t.Fatalf("Commits for build do not match expectation.\nGot:  %v\nWant: %v", c, tc.Expected)
		}
		b.Commits = c
		if err := b.ReplaceIntoDB(); err != nil {
			t.Fatal(err)
		}
	}
}

// dbSerializeAndCompare is a helper function used by TestDbBuild which takes
// a Build object, writes it into the database, reads it back out, and compares
// the structs. Returns any errors encountered including a comparison failure.
func dbSerializeAndCompare(b1 *Build) error {
	if err := b1.ReplaceIntoDB(); err != nil {
		return err
	}
	b2, err := GetBuildFromDB(b1.MasterName, b1.BuilderName, b1.Number)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(b1, b2) {
		for i, s := range b1.Steps {
			if !reflect.DeepEqual(s, b2.Steps[i]) {
				glog.Errorf("Not equal:\n %+v\n %+v\n", s, b2.Steps[i])
			}
		}
		return fmt.Errorf("Builds are not equal! Builds:\nExpected: %+v\nActual:   %+v", b1, b2)
	}
	return nil
}

// testBuildDbSerialization verifies that we can write a build to the DB and
// pull it back out without losing or corrupting the data.
func testBuildDbSerialization(t *testing.T, conf *database.DatabaseConfig) {
	clearDB(t, conf)
	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// Test case: an empty build. Tests null and empty values.
	emptyTime := 0.0
	emptyBuild := &Build{
		Steps:   []*BuildStep{},
		Times:   []float64{emptyTime, emptyTime},
		Commits: []string{},
	}

	// Test case: a completely filled-out build.
	buildFromFullJson, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []*Build{emptyBuild, buildFromFullJson}
	for _, b := range testCases {
		if err = dbSerializeAndCompare(b); err != nil {
			t.Fatal(err)
		}
	}
}

// testUnfinishedBuild verifies that we can write a build which is not yet
// finished, load the build back from the database, and update it when it
// finishes.
func testUnfinishedBuild(t *testing.T, conf *database.DatabaseConfig) {
	clearDB(t, conf)
	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// Obtain and insert an unfinished build.
	httpGet = testGet
	b, err := getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repo)
	if err != nil {
		t.Fatal(err)
	}
	if b.IsFinished() {
		t.Fatal(fmt.Errorf("Unfinished build thinks it's finished!"))
	}
	if err := dbSerializeAndCompare(b); err != nil {
		t.Fatal(err)
	}

	// Ensure that the build is found by getUnfinishedBuilds.
	unfinished, err := getUnfinishedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range unfinished {
		if u.MasterName == b.MasterName && u.BuilderName == b.BuilderName && u.Number == b.Number {
			found = true
			break
		}
	}
	if !found {
		t.Fatal(fmt.Errorf("Unfinished build was not found by getUnfinishedBuilds!"))
	}

	// Add another step to the build to "finish" it, ensure that we can
	// retrieve it as expected.
	b.Finished = b.Started + 1000
	b.Times[1] = b.Finished
	stepStarted := b.Started + 500
	s := &BuildStep{
		BuilderName: b.BuilderName,
		MasterName:  b.MasterName,
		BuildNumber: b.Number,
		Name:        "LastStep",
		Times:       []float64{stepStarted, b.Finished},
		Number:      len(b.Steps),
		Results:     0,
		ResultsRaw:  []interface{}{0.0, []interface{}{}},
		Started:     b.Started + 500.0,
		Finished:    b.Finished,
	}
	b.Steps = append(b.Steps, s)
	if !b.IsFinished() {
		t.Fatal(fmt.Errorf("Finished build thinks it's unfinished!"))
	}
	if err := dbSerializeAndCompare(b); err != nil {
		t.Fatal(err)
	}

	// Ensure that the finished build is NOT found by getUnfinishedBuilds.
	unfinished, err = getUnfinishedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, u := range unfinished {
		if u.MasterName == b.MasterName && u.BuilderName == b.BuilderName && u.Number == b.Number {
			found = true
			break
		}
	}
	if found {
		t.Fatal(fmt.Errorf("Finished build was found by getUnfinishedBuilds!"))
	}
}

// testLastProcessedBuilds verifies that getLastProcessedBuilds gives us
// the expected result.
func testLastProcessedBuilds(t *testing.T, conf *database.DatabaseConfig) {
	clearDB(t, conf)
	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	build, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that we get the right number for not-yet-processed
	// builder/master pair.
	builds, err := getLastProcessedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if builds == nil || len(builds) != 0 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned an unacceptable value for no builds: %v", builds))
	}

	// Ensure that we get the right number for a single already-processed
	// builder/master pair.
	if err := build.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}
	builds, err = getLastProcessedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if builds == nil || len(builds) != 1 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned incorrect number of results: %v", builds))
	}
	if builds[0].MasterName != build.MasterName || builds[0].BuilderName != build.BuilderName || builds[0].Number != build.Number {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned the wrong build: %v", builds[0]))
	}

	// Ensure that we get the correct result for multiple builders.
	build2, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	build2.BuilderName = "Other-Builder"
	build2.Number = build.Number + 10
	if err := build2.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}
	builds, err = getLastProcessedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	compareBuildLists := func(expected, actual []*Build) bool {
		if len(expected) != len(actual) {
			return false
		}
		for _, e := range expected {
			found := false
			for _, a := range actual {
				if e.BuilderName == a.BuilderName && e.MasterName == a.MasterName && e.Number == a.Number {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	if !compareBuildLists([]*Build{build, build2}, builds) {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned incorrect results: %v", builds))
	}

	// Add "older" build, ensure that only the newer ones are returned.
	build3, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	build3.Number -= 10
	if err := build3.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}
	builds, err = getLastProcessedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if !compareBuildLists([]*Build{build, build2}, builds) {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned incorrect results: %v", builds))
	}
}

// TestGetLatestBuilds verifies that getLatestBuilds gives us
// the expected results.
func TestGetLatestBuilds(t *testing.T) {
	// Note: Masters with no builders shouldn't be in the map.
	expected := map[string]map[string]int{
		"client.skia.fyi": map[string]int{
			"Housekeeper-PerCommit":            1035,
			"Housekeeper-Nightly-RecreateSKPs": 58,
		},
		"client.skia.android": map[string]int{
			"Perf-Android-Venue8-PowerVR-x86-Release": 466,
			"Test-Android-Venue8-PowerVR-x86-Debug":   532,
		},
	}

	httpGet = testGet
	actual, err := getLatestBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal(fmt.Errorf("getLatestBuilds returned incorrect results: %v", actual))
	}
}

// testGetUningestedBuilds verifies that getUningestedBuilds works as expected.
func testGetUningestedBuilds(t *testing.T, conf *database.DatabaseConfig) {
	// First, insert some builds into the database as a starting point.
	clearDB(t, conf)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// This builder is no longer found on the master.
	b1, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b1.MasterName = "client.skia.compile"
	b1.BuilderName = "My-Builder"
	b1.Number = 115
	b1.Steps = []*BuildStep{}
	if err := b1.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// This builder needs to load a few builds.
	b2, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b2.MasterName = "client.skia.android"
	b2.BuilderName = "Perf-Android-Venue8-PowerVR-x86-Release"
	b2.Number = 463
	b2.Steps = []*BuildStep{}
	if err := b2.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// This builder is already up-to-date.
	b3, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b3.MasterName = "client.skia.fyi"
	b3.BuilderName = "Housekeeper-PerCommit"
	b3.Number = 1035
	b3.Steps = []*BuildStep{}
	if err := b3.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// This builder is already up-to-date.
	b4, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b4.MasterName = "client.skia.android"
	b4.BuilderName = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	if err := b4.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// Expectations. If the master or builder has no uningested builds,
	// we expect it not to be in the results, even with an empty map/slice.
	expected := map[string]map[string][]int{
		"client.skia.fyi": map[string][]int{
			"Housekeeper-Nightly-RecreateSKPs": []int{ // No already-ingested builds.
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58,
			},
		},
		"client.skia.android": map[string][]int{ // Some already-ingested builds.
			"Perf-Android-Venue8-PowerVR-x86-Release": []int{
				464, 465, 466,
			},
		},
	}
	httpGet = testGet
	actual, err := getUningestedBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal(fmt.Errorf("getUningestedBuilds returned incorrect results: %v", actual))
	}
}

// testIngestNewBuilds verifies that we can successfully query the masters and
// the database for new and unfinished builds, respectively, and ingest them
// into the database.
func testIngestNewBuilds(t *testing.T, conf *database.DatabaseConfig) {
	// First, insert some builds into the database as a starting point.
	clearDB(t, conf)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, true)
	if err != nil {
		t.Fatal(err)
	}

	// This builder needs to load a few builds.
	b1, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b1.MasterName = "client.skia.android"
	b1.BuilderName = "Perf-Android-Venue8-PowerVR-x86-Release"
	b1.Number = 463
	b1.Steps = []*BuildStep{}
	if err := b1.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// This builder has no new builds, but the last one wasn't finished
	// at its time of ingestion.
	b2, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b2.MasterName = "client.skia.fyi"
	b2.BuilderName = "Housekeeper-PerCommit"
	b2.Number = 1035
	b2.Finished = 0.0
	b2.Steps = []*BuildStep{}
	if err := b2.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// Subsequent builders are already up-to-date.
	b3, err := testGetBuildFromMaster(repo)
	b3.MasterName = "client.skia.fyi"
	b3.BuilderName = "Housekeeper-Nightly-RecreateSKPs"
	b3.Number = 58
	b3.Steps = []*BuildStep{}
	if err := b3.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	b4, err := testGetBuildFromMaster(repo)
	if err != nil {
		t.Fatal(err)
	}
	b4.MasterName = "client.skia.android"
	b4.BuilderName = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	if err := b4.ReplaceIntoDB(); err != nil {
		t.Fatal(err)
	}

	// IngestNewBuilds should process the above Venue8 Perf bot's builds
	// 464-466 as well as Housekeeper-PerCommit's unfinished build #1035.
	if err := IngestNewBuilds(repo); err != nil {
		t.Fatal(err)
	}

	// Verify that the expected builds are now in the database.
	expected := []Build{
		Build{
			MasterName:  b1.MasterName,
			BuilderName: b1.BuilderName,
			Number:      464,
		},
		Build{
			MasterName:  b1.MasterName,
			BuilderName: b1.BuilderName,
			Number:      465,
		},
		Build{
			MasterName:  b1.MasterName,
			BuilderName: b1.BuilderName,
			Number:      466,
		},
		Build{
			MasterName:  b2.MasterName,
			BuilderName: b2.BuilderName,
			Number:      1035,
		},
	}
	for _, e := range expected {
		a, err := GetBuildFromDB(e.MasterName, e.BuilderName, e.Number)
		if err != nil {
			t.Fatal(err)
		}
		if !(a.MasterName == e.MasterName && a.BuilderName == e.BuilderName && a.Number == e.Number) {
			t.Fatalf("Incorrect build was inserted! %v", a)
		}
		if !a.IsFinished() {
			t.Fatalf("Failed to update build properly; it should be finished: %v", a)
		}
	}
}

func TestSQLiteBuildDbSerialization(t *testing.T) {
	testBuildDbSerialization(t, ProdDatabaseConfig(true))
}

func TestSQLiteUnfinishedBuild(t *testing.T) {
	testUnfinishedBuild(t, ProdDatabaseConfig(true))
}

func TestSQLiteLastProcessedBuilds(t *testing.T) {
	testLastProcessedBuilds(t, ProdDatabaseConfig(true))
}

func TestSQLiteGetUningestedBuilds(t *testing.T) {
	testGetUningestedBuilds(t, ProdDatabaseConfig(true))
}

func TestSQLiteIngestNewBuilds(t *testing.T) {
	testIngestNewBuilds(t, ProdDatabaseConfig(true))
}

// The below MySQL tests require:
// shell> mysql -u root
// mysql> CREATE DATABASE sk_testing;
// mysql> CREATE USER 'test_user'@'localhost';
// mysql> GRANT SELECT,INSERT,UPDATE,DELETE,CREATE,DROP ON sk_testing.* TO 'test_user'@'localhost';
//
// They are skipped when using the -short flag.
func TestMySQLBuildDbSerialization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MySQL tests with -short.")
	}
	testBuildDbSerialization(t, localMySQLTestDatabaseConfig("test_user", ""))
}

func TestMySQLUnfinishedBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MySQL tests with -short.")
	}
	testUnfinishedBuild(t, localMySQLTestDatabaseConfig("test_user", ""))
}

func TestMySQLLastProcessedBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MySQL tests with -short.")
	}
	testLastProcessedBuilds(t, localMySQLTestDatabaseConfig("test_user", ""))
}

func TestMySQLGetUningestedBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MySQL tests with -short.")
	}
	testGetUningestedBuilds(t, localMySQLTestDatabaseConfig("test_user", ""))
}

func TestMySQLIngestNewBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MySQL tests with -short.")
	}
	testIngestNewBuilds(t, localMySQLTestDatabaseConfig("test_user", ""))
}
