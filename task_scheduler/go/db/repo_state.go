package db

// Patch describes a patch which may be applied to a code checkout.
type Patch struct {
	Issue    string
	Patchset string
	Server   string
}

// Copy returns a copy of the Patch.
func (p Patch) Copy() Patch {
	return p
}

// RepoState encapsulates all of the parameters which define the state of a
// repo.
type RepoState struct {
	Patch
	Repo     string
	Revision string
}

// Copy returns a copy of the RepoState.
func (s *RepoState) Copy() RepoState {
	return RepoState{
		Patch:    s.Patch.Copy(),
		Repo:     s.Repo,
		Revision: s.Revision,
	}
}
