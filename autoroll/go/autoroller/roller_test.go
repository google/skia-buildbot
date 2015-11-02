package autoroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/testutils"
)

// mockRepoManager is a struct used for mocking out the AutoRoller's
// interactions with a RepoManager.
type mockRepoManager struct {
	MockIssueNumber    int64
	MockFullSkiaHashes map[string]string
	MockLastRollRev    string
	MockRolledPast     map[string]bool
	MockSkiaHead       string
	t                  *testing.T
}

// FullSkiaHash returns the full hash of the given short hash or ref in the
// mocked Skia repo.
func (r *mockRepoManager) FullSkiaHash(shortHash string) (string, error) {
	h, ok := r.MockFullSkiaHashes[shortHash]
	if !ok {
		return "", fmt.Errorf("Unknown short hash: %s", shortHash)
	}
	return h, nil
}

// LastRollRev returns the last-rolled Skia commit in the mocked repo.
func (r *mockRepoManager) LastRollRev() string {
	return r.MockLastRollRev
}

// RolledPast determines whether DEPS has rolled past the given commit in the
// mocked repo.
func (r *mockRepoManager) RolledPast(hash string) bool {
	rv, ok := r.MockRolledPast[hash]
	if !ok {
		r.t.Fatal(fmt.Sprintf("Unknown hash: %s", hash))
	}
	return rv
}

// SkiaHead returns the current Skia origin/master branch head in the mocked
// repo.
func (r *mockRepoManager) SkiaHead() string {
	return r.MockSkiaHead
}

// CreateNewRoll pretends to create a new DEPS roll from the mocked repo,
// returning the fake issue number set by the test.
func (r *mockRepoManager) CreateNewRoll(emails, cqExtraTrybots []string, dryRun bool) (int64, error) {
	return r.MockIssueNumber, nil
}

// skiaCommit pretends that a Skia commit has landed.
func (r *mockRepoManager) skiaCommit(hash string) {
	if r.MockFullSkiaHashes == nil {
		r.MockFullSkiaHashes = map[string]string{}
	}
	if r.MockRolledPast == nil {
		r.MockRolledPast = map[string]bool{}
	}
	assert.Equal(r.t, 40, len(hash))
	shortHash := hash[:12]
	r.MockSkiaHead = hash
	r.MockFullSkiaHashes[shortHash] = hash
	r.MockRolledPast[hash] = false
}

// rollerWillUpload sets up expectations for the roller to upload a CL. Returns
// a rietveld.Issue representing the new, in-progress DEPS roll.
func (r *mockRepoManager) rollerWillUpload(rv *mockRietveld, from, to string) *rietveld.Issue {
	emails := []string{"test-sheriff@google.com"}
	// Rietveld API only has millisecond precision.
	now := time.Now().UTC().Round(time.Millisecond)
	description := fmt.Sprintf(`Roll src/third_party/skia/ %s..%s (42 commits).

blah blah
TBR=some-sheriff
`, from[:12], to[:12])
	subject := strings.Split(description, "\n")[0]
	r.MockIssueNumber = nextIssueNum()
	roll := &rietveld.Issue{
		CC:             emails,
		CommitQueue:    true,
		Created:        now,
		CreatedString:  now.Format(rietveld.TIME_FORMAT),
		Description:    description,
		Issue:          r.MockIssueNumber,
		Messages:       []rietveld.IssueMessage{},
		Modified:       now,
		ModifiedString: now.Format(rietveld.TIME_FORMAT),
		Owner:          autoroll.ROLL_AUTHOR,
		Project:        "skia",
		Reviewers:      emails,
		Subject:        subject,
		Patchsets:      []int64{1},
	}
	rv.updateIssue(roll)
	return roll
}

// mockRietveld is a struct used for faking responses from Rietveld.
type mockRietveld struct {
	r       *rietveld.Rietveld
	t       *testing.T
	urlMock *mockhttpclient.URLMock
}

// updateIssue inserts or updates the issue in the mockRietveld.
func (r *mockRietveld) updateIssue(issue *rietveld.Issue) {
	url := fmt.Sprintf("%s/api/%d?messages=true", autoroll.RIETVELD_URL, issue.Issue)
	serialized, err := json.Marshal(issue)
	assert.Nil(r.t, err)
	r.urlMock.MockOnce(url, serialized)
}

// modify changes the last-modified timestamp of the roll and updates it in the
// mockRietveld.
func (r *mockRietveld) modify(issue *rietveld.Issue) {
	now := time.Now().UTC().Round(time.Millisecond)
	issue.Modified = now
	issue.ModifiedString = now.Format(rietveld.TIME_FORMAT)
	r.updateIssue(issue)
}

// rollerWillCloseIssue sets expectations for the roller to close the issue.
func (r *mockRietveld) rollerWillCloseIssue(issue *rietveld.Issue) {
	r.urlMock.MockOnce(fmt.Sprintf("%s/%d/publish", autoroll.RIETVELD_URL, issue.Issue), []byte{})
	r.urlMock.MockOnce(fmt.Sprintf("%s/%d/close", autoroll.RIETVELD_URL, issue.Issue), []byte{})
}

// pretendRollFailed changes the roll to appear to have failed in the
// mockRietveld.
func (r *mockRietveld) pretendRollFailed(issue *rietveld.Issue) {
	issue.CommitQueue = false
	issue.CommitQueueDryRun = false
	r.modify(issue)
}

// pretendRollLanded changes the roll to appear to have succeeded in the
// mockRietveld.
func (r *mockRietveld) pretendRollLanded(rm *mockRepoManager, issue *rietveld.Issue) {
	// Determine what revision we rolled to.
	m := autoroll.ROLL_REV_REGEX.FindStringSubmatch(issue.Subject)
	assert.NotNil(r.t, m)
	assert.Equal(r.t, 2, len(m))
	rolledTo, ok := rm.MockFullSkiaHashes[m[1]]
	assert.True(r.t, ok)
	rm.MockRolledPast[rolledTo] = true
	rm.MockLastRollRev = rolledTo

	issue.Closed = true
	issue.Committed = true
	issue.CommitQueue = false
	issue.CommitQueueDryRun = false
	issue.Description += "\nCommitted: https://chromium.googlesource.com/chromium/src/+/fd01dc2938"
	r.modify(issue)
}

// fakeIssueNum and nextIssueNum() provide auto-incrementing issue numbers.
var fakeIssueNum = int64(100001)

func nextIssueNum() int64 {
	n := fakeIssueNum
	fakeIssueNum++
	return n
}

// checkStatus verifies that we get the expected status from the roller.
func checkStatus(t *testing.T, r *AutoRoller, expectedStatus string, current, last *rietveld.Issue) {
	s := r.GetStatus(true)
	assert.Equal(t, expectedStatus, s.Status)
	assert.Nil(t, s.Error)
	checkRoll := func(t *testing.T, expect *rietveld.Issue, actual *autoroll.AutoRollIssue) {
		if expect != nil {
			assert.NotNil(t, actual)
			ari := autoroll.FromRietveldIssue(expect)
			assert.Nil(t, ari.Validate())
			testutils.AssertDeepEqual(t, ari, actual)
		} else {
			assert.Nil(t, actual)
		}
	}
	checkRoll(t, current, s.CurrentRoll)
	checkRoll(t, last, s.LastRoll)
}

// setup initializes a fake AutoRoller for testing. It returns the working
// directory, AutoRoller instance, URLMock for faking HTTP requests, and an
// rietveld.Issue representing the first CL that was uploaded by the AutoRoller.
func setup(t *testing.T) (string, *AutoRoller, *mockRepoManager, *mockRietveld, *rietveld.Issue) {
	testutils.SkipIfShort(t)

	// Setup mocks.
	urlMock := mockhttpclient.NewURLMock()
	urlMock.Mock(fmt.Sprintf("%s/xsrf_token", autoroll.RIETVELD_URL), []byte("abc123"))
	rv := &mockRietveld{
		r:       rietveld.New(autoroll.RIETVELD_URL, urlMock.Client()),
		t:       t,
		urlMock: urlMock,
	}

	rm := &mockRepoManager{t: t}
	repo_manager.NewRepoManager = func(workdir string, frequency time.Duration) (repo_manager.RepoManager, error) {
		return rm, nil
	}

	workdir, err := ioutil.TempDir("", "test_autoroll_mode_")
	assert.Nil(t, err)

	// Set up more test data.
	initialCommit := "abc1231010101010101010101010101010101010"
	rm.skiaCommit(initialCommit)
	rm.skiaCommit("def4561010101010101010101010101010101010")
	rm.MockLastRollRev = initialCommit
	rm.MockRolledPast[initialCommit] = true
	roll1 := rm.rollerWillUpload(rv, rm.MockLastRollRev, rm.MockSkiaHead)

	// Create the roller.
	roller, err := NewAutoRoller(workdir, []string{}, []string{}, rv.r, time.Hour, time.Hour)
	assert.Nil(t, err)

	// Verify that the bot ran successfully.
	checkStatus(t, roller, STATUS_IN_PROGRESS, roll1, nil)

	return workdir, roller, rm, rv, roll1
}

// TestAutoRollBasic ensures that the typical function of the AutoRoller works
// as expected.
func TestAutoRollBasic(t *testing.T) {
	// setup will initialize the roller and upload a CL.
	workdir, roller, rm, rv, roll1 := setup(t)
	defer func() {
		assert.Nil(t, roller.Close())
		assert.Nil(t, os.RemoveAll(workdir))
	}()

	// Run again. Verify that we let the currently-running roll keep going.
	rv.updateIssue(roll1)
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_IN_PROGRESS, roll1, nil)

	// The roll failed. Verify that we close it and upload another one.
	rv.pretendRollFailed(roll1)
	rv.rollerWillCloseIssue(roll1)
	roll2 := rm.rollerWillUpload(rv, rm.MockLastRollRev, rm.MockSkiaHead)
	assert.Nil(t, roller.doAutoRoll())
	roll1.Closed = true // The roller should have closed this CL.
	checkStatus(t, roller, STATUS_IN_PROGRESS, roll2, roll1)

	// The second roll succeeded. Verify that we're up-to-date.
	rv.pretendRollLanded(rm, roll2)
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_UP_TO_DATE, nil, roll2)

	// Verify that we remain idle.
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_UP_TO_DATE, nil, roll2)
}

// TestAutoRollStop ensures that we can properly stop and restart the
// AutoRoller.
func TestAutoRollStop(t *testing.T) {
	// setup will initialize the roller and upload a CL.
	workdir, roller, rm, rv, roll1 := setup(t)
	defer func() {
		assert.Nil(t, roller.Close())
		assert.Nil(t, os.RemoveAll(workdir))
	}()

	// Stop the bot. Ensure that we close the in-progress roll and don't upload a new one.
	rv.rollerWillCloseIssue(roll1)
	// After the roller closes the CL, it will grab its info from Rietveld
	// and expect the CQ bit to be unset.
	roll1.CommitQueue = false
	rv.updateIssue(roll1)
	// Change the mode, run the bot.
	u := "test@google.com"
	assert.Nil(t, roller.SetMode(autoroll_modes.MODE_STOPPED, u, "Stoppit!"))
	// The roller should have closed the CL.
	roll1.Closed = true
	roll1.CommitQueue = false
	roll1.CommitQueueDryRun = false
	checkStatus(t, roller, STATUS_STOPPED, nil, roll1)

	// Ensure that we don't upload another CL now that we're stopped.
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_STOPPED, nil, roll1)

	// Resume the bot. Ensure that we upload a new CL.
	roll2 := rm.rollerWillUpload(rv, rm.MockLastRollRev, rm.MockSkiaHead)
	assert.Nil(t, roller.SetMode(autoroll_modes.MODE_RUNNING, u, "Resume!"))
	checkStatus(t, roller, STATUS_IN_PROGRESS, roll2, roll1)

	// Pretend the roll landed.
	rv.pretendRollLanded(rm, roll2)
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_UP_TO_DATE, nil, roll2)

	// Stop the roller again.
	rm.skiaCommit("adbcdf1010101010101010101010101010101010")
	assert.Nil(t, roller.SetMode(autoroll_modes.MODE_STOPPED, u, "Stop!"))
	checkStatus(t, roller, STATUS_STOPPED, nil, roll2)

	// Ensure that we don't upload another CL now that we're stopped.
	assert.Nil(t, roller.doAutoRoll())
	checkStatus(t, roller, STATUS_STOPPED, nil, roll2)
}
