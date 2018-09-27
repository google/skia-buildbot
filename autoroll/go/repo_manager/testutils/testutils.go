package testutils

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
)

// MockRepoManager is a struct used for mocking out the AutoRoller's
// interactions with a RepoManager.
type MockRepoManager struct {
	updateCount         int
	mockIssueNumber     int64
	mockFullChildHashes map[string]string
	lastRollRev         string
	rolledPast          map[string]bool
	rollIntoAndroid     bool
	skiaHead            string
	mtx                 sync.RWMutex
	t                   *testing.T
}

// NewRepoManager returns a MockRepoManager instance.
func NewRepoManager(t *testing.T, rollIntoAndroid bool) *MockRepoManager {
	return &MockRepoManager{
		mockFullChildHashes: map[string]string{},
		rolledPast:          map[string]bool{},
		rollIntoAndroid:     rollIntoAndroid,
		t:                   t,
	}
}

// MockRepoManagers fakes out the New*RepoManager functions.
func MockDEPSRepoManager(t *testing.T) {
	repo_manager.NewDEPSRepoManager = func(context.Context, *repo_manager.DEPSRepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client) (repo_manager.RepoManager, error) {
		return NewRepoManager(t, false), nil
	}
	repo_manager.NewAndroidRepoManager = func(context.Context, *repo_manager.AndroidRepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client) (repo_manager.RepoManager, error) {
		return NewRepoManager(t, true), nil
	}
	repo_manager.NewManifestRepoManager = func(context.Context, *repo_manager.ManifestRepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client) (repo_manager.RepoManager, error) {
		return NewRepoManager(t, false), nil
	}
}

// Update pretends to update the MockRepoManager.
func (r *MockRepoManager) Update(ctx context.Context) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.updateCount == 0 {
		return fmt.Errorf("updateCount == 0!")
	}
	r.updateCount--
	return nil
}

// mockUpdate increments the expected Update call count.
func (r *MockRepoManager) mockUpdate() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.updateCount++
}

// assertUpdate asserts that the Update call count is zero.
func (r *MockRepoManager) assertUpdate() {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	assert.Equal(r.t, 0, r.updateCount)
}

// getUpdateCount returns the remaining Update call count.
func (r *MockRepoManager) getUpdateCount() int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.updateCount
}

// FullChildHash returns the full hash of the given short hash or ref in the
// mocked child repo.
func (r *MockRepoManager) FullChildHash(ctx context.Context, shortHash string) (string, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	h, ok := r.mockFullChildHashes[shortHash]
	if !ok {
		return "", fmt.Errorf("Unknown short hash: %s", shortHash)
	}
	return h, nil
}

// MockFullChildHash adds the given mock hash.
func (r *MockRepoManager) MockFullChildHash(short, long string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.mockFullChildHashes[short] = long
}

// LastRollRev returns the last-rolled child commit in the mocked repo.
func (r *MockRepoManager) LastRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastRollRev
}

// MockLastRollRev fakes the last roll revision.
func (r *MockRepoManager) MockLastRollRev(last string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.lastRollRev = last
}

// RolledPast determines whether DEPS has rolled past the given commit in the
// mocked repo.
func (r *MockRepoManager) RolledPast(ctx context.Context, hash string) (bool, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	rv, ok := r.rolledPast[hash]
	if !ok {
		r.t.Fatal(fmt.Sprintf("Unknown hash: %s", hash))
	}
	return rv, nil
}

// mockRolledPast pretends that the DEPS has rolled past the given commit.
func (r *MockRepoManager) mockRolledPast(hash string, rolled bool) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.rolledPast[hash] = rolled
}

// NextRollRev returns the revision for the next roll.
func (r *MockRepoManager) NextRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.skiaHead
}

// mockNextRollRev sets the fake child origin/master branch head.
func (r *MockRepoManager) mockNextRollRev(hash string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.skiaHead = hash
}

// CreateNewRoll pretends to create a new DEPS roll from the mocked repo,
// returning the fake issue number set by the test.
func (r *MockRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.mockIssueNumber, nil
}

// mockChildCommit pretends that a child commit has landed.
func (r *MockRepoManager) mockChildCommit(hash string) {
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

// RollerWillUpload sets up expectations for the roller to upload a CL. Returns
// a gerrit.ChangeInfo representing the new, in-progress DEPS roll.
func (r *MockRepoManager) RollerWillUpload(issueNum int64, from, to string, dryRun bool) *gerrit.ChangeInfo {
	// Gerrit API only has millisecond precision.
	now := time.Now().UTC().Round(time.Millisecond)
	description := fmt.Sprintf(`Roll src/third_party/skia/ %s..%s (42 commits).

blah blah
TBR=some-sheriff
`, from[:12], to[:12])
	r.mockIssueNumber = issueNum
	rev := &gerrit.Revision{
		ID:            "1",
		Number:        1,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Created:       now,
	}
	cqLabel := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		if r.rollIntoAndroid {
			cqLabel = gerrit.AUTOSUBMIT_LABEL_NONE
		} else {
			cqLabel = gerrit.COMMITQUEUE_LABEL_DRY_RUN
		}
	}
	roll := &gerrit.ChangeInfo{
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Subject:       description,
		ChangeId:      fmt.Sprintf("%d", r.mockIssueNumber),
		Issue:         r.mockIssueNumber,
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
	if r.rollIntoAndroid {
		roll.Labels = map[string]*gerrit.LabelEntry{
			gerrit.PRESUBMIT_VERIFIED_LABEL: {
				All: []*gerrit.LabelDetail{},
			},
			gerrit.AUTOSUBMIT_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
					},
				},
			},
		}
	} else {
		roll.Labels = map[string]*gerrit.LabelEntry{
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
		}
	}
	return roll
}

func (r *MockRepoManager) User() string {
	return "test_user"
}

func (r *MockRepoManager) PreUploadSteps() []repo_manager.PreUploadStep {
	return nil
}

func (r *MockRepoManager) CommitsNotRolled() int {
	return -1
}

func (r *MockRepoManager) GetFullHistoryUrl() string {
	return "http://test/url/q/owner:" + r.User()
}

func (r *MockRepoManager) GetIssueUrlBase() string {
	return "http://test/url/c/"
}

func (r *MockRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

func (r *MockRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

func (r *MockRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (r *MockRepoManager) SetStrategy(strategy.NextRollStrategy) {
}
