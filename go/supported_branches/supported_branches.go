package supported_branches

/*
	Package supported_branches contains configuration for supported branches.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sort"
	"strconv"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/util"
)

const (
	// Path to the encoded SupportedBranchesConfig file.
	SUPPORTED_BRANCHES_FILE = "supported-branches.json"
	// The ref where the SupportedBranchesConfig file is stored.
	SUPPORTED_BRANCHES_REF = "infra/config"
)

// SupportedBranchesConfig lists which branches the infrastructure team supports
// for a given repo.
type SupportedBranchesConfig struct {
	// Branches is a list of supported branches.
	Branches []*SupportedBranch `json:"branches"`
}

// Sort sorts the branches in a special order: main branch first, then all other
// branches in alphanumeric order, with the exception of branches which differ
// only by an integer suffix, which are compared according to the integer, ie.
// refs/heads/chrome/m99 sorts before refs/heads/chrome/m100.
func (c *SupportedBranchesConfig) Sort() {
	sort.Sort(SupportedBranchList(c.Branches))
}

// SupportedBranch represents a single supported branch in a given repo.
type SupportedBranch struct {
	// Ref is the full name of the ref, including the "refs/heads/" prefix.
	Ref string `json:"ref"`
	// Owner is the email address of the owner of this branch. It can be a
	// comma-separated list.
	Owner string `json:"owner"`
}

// SupportedBranchList is a helper used for sorting in a special order: main
// branch first, then all other branches in alphanumeric order, with the
// exception of Branches which differ only by an integer suffix, which are
// compared according to the integer, ie. refs/heads/chrome/m99 sorts before
// refs/heads/chrome/m100.
type SupportedBranchList []*SupportedBranch

func (l SupportedBranchList) Len() int { return len(l) }
func (l SupportedBranchList) Less(a, b int) bool {
	refA := l[a].Ref
	refB := l[b].Ref
	if refA == git.DefaultRef {
		return true
	} else if refB == git.DefaultRef {
		return false
	}
	for i := 0; i < len(refA); i++ {
		if i == len(refB) {
			// refB is a substring of refA.
			return false
		}
		if refA[i] == refB[i] {
			continue
		}
		intA, errA := strconv.Atoi(refA[i:])
		intB, errB := strconv.Atoi(refB[i:])
		if errA == nil && errB == nil {
			return intA < intB
		}
		return refA[i] < refB[i]
	}
	return refA < refB
}
func (l SupportedBranchList) Swap(a, b int) {
	l[a], l[b] = l[b], l[a]
}

// DecodeConfig parses a SupportedBranchesConfig.
func DecodeConfig(r io.Reader) (*SupportedBranchesConfig, error) {
	var rv SupportedBranchesConfig
	if err := json.NewDecoder(r).Decode(&rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// EncodeConfig writes a SupportedBranchesConfig.
func EncodeConfig(w io.Writer, c *SupportedBranchesConfig) error {
	c.Sort()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ReadConfig reads a SupportedBranchesConfig from the given file.
func ReadConfig(f string) (*SupportedBranchesConfig, error) {
	var rv *SupportedBranchesConfig
	if err := util.WithReadFile(f, func(r io.Reader) error {
		var err error
		rv, err = DecodeConfig(r)
		return err
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// WriteConfig writes a SupportedBranchesConfig to the given file.
func WriteConfig(f string, c *SupportedBranchesConfig) error {
	return util.WithWriteFile(f, func(w io.Writer) error {
		return EncodeConfig(w, c)
	})
}

// ReadConfigFromRepo reads a SupportedBranchesConfig from the given repo.
func ReadConfigFromRepo(repo *gitiles.Repo) (*SupportedBranchesConfig, error) {
	contents, err := repo.ReadFileAtRef(context.Background(), SUPPORTED_BRANCHES_FILE, "infra/config")
	if err != nil {
		return nil, err
	}
	return DecodeConfig(bytes.NewReader(contents))
}
