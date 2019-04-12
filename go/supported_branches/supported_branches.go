package supported_branches

/*
	Package supported_branches contains configuration for supported branches.
*/

import (
	"bytes"
	"encoding/json"
	"io"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/util"
)

const (
	// Path to the encoded SupportedBranchesConfig file.
	SUPPORTED_BRANCHES_FILE = "supported-branches.json"
)

// SupportedBranchesConfig lists which branches the infrastructure team supports
// for a given repo.
type SupportedBranchesConfig struct {
	// Branches is a list of supported branches.
	Branches []*SupportedBranch `json:"branches"`
}

// SupportedBranch represents a single supported branch in a given repo.
type SupportedBranch struct {
	// Ref is the full name of the ref, including the "refs/heads/" prefix.
	Ref string `json:"ref"`
	// Owner is the email address of the owner of this branch. It can be a
	// comma-separated list.
	Owner string `json:"owner"`
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
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(SUPPORTED_BRANCHES_FILE, "infra/config", &buf); err != nil {
		return nil, err
	}
	return DecodeConfig(&buf)
}
