package types

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
)

const (
	ISSUE_SHORT_LENGTH = 2

	BT_ROW_KEY_VERSION = "2"
)

// Patch describes a patch which may be applied to a code checkout.
type Patch struct {
	Issue     string `json:"issue"`
	PatchRepo string `json:"patch_repo"`
	Patchset  string `json:"patchset"`
	Server    string `json:"server"`
}

// Copy returns a copy of the Patch.
func (p Patch) Copy() Patch {
	return p
}

// Valid indicates whether or not the given Patch is valid; a valid Patch
// has either none or all of its fields set.
func (p Patch) Valid() bool {
	return p.Empty() || p.Full()
}

// Empty returns true iff all of the Patch's fields are empty.
func (p Patch) Empty() bool {
	return p.Issue == "" && p.PatchRepo == "" && p.Patchset == "" && p.Server == ""
}

// Full returns true iff all of the Patch's fields are filled in.
func (p Patch) Full() bool {
	// p.PatchRepo is left out for backward compatibility.
	return p.Issue != "" && p.Patchset != "" && p.Server != ""
}

// patchIdentifier returns the identifier sequence for the patch: short issue
// number, full issue number, patch set number.
func (p Patch) patchIdentifier() []string {
	issueShort := p.Issue
	if len(issueShort) > ISSUE_SHORT_LENGTH {
		issueShort = issueShort[len(p.Issue)-ISSUE_SHORT_LENGTH:]
	}
	return []string{issueShort, p.Issue, p.Patchset}
}

// GetPatchRef returns the ref for the tryjob patch, if the RepoState includes
// a patch, and "" otherwise.
func (p Patch) GetPatchRef() string {
	if p.Full() {
		return fmt.Sprintf("refs/changes/%s", strings.Join(p.patchIdentifier(), "/"))
	}
	return ""
}

// RowKey returns a BigTable-compatible row key for the Patch.
func (p Patch) RowKey() string {
	patchRepo := ""
	if p.PatchRepo != "" {
		var err error
		patchRepo, err = git.NormalizeURL(p.PatchRepo)
		if err != nil {
			sklog.Fatalf("Failed to normalize repo URL: %s", err)
		}
	}
	return strings.Join(append(p.patchIdentifier(), patchRepo, p.Server), "#")
}

// RepoState encapsulates all of the parameters which define the state of a
// repo.
type RepoState struct {
	Patch
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
}

// Copy returns a copy of the RepoState.
func (s *RepoState) Copy() RepoState {
	return RepoState{
		Patch:    s.Patch.Copy(),
		Repo:     s.Repo,
		Revision: s.Revision,
	}
}

// Valid indicates whether or not the given RepoState is valid.
func (s RepoState) Valid() bool {
	return s.Patch.Valid() && s.Repo != "" && s.Revision != ""
}

// IsTryJob returns true iff the RepoState includes a patch.
func (s RepoState) IsTryJob() bool {
	return s.Patch.Full()
}

// GetCommit returns the repograph.Commit referenced by s, or an error if it
// can't be found.
func (s RepoState) GetCommit(repos repograph.Map) (*repograph.Commit, error) {
	repo, ok := repos[s.Repo]
	if !ok {
		return nil, fmt.Errorf("Unknown repo: %q", s.Repo)
	}
	commit := repo.Get(s.Revision)
	if commit == nil {
		return nil, fmt.Errorf("Unknown revision %q in %q", s.Revision, s.Repo)
	}
	return commit, nil
}

// Parents returns RepoStates referencing the "parents" of s. For try jobs, the
// parent is the base RepoState without a Patch. Otherwise, the parents
// reference the parent commits of s.Revision.
func (s RepoState) Parents(repos repograph.Map) ([]RepoState, error) {
	if s.IsTryJob() {
		rv := s.Copy()
		rv.Patch = Patch{}
		return []RepoState{rv}, nil
	}
	commit, err := s.GetCommit(repos)
	if err != nil {
		return nil, err
	}
	parents := commit.GetParents()
	rv := make([]RepoState, 0, len(parents))
	for _, parent := range parents {
		rv = append(rv, RepoState{
			Repo:     s.Repo,
			Revision: parent.Hash,
		})
	}
	return rv, nil
}

// RowKey returns a BigTable-compatible row key for the RepoState.
func (s RepoState) RowKey() string {
	repo, err := git.NormalizeURL(s.Repo)
	if err != nil {
		sklog.Fatalf("Failed to normalize repo URL: %s", err)
	}
	return strings.Join([]string{BT_ROW_KEY_VERSION, s.Revision, repo, s.Patch.RowKey()}, "#")
}

// String returns a human-readable string representation of the RepoState.
func (s RepoState) String() string {
	rv := fmt.Sprintf("%s@%s", s.Repo, s.Revision)
	if !s.Patch.Empty() {
		rv += "+" + s.Patch.GetPatchRef()
	}
	return rv
}
