package repo_manager

import (
	"fmt"
	"regexp"
	"sync"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/gitinfo"
)

const (
	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_SINGLE = "single"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	ForceUpdate() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	RolledPast(string) (bool, error)
	ChildHead() string
	CreateNewRoll(string, []string, string, bool) (int64, error)
	User() string
	SendToGerritCQ(*gerrit.ChangeInfo, string) error
	SendToGerritDryRun(*gerrit.ChangeInfo, string) error
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	infoMtx      sync.RWMutex
	lastRollRev  string
	repoMtx      sync.RWMutex
	parentBranch string
	childDir     string
	childHead    string
	childPath    string
	childRepo    *gitinfo.GitInfo
	childBranch  string
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

// ChildHead returns the current child origin/master branch head.
func (r *commonRepoManager) ChildHead() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.childHead
}

func (r *commonRepoManager) User() string {
	return r.user
}

func (r *commonRepoManager) IsRollSubject(line string) (bool, error) {
	rollSubjectRegex := fmt.Sprintf("^Roll %s [a-zA-Z0-9]+..[a-zA-Z0-9]+ \\([0-9]+ commits\\)$", r.childPath)
	return regexp.MatchString(rollSubjectRegex, line)
}
