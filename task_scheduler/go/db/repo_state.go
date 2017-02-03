package db

// Patch describes a patch which may be applied to a code checkout.
type Patch struct {
	Issue    string `json:"issue"`
	Patchset string `json:"patchset"`
	Server   string `json:"server"`
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
	return p.Issue == "" && p.Patchset == "" && p.Server == ""
}

// Full returns true iff all of the Patch's fields are filled in.
func (p Patch) Full() bool {
	return p.Issue != "" && p.Patchset != "" && p.Server != ""
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
