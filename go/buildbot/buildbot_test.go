package buildbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

var (
	// testJsonInput is raw JSON data as returned from the build master.
	defaultBuild  = testutils.MustReadFile("default_build.json")
	testJsonInput = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "721", -1), "%(gotRevision)s", "051955c355eb742550ddde4eccc3e90b6dc5b887", -1)
	ubuntu0       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "0", -1), "%(gotRevision)s", "4b822ebb7cedd90acbac6a45b897438746973a87", -1)
	ubuntu1       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "1", -1), "%(gotRevision)s", "6d4811eddfa637fac0852c3a0801b773be1f260d", -1)
	ubuntu2       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "2", -1), "%(gotRevision)s", "8d2d1247ef5d2b8a8d3394543df6c12a85881296", -1)
	ubuntu3       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "3", -1), "%(gotRevision)s", "ecb424466a4f3b040586a062c15ed58356f6590e", -1)
	ubuntu4       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "4", -1), "%(gotRevision)s", "06eb2a58139d3ff764f10232d5c8f9362d55e20f", -1)
	ubuntu5       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "5", -1), "%(gotRevision)s", "", -1)
	ubuntu6       = strings.Replace(strings.Replace(defaultBuild, "%(buildnumber)d", "6", -1), "%(gotRevision)s", "d74dfd42a48325ab2f3d4a97278fc283036e0ea4", -1)

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

	testHttpClient = mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		"http://build.chromium.org/p/client.skia/json/builders":                                                                    mockhttpclient.MockGetDialogue([]byte(buildersSkia)),
		"http://build.chromium.org/p/client.skia.android/json/builders":                                                            mockhttpclient.MockGetDialogue([]byte(buildersAndroid)),
		"http://build.chromium.org/p/client.skia.compile/json/builders":                                                            mockhttpclient.MockGetDialogue([]byte(buildersCompile)),
		"http://build.chromium.org/p/client.skia.fyi/json/builders":                                                                mockhttpclient.MockGetDialogue([]byte(buildersFYI)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/0":                 mockhttpclient.MockGetDialogue([]byte(ubuntu0)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/1":                 mockhttpclient.MockGetDialogue([]byte(ubuntu1)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/2":                 mockhttpclient.MockGetDialogue([]byte(ubuntu2)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/3":                 mockhttpclient.MockGetDialogue([]byte(ubuntu3)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/4":                 mockhttpclient.MockGetDialogue([]byte(ubuntu4)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/5":                 mockhttpclient.MockGetDialogue([]byte(ubuntu5)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/6":                 mockhttpclient.MockGetDialogue([]byte(ubuntu6)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX660-x86-Release/builds/721":               mockhttpclient.MockGetDialogue([]byte(testJsonInput)),
		"http://build.chromium.org/p/client.skia/json/builders/Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind/builds/152": mockhttpclient.MockGetDialogue([]byte(testIncompleteBuild)),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/464":         mockhttpclient.MockGetDialogue([]byte(venue464)),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/465":         mockhttpclient.MockGetDialogue([]byte(venue465)),
		"http://build.chromium.org/p/client.skia.android/json/builders/Perf-Android-Venue8-PowerVR-x86-Release/builds/466":         mockhttpclient.MockGetDialogue([]byte(venue466)),
		"http://build.chromium.org/p/client.skia.fyi/json/builders/Housekeeper-PerCommit/builds/1035":                              mockhttpclient.MockGetDialogue([]byte(housekeeper1035)),
	})
)

type testDB interface {
	DB() DB
	Close(*testing.T)
}

type testLocalDB struct {
	db  DB
	dir string
}

func (d *testLocalDB) DB() DB {
	return d.db
}

func (d *testLocalDB) Close(t *testing.T) {
	assert.NoError(t, d.db.Close())
	if d.dir != "" {
		assert.NoError(t, os.RemoveAll(d.dir))
	}
}

type testRemoteDB struct {
	localDB  DB
	remoteDB DB
	dir      string
}

func (d *testRemoteDB) DB() DB {
	return d.remoteDB
}

func (d *testRemoteDB) Close(t *testing.T) {
	assert.NoError(t, d.remoteDB.Close())
	assert.NoError(t, d.localDB.Close())
	assert.NoError(t, os.RemoveAll(d.dir))
}

// clearDB returns a clean testDB instance which must be closed after the test finishes.
func clearDB(t *testing.T, local bool) testDB {
	tempDir, err := ioutil.TempDir("", "buildbot_test_")
	assert.NoError(t, err)
	localDB, err := NewLocalDB(path.Join(tempDir, "buildbot.db"))
	assert.NoError(t, err)
	if local {
		return &testLocalDB{
			db:  localDB,
			dir: tempDir,
		}
	} else {
		port, err := RunBuildServer(":0", localDB)
		assert.NoError(t, err)

		remoteDB, err := NewRemoteDB(fmt.Sprintf("localhost%s", port))
		assert.NoError(t, err)
		return &testRemoteDB{
			localDB:  localDB,
			remoteDB: remoteDB,
			dir:      tempDir,
		}
	}
}

// testGetBuildFromMaster is a helper function which pretends to load JSON data
// from a build master and decodes it into a Build object.
func testGetBuildFromMaster(repos repograph.Map) (*Build, error) {
	httpClient = testHttpClient
	return getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX660-x86-Release", 721, repos)
}

// TestGetBuildFromMaster verifies that we can load JSON data from the build master and
// decode it into a Build object.
func TestGetBuildFromMaster(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	// Default, complete build.
	_, err = testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	// Incomplete build.
	_, err = getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repos)
	assert.NoError(t, err)
}

// TestBuildJsonSerialization verifies that we can serialize a build to JSON
// and back without losing or corrupting the data.
func TestBuildJsonSerialization(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	b1, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	bytes, err := json.Marshal(b1)
	assert.NoError(t, err)
	b2 := &Build{}
	assert.NoError(t, json.Unmarshal(bytes, b2))
	testutils.AssertDeepEqual(t, b1, b2)
}

// testFindCommitsForBuild verifies that findCommitsForBuild correctly obtains
// the list of commits which were newly built in a given build.
func testFindCommitsForBuild(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	httpClient = testHttpClient
	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
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
	// * | d74dfd42a48325ab2f3d4a97278fc283036e0ea4 C (Build #6)
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
		StoleFrom   int
		Stolen      []string
	}{
		// 0. The first build.
		{
			GotRevision: hashes['B'],
			Expected:    []string{hashes['B']}, // Build #0 is limited to a single commit.
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 1. On a linear set of commits, with at least one previous build.
		{
			GotRevision: hashes['D'],
			Expected:    []string{hashes['D'], hashes['C']},
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 2. The first build on a new branch.
		{
			GotRevision: hashes['G'],
			Expected:    []string{hashes['G']},
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 3. After a merge.
		{
			GotRevision: hashes['F'],
			Expected:    []string{hashes['E'], hashes['H'], hashes['F']},
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 4. One last "normal" build.
		{
			GotRevision: hashes['I'],
			Expected:    []string{hashes['I']},
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 5. No GotRevision.
		{
			GotRevision: "",
			Expected:    []string{},
			StoleFrom:   -1,
			Stolen:      []string{},
		},
		// 6. Steal commits from a previously-ingested build.
		{
			GotRevision: hashes['C'],
			Expected:    []string{hashes['C']},
			StoleFrom:   1,
			Stolen:      []string{hashes['C']},
		},
	}
	master := "client.skia"
	builder := "Test-Ubuntu12-ShuttleA-GTX660-x86-Release"
	for buildNum, tc := range testCases {
		build, err := getBuildFromMaster(master, builder, buildNum, repos)
		assert.NoError(t, err)
		// Sanity check. Make sure we have the GotRevision we expect.
		assert.Equal(t, tc.GotRevision, build.GotRevision)
		gotRevProp, err := build.GetStringProperty("got_revision")
		assert.NoError(t, err)
		assert.Equal(t, tc.GotRevision, gotRevProp)

		assert.NoError(t, IngestBuild(d.DB(), build, repos))

		ingested, err := d.DB().GetBuildFromDB(master, builder, buildNum)
		assert.NoError(t, err)
		assert.NotNil(t, ingested)

		// Double-check the inserted build's GotRevision.
		assert.Equal(t, tc.GotRevision, ingested.GotRevision)
		gotRevProp, err = ingested.GetStringProperty("got_revision")
		assert.NoError(t, err)
		assert.Equal(t, tc.GotRevision, gotRevProp)

		// Verify that we got the build (and commits list) we expect.
		build.Commits = tc.Expected
		testutils.AssertDeepEqual(t, build, ingested)

		// Ensure that we can search by commit to find the build we inserted.
		for _, c := range hashes {
			expectBuild := util.In(c, tc.Expected)
			builds, err := d.DB().GetBuildsForCommits([]string{c}, nil)
			assert.NoError(t, err)
			if expectBuild {
				// Assert that we get the build we inserted.
				assert.Equal(t, 1, len(builds))
				assert.Equal(t, 1, len(builds[c]))
				testutils.AssertDeepEqual(t, ingested, builds[c][0])
			} else {
				// Assert that we didn't get the build we inserted.
				for _, gotBuild := range builds[c] {
					assert.NotEqual(t, ingested.Id(), gotBuild.Id())
				}
			}

			n, err := d.DB().GetBuildNumberForCommit(build.Master, build.Builder, c)
			assert.NoError(t, err)
			if expectBuild {
				assert.Equal(t, buildNum, n)
			} else {
				assert.NotEqual(t, buildNum, n)
			}
		}
	}

	// Extra: ensure that build #6 really stole the commit from #1.
	b, err := d.DB().GetBuildFromDB(master, builder, 1)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	assert.False(t, util.In(hashes['C'], b.Commits), fmt.Sprintf("Expected not to find %s in %v", hashes['C'], b.Commits))
}

// dbSerializeAndCompare is a helper function used by TestDbBuild which takes
// a Build object, writes it into the database, reads it back out, and compares
// the structs. Returns any errors encountered including a comparison failure.
func dbSerializeAndCompare(t *testing.T, d testDB, b1 *Build, ignoreIds bool) {
	assert.NoError(t, d.DB().PutBuild(b1))
	b2, err := d.DB().GetBuildFromDB(b1.Master, b1.Builder, b1.Number)
	assert.NoError(t, err)
	assert.NotNil(t, b2)

	testutils.AssertDeepEqual(t, b1, b2)
}

// testBuildDbSerialization verifies that we can write a build to the DB and
// pull it back out without losing or corrupting the data.
func testBuildDbSerialization(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	// Test case: an empty build. Tests null and empty values.
	emptyBuild := &Build{
		Steps:   []*BuildStep{},
		Commits: []string{},
	}
	emptyBuild.fixup()

	// Test case: a completely filled-out build.
	buildFromFullJson, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)

	testCases := []*Build{emptyBuild, buildFromFullJson}
	for _, b := range testCases {
		dbSerializeAndCompare(t, d, b, true)
	}
}

// testUnfinishedBuild verifies that we can write a build which is not yet
// finished, load the build back from the database, and update it when it
// finishes.
func testUnfinishedBuild(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	// Obtain and insert an unfinished build.
	httpClient = testHttpClient
	b, err := getBuildFromMaster("client.skia", "Test-Ubuntu12-ShuttleA-GTX550Ti-x86_64-Release-Valgrind", 152, repos)
	assert.NoError(t, err)
	assert.False(t, b.IsFinished(), "Unfinished build thinks it's finished!")
	dbSerializeAndCompare(t, d, b, true)

	// Ensure that the build is found by GetUnfinishedBuilds.
	unfinished, err := d.DB().GetUnfinishedBuilds(b.Master)
	assert.NoError(t, err)
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
	b.Finished = b.Started.Add(30 * time.Second)
	stepStarted := b.Started.Add(500 * time.Millisecond)
	s := &BuildStep{
		Name:     "LastStep",
		Number:   len(b.Steps),
		Results:  0,
		Started:  stepStarted,
		Finished: b.Finished,
	}
	b.Steps = append(b.Steps, s)
	assert.True(t, b.IsFinished(), "Finished build thinks it's unfinished!")
	dbSerializeAndCompare(t, d, b, true)

	// Ensure that the finished build is NOT found by getUnfinishedBuilds.
	unfinished, err = d.DB().GetUnfinishedBuilds(b.Master)
	assert.NoError(t, err)
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
func testLastProcessedBuilds(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	build, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)

	// Ensure that we get the right number for not-yet-processed
	// builder/master pair.
	builds, err := d.DB().GetLastProcessedBuilds(build.Master)
	assert.NoError(t, err)
	if builds == nil || len(builds) != 0 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned an unacceptable value for no builds: %v", builds))
	}

	// Ensure that we get the right number for a single already-processed
	// builder/master pair.
	assert.NoError(t, d.DB().PutBuild(build))
	builds, err = d.DB().GetLastProcessedBuilds(build.Master)
	assert.NoError(t, err)
	if builds == nil || len(builds) != 1 {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned incorrect number of results: %v", builds))
	}
	m, b, n, err := ParseBuildID(builds[0])
	assert.NoError(t, err)
	if m != build.Master || b != build.Builder || n != build.Number {
		t.Fatal(fmt.Errorf("getLastProcessedBuilds returned the wrong build: %v", builds[0]))
	}

	// Ensure that we get the correct result for multiple builders.
	build2, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	build2.Builder = "Other-Builder"
	build2.Number = build.Number + 10
	assert.NoError(t, d.DB().PutBuild(build2))
	builds, err = d.DB().GetLastProcessedBuilds(build.Master)
	assert.NoError(t, err)
	compareBuildLists := func(expected []*Build, actual []BuildID) bool {
		if len(expected) != len(actual) {
			return false
		}
		for _, e := range expected {
			found := false
			for _, a := range actual {
				m, b, n, err := ParseBuildID(a)
				assert.NoError(t, err)
				if e.Builder == b && e.Master == m && e.Number == n {
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
	assert.NoError(t, err)
	build3.Number -= 10
	assert.NoError(t, d.DB().PutBuild(build3))
	builds, err = d.DB().GetLastProcessedBuilds(build.Master)
	assert.NoError(t, err)
	assert.True(t, compareBuildLists([]*Build{build, build2}, builds), fmt.Sprintf("getLastProcessedBuilds returned incorrect results: %v", builds))
}

// TestGetLatestBuilds verifies that getLatestBuilds gives us
// the expected results.
func TestGetLatestBuilds(t *testing.T) {
	testutils.MediumTest(t)
	// Note: Masters with no builders shouldn't be in the map.
	expected := map[string]map[string]int{
		"client.skia.fyi": {
			"Housekeeper-PerCommit":            1035,
			"Housekeeper-Nightly-RecreateSKPs": 58,
		},
		"client.skia.android": {
			"Perf-Android-Venue8-PowerVR-x86-Release": 466,
			"Test-Android-Venue8-PowerVR-x86-Debug":   532,
		},
	}

	httpClient = testHttpClient
	for m, e := range expected {
		actual, err := getLatestBuilds(m)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, e, actual)
	}
}

// testGetUningestedBuilds verifies that getUningestedBuilds works as expected.
func testGetUningestedBuilds(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	// This builder is no longer found on the master.
	b1, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b1.Master = "client.skia.compile"
	b1.Builder = "My-Builder"
	b1.Number = 115
	b1.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b1))

	// This builder needs to load a few builds.
	b2, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b2.Master = "client.skia.android"
	b2.Builder = "Perf-Android-Venue8-PowerVR-x86-Release"
	b2.Number = 463
	b2.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b2))

	// This builder is already up-to-date.
	b3, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b3.Master = "client.skia.fyi"
	b3.Builder = "Housekeeper-PerCommit"
	b3.Number = 1035
	b3.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b3))

	// This builder is already up-to-date.
	b4, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b4.Master = "client.skia.android"
	b4.Builder = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b4))

	// Expectations. If the master or builder has no uningested builds,
	// we expect it not to be in the results, even with an empty map/slice.
	expected := map[string]map[string][]int{
		"client.skia.fyi": {
			"Housekeeper-Nightly-RecreateSKPs": { // No already-ingested builds.
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58,
			},
		},
		"client.skia.android": { // Some already-ingested builds.
			"Perf-Android-Venue8-PowerVR-x86-Release": {
				464, 465, 466,
			},
		},
	}
	httpClient = testHttpClient
	for m, e := range expected {
		actual, err := getUningestedBuilds(d.DB(), m)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, e, actual)
	}
}

// testIngestNewBuilds verifies that we can successfully query the masters and
// the database for new and unfinished builds, respectively, and ingest them
// into the database.
func testIngestNewBuilds(t *testing.T, local bool) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repo, err := repograph.New(&git.Repo{GitDir: git.GitDir(path.Join(tr.Dir, "skia.git"))})
	assert.NoError(t, err)
	repos := repograph.Map{
		common.REPO_SKIA: repo,
	}

	// This builder needs to load a few builds.
	b1, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b1.Master = "client.skia.android"
	b1.Builder = "Perf-Android-Venue8-PowerVR-x86-Release"
	b1.Number = 463
	b1.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b1))

	// This builder has no new builds, but the last one wasn't finished
	// at its time of ingestion.
	b2, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b2.Master = "client.skia.fyi"
	b2.Builder = "Housekeeper-PerCommit"
	b2.Number = 1035
	b2.Finished = util.TimeUnixZero
	b2.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b2))

	// Subsequent builders are already up-to-date.
	b3, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b3.Master = "client.skia.fyi"
	b3.Builder = "Housekeeper-Nightly-RecreateSKPs"
	b3.Number = 58
	b3.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b3))

	b4, err := testGetBuildFromMaster(repos)
	assert.NoError(t, err)
	b4.Master = "client.skia.android"
	b4.Builder = "Test-Android-Venue8-PowerVR-x86-Debug"
	b4.Number = 532
	b4.Steps = []*BuildStep{}
	assert.NoError(t, d.DB().PutBuild(b4))

	// IngestNewBuilds should process the above Venue8 Perf bot's builds
	// 464-466 as well as Housekeeper-PerCommit's unfinished build #1035.
	for _, m := range MASTER_NAMES {
		assert.NoError(t, ingestNewBuilds(d.DB(), m, repos))
	}
	// Wait for the builds to be inserted.
	time.Sleep(1500 * time.Millisecond)

	// Verify that the expected builds are now in the database.
	expected := []Build{
		{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  464,
		},
		{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  465,
		},
		{
			Master:  b1.Master,
			Builder: b1.Builder,
			Number:  466,
		},
		{
			Master:  b2.Master,
			Builder: b2.Builder,
			Number:  1035,
		},
	}
	for _, e := range expected {
		a, err := d.DB().GetBuildFromDB(e.Master, e.Builder, e.Number)
		assert.NoError(t, err)
		assert.NotNil(t, a)
		if !(a.Master == e.Master && a.Builder == e.Builder && a.Number == e.Number) {
			t.Fatalf("Incorrect build was inserted!\n  %s == %s\n  %s == %s\n  %d == %d", a.Master, e.Master, a.Builder, e.Builder, a.Number, e.Number)
		}
		assert.True(t, a.IsFinished(), fmt.Sprintf("Failed to update build properly; it should be finished: %v", a))
	}
}

// testBuildKeyOrdering ensures that we properly sort build keys so that the
// build numbers are strictly ascending.
func testBuildKeyOrdering(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	b := "Test-Builder"
	m := "test.master"
	assert.NoError(t, d.DB().PutBuild(&Build{
		Builder: b,
		Master:  m,
		Number:  1,
	}))
	assert.NoError(t, d.DB().PutBuild(&Build{
		Builder: b,
		Master:  m,
		Number:  10,
	}))
	assert.NoError(t, d.DB().PutBuild(&Build{
		Builder: b,
		Master:  m,
		Number:  2,
	}))
	max, err := d.DB().GetMaxBuildNumber(m, b)
	assert.NoError(t, err)
	assert.Equal(t, 10, max)
}

// testBuilderComments ensures that we properly handle builder comments.
func testBuilderComments(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	b := "Perf-Android-Venue8-PowerVR-x86-Release"
	u := "me@google.com"

	test := func(expect []*BuilderComment) {
		c, err := d.DB().GetBuilderComments(b)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect, c)
	}

	// Check empty.
	test([]*BuilderComment{})

	// Add a comment.
	c1 := &BuilderComment{
		Builder:       b,
		User:          u,
		Timestamp:     time.Now(),
		Flaky:         true,
		IgnoreFailure: true,
		Message:       "Here's a message!",
	}
	assert.NoError(t, d.DB().PutBuilderComment(c1))
	c1.Id = 1
	test([]*BuilderComment{c1})

	// Ensure that we can't update a comment that doesn't exist.
	c2 := &BuilderComment{
		Id:            30,
		Builder:       b,
		User:          u,
		Timestamp:     time.Now(),
		Flaky:         false,
		IgnoreFailure: true,
		Message:       "This comment doesn't exist, but it has an ID!",
	}
	assert.NotNil(t, d.DB().PutBuilderComment(c2))
	test([]*BuilderComment{c1})

	// Fix the second comment, insert it, and ensure that we get both comments back.
	c2.Id = 0
	assert.NoError(t, d.DB().PutBuilderComment(c2))
	c2.Id = 2
	test([]*BuilderComment{c1, c2})

	// Ensure that we don't get the two comments for a bot which has the first bot as a prefix.
	c, err := d.DB().GetBuilderComments("Perf-Android-Venue8-PowerVR-x86-Release-Suffix")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*BuilderComment{}, c)

	// Ensure that we don't get the two comments for a bot which is a prefix of the first bot.
	c, err = d.DB().GetBuilderComments("Perf-Android-Venue8-PowerVR-x86")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*BuilderComment{}, c)

	// Delete the first comment.
	assert.NoError(t, d.DB().DeleteBuilderComment(c1.Id))
	test([]*BuilderComment{c2})

	// Try to re-insert the first comment. Ensure that we can't.
	assert.NotNil(t, d.DB().PutBuilderComment(c1))

	// Try to delete the no-longer-existing first comment.
	assert.NotNil(t, d.DB().DeleteBuilderComment(c1.Id))
}

// testCommitComments ensures that we properly handle builder comments.
func testCommitComments(t *testing.T, local bool) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t, local)
	defer d.Close(t)

	c := "3e9eff3518fe26312c0e1f5bd5f49e17cf270d9a"
	u := "me@google.com"

	test := func(expect []*CommitComment) {
		comments, err := d.DB().GetCommitComments(c)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect, comments)
	}

	// Check empty.
	test([]*CommitComment{})

	// Add a comment.
	c1 := &CommitComment{
		Commit:        c,
		User:          u,
		Timestamp:     time.Now(),
		IgnoreFailure: true,
		Message:       "Here's a message!",
	}
	assert.NoError(t, d.DB().PutCommitComment(c1))
	c1.Id = 1
	test([]*CommitComment{c1})

	// Ensure that we can't update a comment that doesn't exist.
	c2 := &CommitComment{
		Id:        30,
		Commit:    c,
		User:      u,
		Timestamp: time.Now(),
		Message:   "This comment doesn't exist, but it has an ID!",
	}
	assert.NotNil(t, d.DB().PutCommitComment(c2))
	test([]*CommitComment{c1})

	// Fix the second comment, insert it, and ensure that we get both comments back.
	c2.Id = 0
	assert.NoError(t, d.DB().PutCommitComment(c2))
	c2.Id = 2
	test([]*CommitComment{c1, c2})

	// Ensure that we don't get the two comments for a commit which has the first commit as a prefix.
	comments, err := d.DB().GetCommitComments("3e9eff3518fe26312c0e1f5bd5f49e17cf270d9asuffix")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*CommitComment{}, comments)

	// Ensure that we don't get the two comments for a commit which is a prefix of the first commit.
	comments, err = d.DB().GetCommitComments("3e9eff3518fe26312c0e1f5bd5f49e17cf27")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*CommitComment{}, comments)

	// Delete the first comment.
	assert.NoError(t, d.DB().DeleteCommitComment(c1.Id))
	test([]*CommitComment{c2})

	// Try to re-insert the first comment. Ensure that we can't.
	assert.NotNil(t, d.DB().PutCommitComment(c1))

	// Try to delete the no-longer-existing first comment.
	assert.NotNil(t, d.DB().DeleteCommitComment(c1.Id))
}

func TestLocalFindCommitsForBuild(t *testing.T) {
	testFindCommitsForBuild(t, true)
}

func TestRemoteFindCommitsForBuild(t *testing.T) {
	testFindCommitsForBuild(t, false)
}

func TestLocalBuildDbSerialization(t *testing.T) {
	testBuildDbSerialization(t, true)
}

func TestRemoteBuildDbSerialization(t *testing.T) {
	testBuildDbSerialization(t, false)
}

func TestLocalUnfinishedBuild(t *testing.T) {
	testUnfinishedBuild(t, true)
}

func TestRemoteUnfinishedBuild(t *testing.T) {
	testUnfinishedBuild(t, false)
}

func TestLocalLastProcessedBuilds(t *testing.T) {
	testLastProcessedBuilds(t, true)
}

func TestRemoteLastProcessedBuilds(t *testing.T) {
	testLastProcessedBuilds(t, false)
}

func TestLocalGetUningestedBuilds(t *testing.T) {
	testGetUningestedBuilds(t, true)
}

func TestRemoteGetUningestedBuilds(t *testing.T) {
	testGetUningestedBuilds(t, false)
}

func TestLocalIngestNewBuilds(t *testing.T) {
	testIngestNewBuilds(t, true)
}

func TestRemoteIngestNewBuilds(t *testing.T) {
	testIngestNewBuilds(t, false)
}

func TestLocalBuildKeyOrdering(t *testing.T) {
	testBuildKeyOrdering(t, true)
}

func TestRemoteBuildKeyOrdering(t *testing.T) {
	testBuildKeyOrdering(t, false)
}

func TestLocalBuilderComments(t *testing.T) {
	testBuilderComments(t, true)
}

func TestRemoteBuilderComments(t *testing.T) {
	testBuilderComments(t, false)
}

func TestLocalCommitComments(t *testing.T) {
	testCommitComments(t, true)
}

func TestRemoteCommitComments(t *testing.T) {
	testCommitComments(t, false)
}

func TestInt64Serialization(t *testing.T) {
	testutils.SmallTest(t)
	cases := []int64{0, 1, 15, 255, 2047, 4096, 8191, -1}
	for _, c := range cases {
		v, err := bytesToIntBigEndian(intToBytesBigEndian(c))
		assert.NoError(t, err)
		assert.Equal(t, c, v)
	}
	_, err := bytesToIntBigEndian([]byte{1, 2, 3, 4, 5, 6, 7})
	assert.NotNil(t, err)
	_, err = bytesToIntBigEndian([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	assert.NotNil(t, err)
}

func TestBuildIDs(t *testing.T) {
	testutils.SmallTest(t)
	type id struct {
		Master  string
		Builder string
		Number  int
	}
	cases := []id{
		{
			Master:  "my.master",
			Builder: "My-Builder",
			Number:  0,
		},
		{
			Master:  "my.master",
			Builder: "My-Builder",
			Number:  42,
		},
		{
			Master:  "my.master",
			Builder: "My-Builder",
			Number:  -1,
		},
	}

	ids := make([]string, 0, len(cases))
	for _, c := range cases {
		i := MakeBuildID(c.Master, c.Builder, c.Number)
		m, b, n, err := ParseBuildID(i)
		assert.NoError(t, err)
		assert.Equal(t, c.Master, m)
		assert.Equal(t, c.Builder, b)
		assert.Equal(t, c.Number, n)
		ids = append(ids, string(i))
	}
	assert.True(t, sort.StringsAreSorted(ids))
}
