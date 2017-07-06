package repo_manager

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_SINGLE = "single"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	Update() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	NextRollRev() string
	RolledPast(string) (bool, error)
	CreateNewRoll(string, string, []string, string, bool) (int64, error)
	User() string
	SendToGerritCQ(*gerrit.ChangeInfo, string) error
	SendToGerritDryRun(*gerrit.ChangeInfo, string) error
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	infoMtx      sync.RWMutex
	lastRollRev  string
	nextRollRev  string
	repoMtx      sync.RWMutex
	parentBranch string
	childDir     string
	childPath    string
	childRepo    *git.Checkout
	childBranch  string
	strategy     string
	user         string
	workdir      string
	g            gerrit.GerritInterface
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *commonRepoManager) FullChildHash(shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *commonRepoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether the repo has rolled past the given commit.
func (r *commonRepoManager) RolledPast(hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return git.GitDir(r.childDir).IsAncestor(hash, r.lastRollRev)
}

// NextRollRev returns the revision of the next roll.
func (r *commonRepoManager) NextRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.nextRollRev
}

// Start makes the RepoManager begin the periodic update process.
func Start(r RepoManager, frequency time.Duration, ctx context.Context) {
	lv := metrics2.NewLiveness("last-successful-repo-manager-update")
	go util.RepeatCtx(frequency, ctx, func() {
		if err := r.Update(); err != nil {
			sklog.Errorf("Failed to update repo manager: %s", err)
		} else {
			lv.Reset()
		}
	})
}

func (r *commonRepoManager) User() string {
	return r.user
}

func (r *commonRepoManager) IsRollSubject(line string) (bool, error) {
	rollSubjectRegex := fmt.Sprintf("^Roll %s [a-zA-Z0-9]+..[a-zA-Z0-9]+ \\([0-9]+ commits\\)$", r.childPath)
	return regexp.MatchString(rollSubjectRegex, line)
}
