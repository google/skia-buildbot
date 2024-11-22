package common

import (
	"hash/fnv"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	ChromiumSrcGit = "https://chromium.googlesource.com/chromium/src.git"
)

func NewChromiumCommit(gitHash string) *pinpoint_proto.Commit {
	return NewCommit(ChromiumSrcGit, gitHash)
}

func NewCommit(repository, gitHash string) *pinpoint_proto.Commit {
	return &pinpoint_proto.Commit{
		GitHash:    gitHash,
		Repository: repository,
	}
}

// NewCombinedCommit returns a new CombinedCommit object.
func NewCombinedCommit(main *pinpoint_proto.Commit, deps ...*pinpoint_proto.Commit) *CombinedCommit {
	return &CombinedCommit{
		Main:         main,
		ModifiedDeps: deps,
	}
}

// A CombinedCommit represents one main base commit with any dependencies that require
// overrides as part of the build request.
// For example, if Commit is chromium/src@1, Dependency may be V8@2 which is passed
// along to Buildbucket as a deps_revision_overrides.
type CombinedCommit pinpoint_proto.CombinedCommit

// DepsToMap translates all deps into a map.
func (cc *CombinedCommit) DepsToMap() map[string]string {
	resp := make(map[string]string, 0)
	for _, c := range cc.ModifiedDeps {
		resp[c.Repository] = c.GitHash
	}
	return resp
}

// GetMainGitHash returns the git hash of main.
func (cc *CombinedCommit) GetMainGitHash() string {
	if cc.Main == nil {
		return ""
	}

	return cc.Main.GitHash
}

// Key returns all git hashes combined to use for map indexing
func (cc *CombinedCommit) Key() uint32 {
	h := fnv.New32a()

	if cc.Main == nil {
		return h.Sum32()
	}

	h.Write([]byte(cc.Main.GitHash))
	if cc.ModifiedDeps == nil {
		return h.Sum32()
	}

	for _, v := range cc.ModifiedDeps {
		h.Write([]byte(v.GitHash))
	}

	return h.Sum32()
}

// Clone returns a copy of this combined commit.
func (cc *CombinedCommit) Clone() *CombinedCommit {
	if cc.Main == nil {
		return &CombinedCommit{}
	}
	newCombinedCommit := &CombinedCommit{
		Main: &pinpoint_proto.Commit{
			Repository: cc.Main.Repository,
			GitHash:    cc.Main.GitHash,
		},
	}

	if cc.ModifiedDeps != nil {
		newModDeps := make([]*pinpoint_proto.Commit, len(cc.ModifiedDeps))
		copy(newModDeps, cc.ModifiedDeps)
		newCombinedCommit.ModifiedDeps = newModDeps
	}

	return newCombinedCommit
}

// UpsertModifiedDep inserts or updates a commit to ModifiedDeps
func (cc *CombinedCommit) UpsertModifiedDep(commit *pinpoint_proto.Commit) {
	// This operation is O(n) but is bound worst case by min(the number of
	// git-based dependencies a repository supports, bisection iterations)
	// so this should be okay. At the time of implementation, there are ~250
	// git-based repositories, and Catapult supports 30 bisection iterations,
	// so O(30).
	if cc.ModifiedDeps == nil {
		cc.ModifiedDeps = []*pinpoint_proto.Commit{commit}
		return
	}
	for _, mc := range cc.ModifiedDeps {
		if mc.Repository == commit.Repository {
			mc.GitHash = commit.GitHash
			return
		}
	}

	cc.ModifiedDeps = append(cc.ModifiedDeps, commit)
	return
}

// GetLatestModifiedDep returns the most recently added commit.
func (cc *CombinedCommit) GetLatestModifiedDep() *pinpoint_proto.Commit {
	if cc.ModifiedDeps == nil || len(cc.ModifiedDeps) == 0 {
		return nil
	}
	return cc.ModifiedDeps[len(cc.ModifiedDeps)-1]
}
