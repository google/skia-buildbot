package autoroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	COMMITTED_STR   = "Committed: https://chromium.googlesource.com/chromium/src/+/fd01dc2938"
	FAKE_GERRIT_URL = "https://fake-skia-review.googlesource.com"
)

var noTrybots = []*buildbucket.Build{}

// mockRepoManager is a struct used for mocking out the AutoRoller's
// interactions with a RepoManager.
type mockRepoManager struct {
	updateCount              int
	mockIssueNumber          int64
	mockFullChildHashes      map[string]string
	lastRollRev              string
	rolledPast               map[string]bool
	skiaHead                 string
	sendToGerritDryRunCalled bool
	sendToGerritCQCalled     bool
	mtx                      sync.RWMutex
	t                        *testing.T
}

// Update pretends to update the mockRepoManager.
func (r *mockRepoManager) Update() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.updateCount == 0 {
		return fmt.Errorf("updateCount == 0!")
	}
	r.updateCount--
	return nil
}

// mockUpdate increments the expected Update call count.
func (r *mockRepoManager) mockUpdate() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.updateCount++
}

// assertUpdate asserts that the Update call count is zero.
func (r *mockRepoManager) assertUpdate() {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	assert.Equal(r.t, 0, r.updateCount)
}

// getUpdateCount returns the remaining Update call count.
func (r *mockRepoManager) getUpdateCount() int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.updateCount
}

// FullChildHash returns the full hash of the given short hash or ref in the
// mocked child repo.
func (r *mockRepoManager) FullChildHash(shortHash string) (string, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	h, ok := r.mockFullChildHashes[shortHash]
	if !ok {
		return "", fmt.Errorf("Unknown short hash: %s", shortHash)
	}
	return h, nil
}

// mockFullChildHash adds the given mock hash.
func (r *mockRepoManager) mockFullChildHash(short, long string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.mockFullChildHashes[short] = long
}

// LastRollRev returns the last-rolled child commit in the mocked repo.
func (r *mockRepoManager) LastRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastRollRev
}

// mockLastRollRev fakes the last roll revision.
func (r *mockRepoManager) mockLastRollRev(last string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.lastRollRev = last
}

// RolledPast determines whether DEPS has rolled past the given commit in the
// mocked repo.
func (r *mockRepoManager) RolledPast(hash string) (bool, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	rv, ok := r.rolledPast[hash]
	if !ok {
		r.t.Fatal(fmt.Sprintf("Unknown hash: %s", hash))
	}
	return rv, nil
}

// mockRolledPast pretends that the DEPS has rolled past the given commit.
func (r *mockRepoManager) mockRolledPast(hash string, rolled bool) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.rolledPast[hash] = rolled
}

// NextRollRev returns the revision for the next roll.
func (r *mockRepoManager) NextRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.skiaHead
}

// mockNextRollRev sets the fake child origin/master branch head.
func (r *mockRepoManager) mockNextRollRev(hash string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.skiaHead = hash
}

// CreateNewRoll pretends to create a new DEPS roll from the mocked repo,
// returning the fake issue number set by the test.
func (r *mockRepoManager) CreateNewRoll(from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.mockIssueNumber, nil
}

// mockChildCommit pretends that a child commit has landed.
func (r *mockRepoManager) mockChildCommit(hash string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.mockFullChildHashes == nil {
		r.mockFullChildHashes = map[string]string{}
	}
	if r.rolledPast == nil {
		r.rolledPast = map[string]bool{}
	}
	assert.Equal(r.t, 40, len(hash))
	shortHash := hash[:12]
	r.skiaHead = hash
	r.mockFullChildHashes[shortHash] = hash
	r.rolledPast[hash] = false
}

// rollerWillUpload sets up expectations for the roller to upload a CL. Returns
// a gerrit.ChangeInfo representing the new, in-progress DEPS roll.
func (r *mockRepoManager) rollerWillUpload(rv *mockCodereview, from, to string, tryResults []*buildbucket.Build, dryRun bool) *gerrit.ChangeInfo {
	// Rietveld API only has millisecond precision.
	now := time.Now().UTC().Round(time.Millisecond)
	description := fmt.Sprintf(`Roll src/third_party/skia/ %s..%s (42 commits).

blah blah
TBR=some-sheriff
`, from[:12], to[:12])
	r.mockIssueNumber = rv.nextIssueNum()
	rev := &gerrit.Revision{
		ID:            "1",
		Number:        1,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Created:       now,
	}
	cqLabel := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		cqLabel = gerrit.COMMITQUEUE_LABEL_DRY_RUN
	}
	roll := &gerrit.ChangeInfo{
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Subject:       description,
		ChangeId:      fmt.Sprintf("%d", r.mockIssueNumber),
		Issue:         r.mockIssueNumber,
		Labels: map[string]*gerrit.LabelEntry{
			gerrit.CODEREVIEW_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.CODEREVIEW_LABEL_APPROVE,
					},
				},
			},
			gerrit.COMMITQUEUE_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: cqLabel,
					},
				},
			},
		},
		Owner: &gerrit.Owner{
			Email: "fake-deps-roller@chromium.org",
		},
		Project: "skia",
		Revisions: map[string]*gerrit.Revision{
			"1": rev,
		},
		Patchsets:     []*gerrit.Revision{rev},
		Updated:       now,
		UpdatedString: now.Format(gerrit.TIME_FORMAT),
	}
	rv.updateIssue(roll, tryResults)
	return roll
}

func (r *mockRepoManager) User() string {
	return "test_user"
}

func (r *mockRepoManager) SendToGerritCQ(*gerrit.ChangeInfo, string) error {
	r.sendToGerritCQCalled = true
	return nil
}

func (r *mockRepoManager) SendToGerritDryRun(*gerrit.ChangeInfo, string) error {
	r.sendToGerritDryRunCalled = true
	return nil
}

// mockCodereview is a struct used for faking responses from Rietveld.
type mockCodereview struct {
	fakeIssueNum int64
	g            *gerrit.Gerrit
	t            *testing.T
	urlMock      *mockhttpclient.URLMock
}

// assertMocksEmpty asserts that all of the URLs in the URLMock have been used.
func (r *mockCodereview) assertMocksEmpty() {
	assert.True(r.t, r.urlMock.Empty(), fmt.Sprintf("URLS:\n%s", strings.Join(r.urlMock.List(), "\n")))
}

// mockTrybotResults sets up a fake response to a request for trybot results.
func (r *mockCodereview) mockTrybotResults(issue *gerrit.ChangeInfo, results []*buildbucket.Build) {
	url := fmt.Sprintf("https://cr-buildbucket.appspot.com/api/buildbucket/v1/search?tag=buildset%%3Apatch%%2Fgerrit%%2Ffake-skia-review.googlesource.com%%2F%d%%2F1", issue.Issue)
	serialized, err := json.Marshal(struct {
		Builds []*buildbucket.Build
	}{
		Builds: results,
	})
	assert.NoError(r.t, err)
	r.urlMock.MockOnce(url, mockhttpclient.MockGetDialogue(serialized))
}

// updateIssue inserts or updates the issue in the mockCodereview.
func (r *mockCodereview) updateIssue(issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build) {
	url := fmt.Sprintf("%s/changes/%d/detail?o=ALL_REVISIONS", r.g.Url(0), issue.Issue)
	serialized, err := json.Marshal(issue)
	assert.NoError(r.t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	r.urlMock.MockOnce(url, mockhttpclient.MockGetDialogue(serialized))
	r.mockTrybotResults(issue, tryResults)
}

// modify changes the last-modified timestamp of the roll and updates it in the
// mockCodereview.
func (r *mockCodereview) modify(issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build) {
	now := time.Now().UTC().Round(time.Millisecond)
	issue.Updated = now
	issue.UpdatedString = now.Format(gerrit.TIME_FORMAT)
	r.updateIssue(issue, tryResults)
}

// rollerWillCloseIssue sets expectations for the roller to close the issue.
func (r *mockCodereview) rollerWillCloseIssue(issue *gerrit.ChangeInfo) {
	p := mockhttpclient.MockPostDialogue("application/json", mockhttpclient.DONT_CARE_REQUEST, []byte{})
	url := fmt.Sprintf("%s/a/changes/%d/abandon", r.g.Url(0), issue.Issue)
	r.urlMock.MockOnce(url, p)
}

// rollerWillSwitchDryRun sets expectations for the roller to switch the issue
// into or out of dry run mode.
func (r *mockCodereview) rollerWillSwitchDryRun(issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build, dryRun bool) {
	r.updateIssue(issue, tryResults) // Initial issue update.
	value := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		value = gerrit.COMMITQUEUE_LABEL_DRY_RUN
	}
	issue.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = value
	r.updateIssue(issue, tryResults)
}

// pretendDryRunFinished sets expectations for when the dry run has finished.
func (r *mockCodereview) pretendDryRunFinished(issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build, success bool) {
	result := autoroll.TRYBOT_RESULT_FAILURE
	if success {
		result = autoroll.TRYBOT_RESULT_SUCCESS
	}
	for _, t := range tryResults {
		t.Status = autoroll.TRYBOT_STATUS_COMPLETED
		t.Result = result
	}
	issue.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	r.updateIssue(issue, tryResults) // Initial issue update.

	// The roller will add a comment to the issue and close it if the dry run failed.
	if success {
		p := mockhttpclient.MockPostDialogue("application/json", mockhttpclient.DONT_CARE_REQUEST, []byte{})
		url := fmt.Sprintf("%s/a/changes/%d/revisions/%s/review", r.g.Url(0), issue.Issue, issue.Patchsets[len(issue.Patchsets)-1].ID)
		r.urlMock.MockOnce(url, p)
		r.updateIssue(issue, tryResults) // Update the issue after adding a comment.
	} else {
		r.rollerWillCloseIssue(issue)
	}
}

// pretendRollFailed changes the roll to appear to have failed in the
// mockCodereview.
func (r *mockCodereview) pretendRollFailed(issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build) {
	issue.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	r.modify(issue, tryResults)
}

// pretendRollLanded changes the roll to appear to have succeeded in the
// mockCodereview.
func (r *mockCodereview) pretendRollLanded(rm *mockRepoManager, issue *gerrit.ChangeInfo, tryResults []*buildbucket.Build) {
	// Determine what revision we rolled to.
	m := autoroll.ROLL_REV_REGEX.FindStringSubmatch(issue.Subject)
	assert.NotNil(r.t, m)
	assert.Equal(r.t, 3, len(m))
	rolledTo, err := rm.FullChildHash(m[2])
	assert.NoError(r.t, err)
	rm.mockRolledPast(rolledTo, true)
	rm.mockLastRollRev(rolledTo)
	rm.mockUpdate()

	issue.Committed = true
	issue.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	issue.Subject += "\n" + COMMITTED_STR
	issue.Status = gerrit.CHANGE_STATUS_MERGED
	r.modify(issue, tryResults)
}

// nextIssueNum provides auto-incrementing fake issue numbers.
func (r *mockCodereview) nextIssueNum() int64 {
	n := r.fakeIssueNum
	r.fakeIssueNum++
	return n
}

// checkStatus verifies that we get the expected status from the roller.
func checkStatus(t *testing.T, r *AutoRoller, rv *mockCodereview, rm *mockRepoManager, expectedStatus string, current *gerrit.ChangeInfo, currentTrybots []*buildbucket.Build, currentDryRun bool, last *gerrit.ChangeInfo, lastTrybots []*buildbucket.Build, lastDryRun bool) {
	rv.assertMocksEmpty()
	rm.assertUpdate()
	s := r.GetStatus(true)
	assert.Equal(t, expectedStatus, s.Status)
	assert.Equal(t, s.Error, "")
	checkRoll := func(t *testing.T, expect *gerrit.ChangeInfo, actual *autoroll.AutoRollIssue, expectTrybots []*buildbucket.Build, dryRun bool) {
		if expect != nil {
			assert.NotNil(t, actual)
			ari, err := autoroll.FromGerritChangeInfo(expect, rm.FullChildHash, false)
			assert.NoError(t, err)
			tryResults := make([]*autoroll.TryResult, 0, len(expectTrybots))
			for _, b := range expectTrybots {
				tryResult, err := autoroll.TryResultFromBuildbucket(b)
				assert.NoError(t, err)
				tryResults = append(tryResults, tryResult)
			}
			ari.TryResults = tryResults

			// This is kind of a hack to prevent having to pass the
			// expected dry run result around.
			if dryRun {
				if ari.AllTrybotsFinished() {
					ari.Result = autoroll.ROLL_RESULT_DRY_RUN_FAILURE
					if ari.AllTrybotsSucceeded() {
						ari.Result = autoroll.ROLL_RESULT_DRY_RUN_SUCCESS
					}
				}
			}

			assert.NoError(t, ari.Validate())
			testutils.AssertDeepEqual(t, ari, actual)
		} else {
			assert.Nil(t, actual)
		}
	}
	checkRoll(t, current, s.CurrentRoll, currentTrybots, currentDryRun)
	checkRoll(t, last, s.LastRoll, lastTrybots, lastDryRun)
}

// setup initializes a fake AutoRoller for testing. It returns the working
// directory, AutoRoller instance, URLMock for faking HTTP requests, and an
// gerrit.ChangeInfo representing the first CL that was uploaded by the AutoRoller.
func setup(t *testing.T, strategy string) (string, *AutoRoller, *mockRepoManager, *mockCodereview, *gerrit.ChangeInfo) {
	testutils.SkipIfShort(t)

	// Setup mocks.
	urlMock := mockhttpclient.NewURLMock()

	workdir, err := ioutil.TempDir("", "test_autoroll_mode_")
	assert.NoError(t, err)

	gitcookies := path.Join(workdir, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(FAKE_GERRIT_URL, gitcookies, urlMock.Client())
	assert.NoError(t, err)
	rv := &mockCodereview{
		fakeIssueNum: 10001,
		g:            g,
		t:            t,
		urlMock:      urlMock,
	}

	rm := &mockRepoManager{t: t}
	repo_manager.NewDEPSRepoManager = func(workdir, parentRepo, parentBranch, childPath, childBranch string, depot_tools string, g *gerrit.Gerrit, strategy string) (repo_manager.RepoManager, error) {
		return rm, nil
	}

	// Set up more test data.
	initialCommit := "abc1231010101010101010101010101010101010"
	rm.mockChildCommit(initialCommit)
	rm.mockChildCommit("def4561010101010101010101010101010101010")
	rm.mockLastRollRev(initialCommit)
	rm.mockRolledPast(initialCommit, true)
	roll1 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), noTrybots, false)

	// Create the roller.
	roller, err := NewAutoRoller(workdir, "parent.git", "master", "src/third_party/skia", "master", "", []string{}, g, "depot_tools", false, strategy)
	assert.NoError(t, err)

	// Verify that the bot ran successfully.
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll1, noTrybots, false, nil, nil, false)

	return workdir, roller, rm, rv, roll1
}

// TestAutoRollBasic ensures that the typical function of the AutoRoller works
// as expected.
func TestAutoRollBasic(t *testing.T) {
	testutils.LargeTest(t)
	// setup will initialize the roller and upload a CL.
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_BATCH)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()

	// Run again. Verify that we let the currently-running roll keep going.
	rv.updateIssue(roll1, noTrybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll1, noTrybots, false, nil, nil, false)

	// The roll failed. Verify that we close it and upload another one.
	rv.pretendRollFailed(roll1, noTrybots)
	rv.rollerWillCloseIssue(roll1)
	roll2 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), noTrybots, false)
	assert.NoError(t, roller.doAutoRoll())
	// The roller should have closed this CL.
	roll1.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll2, noTrybots, false, roll1, noTrybots, false)

	// The second roll succeeded. Verify that we're up-to-date.
	rv.pretendRollLanded(rm, roll2, noTrybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_UP_TO_DATE, nil, nil, false, roll2, noTrybots, false)

	// Verify that we remain idle.
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_UP_TO_DATE, nil, nil, false, roll2, noTrybots, false)
}

// TestAutoRollStop ensures that we can properly stop and restart the
// AutoRoller.
func TestAutoRollStop(t *testing.T) {
	testutils.MediumTest(t)
	// setup will initialize the roller and upload a CL.
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_BATCH)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()

	// Stop the bot. Ensure that we close the in-progress roll and don't upload a new one.
	rv.updateIssue(roll1, noTrybots)
	rv.rollerWillCloseIssue(roll1)
	// After the roller closes the CL, it will grab its info from Rietveld
	// and expect the CQ bit to be unset. and the issue to be closed.
	roll1.Status = gerrit.CHANGE_STATUS_ABANDONED
	roll1.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	// Change the mode, run the bot.
	u := "test@google.com"
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_STOPPED, u, "Stoppit!"))
	// The roller should have closed the CL.
	roll1.Status = gerrit.CHANGE_STATUS_ABANDONED
	roll1.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	checkStatus(t, roller, rv, rm, STATUS_STOPPED, nil, nil, false, roll1, noTrybots, false)

	// Ensure that we don't upload another CL now that we're stopped.
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_STOPPED, nil, nil, false, roll1, noTrybots, false)

	// Resume the bot. Ensure that we upload a new CL.
	roll2 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), noTrybots, false)
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_RUNNING, u, "Resume!"))
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll2, noTrybots, false, roll1, noTrybots, false)

	// Pretend the roll landed.
	rv.pretendRollLanded(rm, roll2, noTrybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_UP_TO_DATE, nil, nil, false, roll2, noTrybots, false)

	// Stop the roller again.
	rm.mockChildCommit("adbcdf1010101010101010101010101010101010")
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_STOPPED, u, "Stop!"))
	checkStatus(t, roller, rv, rm, STATUS_STOPPED, nil, nil, false, roll2, noTrybots, false)

	// Ensure that we don't upload another CL now that we're stopped.
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_STOPPED, nil, nil, false, roll2, noTrybots, false)
}

// TestAutoRollDryRun ensures that the Dry Run functionalify works as expected.
func TestAutoRollDryRun(t *testing.T) {
	testutils.MediumTest(t)
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_BATCH)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()

	// Change the mode to dry run. Expect the bot to switch the in-progress
	// roll to a dry run. There is one unfinished trybot.
	u := "test@google.com"
	trybot := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	trybots := []*buildbucket.Build{trybot}
	rv.rollerWillSwitchDryRun(roll1, trybots, true)
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_DRY_RUN, u, "Dry run."))
	assert.True(t, rm.sendToGerritDryRunCalled)
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_IN_PROGRESS, roll1, trybots, true, nil, nil, false)

	// Dry run succeeded.
	rv.pretendDryRunFinished(roll1, trybots, true)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_SUCCESS, roll1, trybots, true, nil, nil, false)

	// Run again. Ensure that we don't do anything crazy.
	rv.updateIssue(roll1, trybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_SUCCESS, roll1, trybots, true, nil, nil, false)

	// Add an experimental trybot. Ensure that its failure is ignored.
	trybots = append(trybots, &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Result:         autoroll.TRYBOT_RESULT_FAILURE,
		Status:         autoroll.TRYBOT_STATUS_COMPLETED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq-experimental\"}",
	})
	rv.updateIssue(roll1, trybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_SUCCESS, roll1, trybots, true, nil, nil, false)

	// New child commit: verify that we close the existing dry run and open another.
	rm.mockChildCommit("adbcdf1010101010101010101010101010101010")
	rv.updateIssue(roll1, trybots)
	rv.rollerWillCloseIssue(roll1)
	trybot2 := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	trybots2 := []*buildbucket.Build{trybot2}
	roll2 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), trybots2, true)
	assert.NoError(t, roller.doAutoRoll())
	// Roller should have closed this issue.
	roll1.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_IN_PROGRESS, roll2, trybots2, true, roll1, trybots, true)

	// Dry run failed. Ensure that we close the roll and open another, same
	// as in non-dry-run mode.
	rv.pretendDryRunFinished(roll2, trybots2, false)
	trybot3 := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	trybots3 := []*buildbucket.Build{trybot3}
	roll3 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), trybots3, true)
	assert.NoError(t, roller.doAutoRoll())
	// Roller should have closed this issue.
	roll2.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_IN_PROGRESS, roll3, trybots3, true, roll2, trybots2, true)

	// Ensure that we switch back to normal running mode as expected.
	rv.rollerWillSwitchDryRun(roll3, trybots3, false)
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_RUNNING, u, "Normal mode."))
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll3, trybots3, false, roll2, trybots2, true)

	// Switch back to dry run.
	rv.rollerWillSwitchDryRun(roll3, trybots3, true)
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_DRY_RUN, u, "Dry run again."))
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_IN_PROGRESS, roll3, trybots3, true, roll2, trybots2, true)

	// Dry run succeeded.
	rv.pretendDryRunFinished(roll3, trybots3, true)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_DRY_RUN_SUCCESS, roll3, trybots3, true, roll2, trybots2, true)

	// The successful dry run will not have the commit bit set. Make sure
	// that, when we switch back into normal mode, we re-set the commit bit
	// instead of closing the roll and opening a new one.
	rv.rollerWillSwitchDryRun(roll3, trybots3, false)
	assert.NoError(t, roller.SetMode(autoroll_modes.MODE_RUNNING, u, "Normal mode."))
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll3, trybots3, false, roll2, trybots2, true)
}

// TestAutoRollCommitLandRace ensures that we correctly handle the case in which
// a roll CL succeeds, is closed by the CQ, but does not show up in the repo by
// the time we check for it. In this case, we expect the roller to repeatedly
// sync the code, waiting for the commit to show up.
func TestAutoRollCommitLandRace(t *testing.T) {
	testutils.LargeTest(t)
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_BATCH)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()

	// Pretend the roll landed but has not yet showed up in the repo.
	trybot := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_COMPLETED,
		Result:         autoroll.TRYBOT_RESULT_SUCCESS,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	trybots := []*buildbucket.Build{trybot}

	roll1.Committed = true
	roll1.Status = gerrit.CHANGE_STATUS_MERGED
	roll1.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_NONE
	roll1.Subject += "\n" + COMMITTED_STR
	rv.modify(roll1, trybots)

	// The repo will have to force update multiple times.
	rm.mockUpdate()
	rm.mockUpdate()
	rm.mockUpdate()
	// This goroutine will cause the CL to "land" after a couple of tries.
	go func() {
		for {
			if rm.getUpdateCount() == 0 {
				m := autoroll.ROLL_REV_REGEX.FindStringSubmatch(roll1.Subject)
				assert.NotNil(t, m)
				assert.Equal(t, 3, len(m))
				rolledTo, err := rm.FullChildHash(m[2])
				assert.NoError(t, err)
				rm.mockRolledPast(rolledTo, true)
				rm.mockLastRollRev(rolledTo)
				rm.mockUpdate()
				return

			}
			time.Sleep(time.Second)
		}
	}()

	// Run the roller.
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_UP_TO_DATE, nil, nil, false, roll1, trybots, false)
}

// TestAutoRollThrottle ensures that we properly throttle the roller so that it
// doesn't upload new CLs over and over.
func TestAutoRollThrottle(t *testing.T) {
	testutils.MediumTest(t)
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_BATCH)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()

	// The roll failed. Verify that we close it and upload another one.
	rv.pretendRollFailed(roll1, noTrybots)
	rv.rollerWillCloseIssue(roll1)
	roll2 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), noTrybots, false)
	assert.NoError(t, roller.doAutoRoll())
	// The roller should have closed this CL.
	roll1.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll2, noTrybots, false, roll1, noTrybots, false)

	// The roll failed. Verify that we close it and upload another one.
	rv.pretendRollFailed(roll2, noTrybots)
	rv.rollerWillCloseIssue(roll2)
	roll3 := rm.rollerWillUpload(rv, rm.LastRollRev(), rm.NextRollRev(), noTrybots, false)
	assert.NoError(t, roller.doAutoRoll())
	// The roller should have closed this CL.
	roll2.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll3, noTrybots, false, roll2, noTrybots, false)

	// Now we should be throttled.
	rv.pretendRollFailed(roll3, noTrybots)
	rv.rollerWillCloseIssue(roll3)
	assert.NoError(t, roller.doAutoRoll())
	// The roller should have closed this CL.
	roll3.Status = gerrit.CHANGE_STATUS_ABANDONED
	checkStatus(t, roller, rv, rm, STATUS_THROTTLED, nil, nil, false, roll3, noTrybots, false)
}

// TestAutoRollSingle ensures that the one-at-a-time mode works as expected.
// This is more of a sanity check, since the actual behavior is done in the
// RepoManager.
func TestAutoRollSingle(t *testing.T) {
	testutils.MediumTest(t)
	// setup will initialize the roller and upload a CL.
	workdir, roller, rm, rv, roll1 := setup(t, repo_manager.ROLL_STRATEGY_SINGLE)
	defer func() {
		assert.NoError(t, roller.Close())
		assert.NoError(t, os.RemoveAll(workdir))
	}()
	c2 := "1111111111111111111111111111111111111111"
	c3 := "2222222222222222222222222222222222222222"
	rm.mockChildCommit(c2)
	rm.mockChildCommit(c3)

	fullHash := func(c string) (string, error) {
		if strings.HasPrefix(c2, c) {
			return c2, nil
		}
		if strings.HasPrefix(c3, c) {
			return c3, nil
		}
		return c, nil
	}

	check := func(i *gerrit.ChangeInfo, c string) {
		ari, err := autoroll.FromGerritChangeInfo(i, fullHash, false)
		assert.NoError(t, err)
		assert.Equal(t, c, ari.RollingTo)
	}

	// Verify that roll1 has a single commit.
	check(roll1, "def456101010") // From setup()

	// The roll landed, but we still have commits to roll.
	rv.pretendRollLanded(rm, roll1, noTrybots)
	roll2 := rm.rollerWillUpload(rv, rm.LastRollRev(), c2, noTrybots, false)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll2, noTrybots, false, roll1, noTrybots, false)
	check(roll2, c2)

	// Land, upload, repeat until up-to-date.
	rv.pretendRollLanded(rm, roll2, noTrybots)
	roll3 := rm.rollerWillUpload(rv, rm.LastRollRev(), c3, noTrybots, false)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_IN_PROGRESS, roll3, noTrybots, false, roll2, noTrybots, false)
	check(roll3, c3)

	// The roll succeeded. Verify that we're up-to-date.
	rv.pretendRollLanded(rm, roll3, noTrybots)
	assert.NoError(t, roller.doAutoRoll())
	checkStatus(t, roller, rv, rm, STATUS_UP_TO_DATE, nil, nil, false, roll3, noTrybots, false)
}
