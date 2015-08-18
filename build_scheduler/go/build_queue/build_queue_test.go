package build_queue

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	TEST_AUTHOR  = "Eric Boren (borenet@google.com)"
	TEST_BUILDER = "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-BuildBucket"
	TEST_REPO    = "https://skia.googlesource.com/skia.git"
)

var (
	// The test repo is laid out like this:
	//
	// *   06eb2a58139d3ff764f10232d5c8f9362d55e20f I (HEAD, origin/master)
	// *   ecb424466a4f3b040586a062c15ed58356f6590e F
	// |\
	// | * d30286d2254716d396073c177a754f9e152bbb52 H
	// | * 8d2d1247ef5d2b8a8d3394543df6c12a85881296 G
	// * | 67635e7015d74b06c00154f7061987f426349d9f E
	// * | 6d4811eddfa637fac0852c3a0801b773be1f260d D
	// * | d74dfd42a48325ab2f3d4a97278fc283036e0ea4 C
	// |/
	// *   4b822ebb7cedd90acbac6a45b897438746973a87 B
	// *   051955c355eb742550ddde4eccc3e90b6dc5b887 A
	//
	hashes = map[rune]string{
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
)

// clearDB initializes the database, upgrading it if needed, and removes all
// data to ensure that the test begins with a clean slate. Returns a MySQLTestDatabase
// which must be closed after the test finishes.
func clearDB(t *testing.T) *testutil.MySQLTestDatabase {
	failMsg := "Database initialization failed. Do you have the test database set up properly?  Details: %v"

	// Set up the database.
	testDb := testutil.SetupMySQLTestDatabase(t, buildbot.MigrationSteps())

	conf := testutil.LocalTestDatabaseConfig(buildbot.MigrationSteps())
	var err error
	buildbot.DB, err = sqlx.Open("mysql", conf.MySQLString())
	assert.Nil(t, err, failMsg)

	return testDb
}

func TestLambda(t *testing.T) {
	cases := []struct {
		in  float64
		out float64
	}{
		{
			in:  0.0,
			out: math.Inf(1),
		},
		{
			in:  1.0,
			out: 0.0,
		},
		{
			in:  0.5,
			out: 0.028881132523331052,
		},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.out, lambda(tc.in))
	}
}

func TestBuildScoring(t *testing.T) {
	testutils.SkipIfShort(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repos := gitinfo.NewRepoMap(tr.Dir)
	repo, err := repos.Repo(TEST_REPO)
	assert.Nil(t, err)
	assert.Nil(t, repos.Update())

	details := map[string]*gitinfo.LongCommit{}
	for _, h := range hashes {
		d, err := repo.Details(h)
		assert.Nil(t, err)
		details[h] = d
	}

	now := details[hashes['I']].Timestamp.Add(1 * time.Hour)
	build1 := &buildbot.Build{
		GotRevision: hashes['A'],
		Commits:     []string{hashes['A'], hashes['B'], hashes['C']},
	}
	cases := []struct {
		commit        *gitinfo.LongCommit
		build         *buildbot.Build
		expectedScore float64
		lambda        float64
	}{
		// Built at the given commit.
		{
			commit:        details[hashes['A']],
			build:         build1,
			expectedScore: 1.0,
			lambda:        lambda(1.0),
		},
		// Build included the commit.
		{
			commit:        details[hashes['B']],
			build:         build1,
			expectedScore: 1.0 / 3.0,
			lambda:        lambda(1.0),
		},
		// Build included the commit.
		{
			commit:        details[hashes['C']],
			build:         build1,
			expectedScore: 1.0 / 3.0,
			lambda:        lambda(1.0),
		},
		// Build did not include the commit.
		{
			commit:        details[hashes['D']],
			build:         build1,
			expectedScore: -1.0,
			lambda:        lambda(1.0),
		},
		// Build is nil.
		{
			commit:        details[hashes['A']],
			build:         nil,
			expectedScore: -1.0,
			lambda:        lambda(1.0),
		},
		// Same cases, but with lambda set to something interesting.
		// Built at the given commit.
		{
			commit:        details[hashes['A']],
			build:         build1,
			expectedScore: 0.958902488117383,
			lambda:        lambda(0.5),
		},
		// Build included the commit.
		{
			commit:        details[hashes['B']],
			build:         build1,
			expectedScore: 0.3228038362210165,
			lambda:        lambda(0.5),
		},
		// Build included the commit.
		{
			commit:        details[hashes['C']],
			build:         build1,
			expectedScore: 0.32299553133576475,
			lambda:        lambda(0.5),
		},
		// Build did not include the commit.
		{
			commit:        details[hashes['D']],
			build:         build1,
			expectedScore: -0.9690254634399716,
			lambda:        lambda(0.5),
		},
		// Build is nil.
		{
			commit:        details[hashes['A']],
			build:         nil,
			expectedScore: -0.958902488117383,
			lambda:        lambda(0.5),
		},
		// Same cases, but with an even more agressive lambda.
		// Built at the given commit.
		{
			commit:        details[hashes['A']],
			build:         build1,
			expectedScore: 0.756679619938755,
			lambda:        lambda(0.01),
		},
		// Build included the commit.
		{
			commit:        details[hashes['B']],
			build:         build1,
			expectedScore: 0.269316526502904,
			lambda:        lambda(0.01),
		},
		// Build included the commit.
		{
			commit:        details[hashes['C']],
			build:         build1,
			expectedScore: 0.2703808739655321,
			lambda:        lambda(0.01),
		},
		// Build did not include the commit.
		{
			commit:        details[hashes['D']],
			build:         build1,
			expectedScore: -0.8113588225688924,
			lambda:        lambda(0.01),
		},
		// Build is nil.
		{
			commit:        details[hashes['A']],
			build:         nil,
			expectedScore: -0.756679619938755,
			lambda:        lambda(0.01),
		},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.expectedScore, scoreBuild(tc.commit, tc.build, now, tc.lambda))
	}
}

type buildQueueExpect struct {
	bc  *BuildCandidate
	err error
}

func testBuildQueue(t *testing.T, timeDecay24Hr float64, expectations []*buildQueueExpect, testInsert bool) {
	testutils.SkipIfShort(t)

	// Initialize the buildbot database.
	d := clearDB(t)
	defer d.Close(t)

	// Load the test repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	repos := gitinfo.NewRepoMap(tr.Dir)
	repo, err := repos.Repo(TEST_REPO)
	assert.Nil(t, err)
	assert.Nil(t, repos.Update())

	// Create the BuildQueue.
	q, err := NewBuildQueue(PERIOD_FOREVER, repos, DEFAULT_SCORE_THRESHOLD, timeDecay24Hr, []string{TEST_BUILDER})
	assert.Nil(t, err)

	// Fake time.Now()
	details, err := repo.Details(hashes['I'])
	assert.Nil(t, err)
	now := details.Timestamp.Add(1 * time.Hour)

	// Update the queue.
	assert.Nil(t, q.update(now))

	// Ensure that we get the expected BuildCandidate at each step. Insert
	// each BuildCandidate into the buildbot database to simulate actually
	// running builds.
	buildNum := 0
	for _, expected := range expectations {
		bc, err := q.Pop([]string{TEST_BUILDER})
		assert.Equal(t, expected.err, err)
		if err != nil {
			break
		}
		glog.Infof("\n%v\n%v", expected.bc, bc)
		assert.True(t, reflect.DeepEqual(expected.bc, bc))
		if testInsert || buildNum == 0 {
			// Actually insert a build, as if we're really using the scheduler.
			// Do this even if we're not testing insertion, because if we don't,
			// the queue won't know about this builder.
			b := &buildbot.Build{
				Builder:     bc.Builder,
				Master:      "fake",
				Number:      buildNum,
				BuildSlave:  "fake",
				Branch:      "master",
				GotRevision: bc.Commit,
				Repository:  TEST_REPO,
			}
			assert.Nil(t, buildbot.IngestBuild(b, repos))
			buildNum++
			assert.Nil(t, q.update(now))
		}
	}
}

var zeroLambdaExpectations = []*buildQueueExpect{
	// First round: a single build at origin/master.
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['I'],
			Builder: TEST_BUILDER,
			Score:   math.MaxFloat64,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Second round: bisect 9 -> 4 + 5
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['H'],
			Builder: TEST_BUILDER,
			Score:   1.6611111111111108,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Third round: bisect 4 + 5 -> 4 + 3 + 2
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['D'],
			Builder: TEST_BUILDER,
			Score:   1.3666666666666665,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Fourth round: bisect 4 + 3 + 2 -> 3 + 2 + 2 + 2
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['B'],
			Builder: TEST_BUILDER,
			Score:   1.25,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Fifth round: bisect 3 + 2 + 2 + 2 -> 2 + 2 + 2 + 2 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['F'],
			Builder: TEST_BUILDER,
			Score:   0.8333333333333335,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Sixth round: bisect 2 + 2 + 2 + 2 + 1 -> 2 + 2 + 2 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['C'],
			Builder: TEST_BUILDER,
			Score:   0.5,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Seventh round: bisect 2 + 2 + 2 + 1 + 1 + 1 -> 2 + 2 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['G'],
			Builder: TEST_BUILDER,
			Score:   0.5,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Eighth round: bisect 2 + 2 + 1 + 1 + 1 + 1 + 1 -> 2 + 1 + 1 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['E'],
			Builder: TEST_BUILDER,
			Score:   0.5,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Ninth round: bisect 2 + 1 + 1 + 1 + 1 + 1 + 1 + 1 -> 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['A'],
			Builder: TEST_BUILDER,
			Score:   0.5,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Tenth round: All commits individually tested; Score is 0.
	{
		nil,
		ERR_EMPTY_QUEUE,
	},
}

func TestBuildQueueZeroLambdaNoInsert(t *testing.T) {
	testBuildQueue(t, 1.0, zeroLambdaExpectations, false)
}

func TestBuildQueueZeroLambdaInsert(t *testing.T) {
	testBuildQueue(t, 1.0, zeroLambdaExpectations, true)
}

var lambdaExpectations = []*buildQueueExpect{
	// First round: a single build at origin/master.
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['I'],
			Builder: TEST_BUILDER,
			Score:   math.MaxFloat64,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Second round: bisect 9 -> 4 + 5
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['H'],
			Builder: TEST_BUILDER,
			Score:   1.5443883824098559,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Third round: bisect 4 + 5 -> 4 + 3 + 2
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['E'],
			Builder: TEST_BUILDER,
			Score:   1.271875846535702,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Fourth round: bisect 4 + 3 + 2 -> 3 + 2 + 2 + 2
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['B'],
			Builder: TEST_BUILDER,
			Score:   1.1555140209176449,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Fifth round: bisect 3 + 2 + 2 + 2 -> 2 + 2 + 2 + 2 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['D'],
			Builder: TEST_BUILDER,
			Score:   0.7746079616627588,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Sixth round: bisect 2 + 2 + 2 + 2 + 1 -> 2 + 2 + 2 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['F'],
			Builder: TEST_BUILDER,
			Score:   0.46716910846026205,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Seventh round: bisect 2 + 2 + 2 + 1 + 1 + 1 -> 2 + 2 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['G'],
			Builder: TEST_BUILDER,
			Score:   0.46518052365376716,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Eighth round: bisect 2 + 2 + 1 + 1 + 1 + 1 + 1 -> 2 + 1 + 1 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['C'],
			Builder: TEST_BUILDER,
			Score:   0.464730147870253,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Ninth round: bisect 2 + 1 + 1 + 1 + 1 + 1 + 1 + 1 -> 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1
	{
		&BuildCandidate{
			Author:  TEST_AUTHOR,
			Commit:  hashes['A'],
			Builder: TEST_BUILDER,
			Score:   0.4535775776740314,
			Repo:    TEST_REPO,
		},
		nil,
	},
	// Tenth round: All commits individually tested; Score is 0.
	{
		nil,
		ERR_EMPTY_QUEUE,
	},
}

func TestBuildQueueLambdaNoInsert(t *testing.T) {
	testBuildQueue(t, 0.2, lambdaExpectations, false)
}

func TestBuildQueueLambdaInsert(t *testing.T) {
	testBuildQueue(t, 0.2, lambdaExpectations, true)
}
