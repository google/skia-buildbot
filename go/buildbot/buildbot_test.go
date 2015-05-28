package buildbot

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
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

	testHttpClient = mockhttpclient.New(map[string][]byte{
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
	})
)

// clearDB initializes the database, upgrading it if needed, and removes all
// data to ensure that the test begins with a clean slate. Returns a MySQLTestDatabase
// which must be closed after the test finishes.
func clearDB(t *testing.T) *testutil.MySQLTestDatabase {
	failMsg := "Database initialization failed. Do you have the test database set up properly?  Details: %v"

	// Set up the database.
	testDb := testutil.SetupMySQLTestDatabase(t, migrationSteps)

	conf := testutil.LocalTestDatabaseConfig(migrationSteps)
	var err error
	DB, err = sqlx.Open("mysql", conf.MySQLString())
	assert.Nil(t, err, failMsg)

	return testDb
}

// testGetBuildFromMaster is a helper function which pretends to load JSON data
// from a build master and decodes it into a Build object.
func testGetBuildFromMaster(repos *repoMap) (*Build, error) {
	httpClient = testHttpClient
	return getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX660-x86-Release", 721, repos)
}

// TestGetBuildFromMaster verifies that we can load JSON data from the build master and
// decode it into a Build object.
func TestGetBuildFromMaster(t *testing.T) {
	testutils.SkipIfShort(t)
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	// Default, complete build.
	_, err = testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	// Incomplete build.
	_, err = getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repos)
	assert.Nil(t, err)
}

// TestBuildJsonSerialization verifies that we can serialize a build to JSON
// and back without losing or corrupting the data.
func TestBuildJsonSerialization(t *testing.T) {
	testutils.SkipIfShort(t)
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	b1, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	bytes, err := json.Marshal(b1)
	assert.Nil(t, err)
	b2 := &Build{}
	assert.Nil(t, json.Unmarshal(bytes, b2))
	testutils.AssertDeepEqual(t, b1, b2)
}

// TestFindCommitsForBuild verifies that findCommitsForBuild correctly obtains
// the list of commits which were newly built in a given build.
func TestFindCommitsForBuild(t *testing.T) {
	testutils.SkipIfShort(t)
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
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
		b, err := testGetBuildFromMaster(repos)
		assert.Nil(t, err)
		b.GotRevision = tc.GotRevision
		b.Number = buildNum
		c, err := findCommitsForBuild(b, repo)
		assert.Nil(t, err)
		assert.True(t, util.SSliceEqual(c, tc.Expected), fmt.Sprintf("Commits for build do not match expectation.\nGot:  %v\nWant: %v", c, tc.Expected))
		b.Commits = c
		assert.Nil(t, b.ReplaceIntoDB())
	}
}

// dbSerializeAndCompare is a helper function used by TestDbBuild which takes
// a Build object, writes it into the database, reads it back out, and compares
// the structs. Returns any errors encountered including a comparison failure.
func dbSerializeAndCompare(t *testing.T, b1 *Build) {
	assert.Nil(t, b1.ReplaceIntoDB())
	b2, err := GetBuildFromDB(b1.Builder, b1.Master, b1.Number)
	assert.Nil(t, err)

	// Force the IDs to be equal, since the DB assigns ID, and we
	// don't care to try to predict them.
	b2.Id = b1.Id
	assert.Equal(t, len(b1.Steps), len(b2.Steps), "Got incorrect number of steps.")
	for i, s := range b2.Steps {
		s.Id = b1.Steps[i].Id
	}

	testutils.AssertDeepEqual(t, b1, b2)
}

// testBuildDbSerialization verifies that we can write a build to the DB and
// pull it back out without losing or corrupting the data.
func testBuildDbSerialization(t *testing.T) {
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	// Test case: an empty build. Tests null and empty values.
	emptyTime := 0.0
	emptyBuild := &Build{
		Steps:   []*BuildStep{},
		Times:   []float64{emptyTime, emptyTime},
		Commits: []string{},
	}

	// Test case: a completely filled-out build.
	buildFromFullJson, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)

	testCases := []*Build{emptyBuild, buildFromFullJson}
	for _, b := range testCases {
		dbSerializeAndCompare(t, b)
	}
}

// testUnfinishedBuild verifies that we can write a build which is not yet
// finished, load the build back from the database, and update it when it
// finishes.
func testUnfinishedBuild(t *testing.T) {
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	// Obtain and insert an unfinished build.
	httpClient = testHttpClient
	b, err := getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repos)
	assert.Nil(t, err)
	assert.False(t, b.IsFinished(), fmt.Errorf("Unfinished build thinks it's finished!"))
	dbSerializeAndCompare(t, b)

	// Ensure that the build is found by getUnfinishedBuilds.
	unfinished, err := getUnfinishedBuilds()
	assert.Nil(t, err)
	found := false
	for _, u := range unfinished {
		if u.Master == b.Master && u.Builder == b.Builder && u.Number == b.Number {
			found = true
			break
		}
	}
	assert.True(t, found, "Unfinished build was not found by getUnfinishedBuilds!")

	// Add another step to the build to "finish" it, ensure that we can
	// retrieve it as expected.
	b.Finished = b.Started + 1000
	b.Times[1] = b.Finished
	stepStarted := b.Started + 500
	s := &BuildStep{
		BuildID:    b.Id,
		Name:       "LastStep",
		Times:      []float64{stepStarted, b.Finished},
		Number:     len(b.Steps),
		Results:    0,
		ResultsRaw: []interface{}{0.0, []interface{}{}},
		Started:    b.Started + 500.0,
		Finished:   b.Finished,
	}
	b.Steps = append(b.Steps, s)
	assert.True(t, b.IsFinished(), "Finished build thinks it's unfinished!")
	dbSerializeAndCompare(t, b)

	// Ensure that the finished build is NOT found by getUnfinishedBuilds.
	unfinished, err = getUnfinishedBuilds()
	assert.Nil(t, err)
	found = false
	for _, u := range unfinished {
		if u.Master == b.Master && u.Builder == b.Builder && u.Number == b.Number {
			found = true
			break
		}
	}
	assert.False(t, found, "Finished build was found by getUnfinishedBuilds!")
}

// testLastProcessedBuilds verifies that getLastProcessedBuilds gives us
// the expected result.
func testLastProcessedBuilds(t *testing.T) {
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	build, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)

	// Ensure that we get the right number for not-yet-processed
	// builder/master pair.
	builds, err := getLastProcessedBuilds()
	assert.Nil(t, err)
	if builds == nil || len(builds) != 0 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned an unacceptable value for no builds: %v", builds))
	}

	// Ensure that we get the right number for a single already-processed
	// builder/master pair.
	assert.Nil(t, build.ReplaceIntoDB())
	builds, err = getLastProcessedBuilds()
	assert.Nil(t, err)
	if builds == nil || len(builds) != 1 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned incorrect number of results: %v", builds))
	}
	if builds[0].Master != build.Master || builds[0].Builder != build.Builder || builds[0].Number != build.Number {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned the wrong build: %v", builds[0]))
	}

	// Ensure that we get the correct result for multiple builders.
	build2, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	build2.Builder = "Other-Builder"
	build2.Number = build.Number + 10
	assert.Nil(t, build2.ReplaceIntoDB())
	builds, err = getLastProcessedBuilds()
	assert.Nil(t, err)
	compareBuildLists := func(expected []*Build, actual []*BuildID) bool {
		if len(expected) != len(actual) {
			return false
		}
		for _, e := range expected {
			found := false
			for _, a := range actual {
				if e.Builder == a.Builder && e.Master == a.Master && e.Number == a.Number {
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
	assert.True(t, compareBuildLists([]*Build{build, build2}, builds), fmt.Sprintf("getLastProcessedBuilds returned incorrect results: %v", builds))

	// Add "older" build, ensure that only the newer ones are returned.
	build3, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	build3.Number -= 10
	assert.Nil(t, build3.ReplaceIntoDB())
	builds, err = getLastProcessedBuilds()
	assert.Nil(t, err)
	assert.True(t, compareBuildLists([]*Build{build, build2}, builds), fmt.Sprintf("getLastProcessedBuilds returned incorrect results: %v", builds))
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

	httpClient = testHttpClient
	actual, err := getLatestBuilds()
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, expected, actual)
}

// testGetUningestedBuilds verifies that getUningestedBuilds works as expected.
func testGetUningestedBuilds(t *testing.T) {
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	// This builder is no longer found on the master.
	b1, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b1.Master = "client.skia.compile"
	b1.Builder = "My-Builder"
	b1.Number = 115
	b1.Steps = []*BuildStep{}
	assert.Nil(t, b1.ReplaceIntoDB())

	// This builder needs to load a few builds.
	b2, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b2.Master = "client.skia.android"
	b2.Builder = "Perf-Android-Venue8-PowerVR-x86-Release"
	b2.Number = 463
	b2.Steps = []*BuildStep{}
	assert.Nil(t, b2.ReplaceIntoDB())

	// This builder is already up-to-date.
	b3, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b3.Master = "client.skia.fyi"
	b3.Builder = "Housekeeper-PerCommit"
	b3.Number = 1035
	b3.Steps = []*BuildStep{}
	assert.Nil(t, b3.ReplaceIntoDB())

	// This builder is already up-to-date.
	b4, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b4.Master = "client.skia.android"
	b4.Builder = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	assert.Nil(t, b4.ReplaceIntoDB())

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
	httpClient = testHttpClient
	actual, err := getUningestedBuilds()
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, expected, actual)
}

// testIngestNewBuilds verifies that we can successfully query the masters and
// the database for new and unfinished builds, respectively, and ingest them
// into the database.
func testIngestNewBuilds(t *testing.T) {
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repo, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "skia.git"), false, true)
	assert.Nil(t, err)

	repos := &repoMap{
		repos: map[string]*gitinfo.GitInfo{
			"https://skia.googlesource.com/skia.git": repo,
		},
		workdir: tr.Dir,
	}

	// This builder needs to load a few builds.
	b1, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b1.Master = "client.skia.android"
	b1.Builder = "Perf-Android-Venue8-PowerVR-x86-Release"
	b1.Number = 463
	b1.Steps = []*BuildStep{}
	assert.Nil(t, b1.ReplaceIntoDB())

	// This builder has no new builds, but the last one wasn't finished
	// at its time of ingestion.
	b2, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b2.Master = "client.skia.fyi"
	b2.Builder = "Housekeeper-PerCommit"
	b2.Number = 1035
	b2.Finished = 0.0
	b2.Steps = []*BuildStep{}
	assert.Nil(t, b2.ReplaceIntoDB())

	// Subsequent builders are already up-to-date.
	b3, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b3.Master = "client.skia.fyi"
	b3.Builder = "Housekeeper-Nightly-RecreateSKPs"
	b3.Number = 58
	b3.Steps = []*BuildStep{}
	assert.Nil(t, b3.ReplaceIntoDB())

	b4, err := testGetBuildFromMaster(repos)
	assert.Nil(t, err)
	b4.Master = "client.skia.android"
	b4.Builder = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	assert.Nil(t, b4.ReplaceIntoDB())

	// IngestNewBuilds should process the above Venue8 Perf bot's builds
	// 464-466 as well as Housekeeper-PerCommit's unfinished build #1035.
	assert.Nil(t, ingestNewBuilds(repos))

	// Verify that the expected builds are now in the database.
	expected := []Build{
		Build{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  464,
		},
		Build{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  465,
		},
		Build{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  466,
		},
		Build{
			Master:  b2.Master,
			Builder: b2.Builder,
			Number:  1035,
		},
	}
	for _, e := range expected {
		a, err := GetBuildFromDB(e.Builder, e.Master, e.Number)
		assert.Nil(t, err)
		if !(a.Master == e.Master && a.Builder == e.Builder && a.Number == e.Number) {
			t.Fatalf("Incorrect build was inserted!\n  %s == %s\n  %s == %s\n  %d == %d", a.Master, e.Master, a.Builder, e.Builder, a.Number, e.Number)
		}
		assert.True(t, a.IsFinished(), fmt.Sprintf("Failed to update build properly; it should be finished: %v", a))
	}
}

func TestMySQLBuildDbSerialization(t *testing.T) {
	testutils.SkipIfShort(t)
	testBuildDbSerialization(t)
}

func TestMySQLUnfinishedBuild(t *testing.T) {
	testutils.SkipIfShort(t)
	testUnfinishedBuild(t)
}

func TestMySQLLastProcessedBuilds(t *testing.T) {
	testutils.SkipIfShort(t)
	testLastProcessedBuilds(t)
}

func TestMySQLGetUningestedBuilds(t *testing.T) {
	testutils.SkipIfShort(t)
	testGetUningestedBuilds(t)
}

func TestMySQLIngestNewBuilds(t *testing.T) {
	testutils.SkipIfShort(t)
	testIngestNewBuilds(t)
}
