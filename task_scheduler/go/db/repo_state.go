package db

// Patch describes a patch which may be applied to a code checkout.
type Patch struct {
	Issue    string
	Patchset string
	Storage  string
}

// Copy returns a copy of the Patch.
func (p Patch) Copy() Patch {
	return Patch{
		Issue:    p.Issue,
		Patchset: p.Patchset,
		Storage:  p.Storage,
	}
}

// RepoState encapsulates all of the parameters which define the state of a
// repo.
type RepoState struct {
	Patch
	Repo     string
	Revision string
}

// Equal returns true iff the other RepoState is equivalent to this one.
func (s *RepoState) Equal(other *RepoState) bool {
	return (s.Repo == other.Repo &&
		s.Revision == other.Revision &&
		s.Issue == other.Issue &&
		s.Patchset == other.Patchset &&
		s.Storage == other.Storage)
}

// Copy returns a copy of the RepoState.
func (s *RepoState) Copy() RepoState {
	return RepoState{
		Patch:    s.Patch.Copy(),
		Repo:     s.Repo,
		Revision: s.Revision,
	}
}
