package repo_manager

import (
	"go.skia.org/infra/go/git"

	"go.skia.org/infra/go/gerrit"
)

const (
	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_SINGLE = "single"
)

// RepoManager is used by AutoRoller for managing checkouts.
type RepoManager interface {
	ForceUpdate() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	RolledPast(string) (bool, error)
	ChildHead() string
	CreateNewRoll(string, []string, string, bool, bool) (int64, error)
	User() string
	SendToGerritCQ(*gerrit.ChangeInfo, string) error
	SendToGerritDryRun(*gerrit.ChangeInfo, string) error
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *repoManager) FullChildHash(shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *repoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether the repo has rolled past the given commit.
func (r *repoManager) RolledPast(hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return git.GitDir(r.childDir).IsAncestor(hash, r.lastRollRev)
}

// ChildHead returns the current child origin/master branch head.
func (r *repoManager) ChildHead() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.childHead
}

func (r *repoManager) User() string {
	return r.user
}
