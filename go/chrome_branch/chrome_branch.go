package chrome_branch

/*
        Package chrome_branch provides utilities for retrieving Chrome release
	branches.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	RefMain = git.DefaultRef

	branchBeta     = "beta"
	branchStable   = "stable"
	jsonUrl        = "https://omahaproxy.appspot.com/all.json"
	os             = "linux"
	refTmplRelease = "refs/branch-heads/%d"
)

var trueBranchRegex = regexp.MustCompile(`(\d+)`)

// ReleaseBranchRef returns the fully-qualified ref for the release branch with
// the given number.
func ReleaseBranchRef(number int) string {
	return fmt.Sprintf(refTmplRelease, number)
}

// Branch describes a single Chrome release branch.
type Branch struct {
	// Milestone number for this branch.
	Milestone int `json:"milestone"`
	// Branch number for this branch. Always zero for the main branch, because
	// numbered release candidates are cut from this branch regularly and there
	// is no single number which refers to it.
	Number int `json:"number"`
	// Fully-qualified ref for this branch.
	Ref string `json:"ref"`
}

// Copy the Branch.
func (b *Branch) Copy() *Branch {
	if b == nil {
		return nil
	}
	return &Branch{
		Milestone: b.Milestone,
		Number:    b.Number,
		Ref:       b.Ref,
	}
}

// Validate returns an error if the Branch is not valid.
func (b *Branch) Validate() error {
	if b.Milestone == 0 {
		return skerr.Fmt("Milestone is required.")
	}
	if b.Ref == "" {
		return skerr.Fmt("Ref is required.")
	}
	if b.Ref == RefMain {
		if b.Number != 0 {
			return skerr.Fmt("Number must be zero for main branch.")
		}
	} else {
		if b.Number == 0 {
			return skerr.Fmt("Number is required for non-main branches.")
		}
	}
	return nil
}

// Branches describes the mapping from Chrome release channel name to branch
// number.
type Branches struct {
	Main   *Branch `json:"main"`
	Beta   *Branch `json:"beta"`
	Stable *Branch `json:"stable"`
}

// Copy the Branches.
func (b *Branches) Copy() *Branches {
	return &Branches{
		Main:   b.Main.Copy(),
		Beta:   b.Beta.Copy(),
		Stable: b.Stable.Copy(),
	}
}

// Validate returns an error if the Branches are not valid.
func (b *Branches) Validate() error {
	if b.Beta == nil {
		return skerr.Fmt("Beta branch is missing.")
	}
	if err := b.Beta.Validate(); err != nil {
		return skerr.Wrapf(err, "Beta branch is invalid")
	}

	if b.Stable == nil {
		return skerr.Fmt("Stable branch is missing.")
	}
	if err := b.Stable.Validate(); err != nil {
		return skerr.Wrapf(err, "Stable branch is invalid")
	}

	if b.Main == nil {
		return skerr.Fmt("Main branch is missing.")
	}
	if err := b.Main.Validate(); err != nil {
		return skerr.Wrapf(err, "Main branch is invalid")
	}

	return nil
}

// Get retrieves the current Branches.
func Get(ctx context.Context, c *http.Client) (*Branches, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonUrl, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)

	// TODO(borenet): It seems like we could just parse the branch number
	// out of current_version as well. Alternatively, we could load data
	// from chromiumdash.appspot.com which is roughly half the size and has
	// milestone and chrome_branch fields, without needing to choose an OS.
	type osVersion struct {
		Os       string `json:"os"`
		Versions []struct {
			Branch         string `json:"true_branch"`
			Channel        string `json:"channel"`
			CurrentVersion string `json:"current_version"`
		}
	}
	var osVersions []osVersion
	if err := json.NewDecoder(resp.Body).Decode(&osVersions); err != nil {
		return nil, skerr.Wrap(err)
	}
	for _, osv := range osVersions {
		if osv.Os == os {
			rv := &Branches{}
			for _, v := range osv.Versions {
				m := trueBranchRegex.FindString(v.Branch)
				if m == "" {
					return nil, skerr.Fmt("invalid branch number: %q", v.Branch)
				}
				number, err := strconv.Atoi(m)
				if err != nil {
					return nil, skerr.Wrapf(err, "invalid branch number")
				}
				split := strings.Split(v.CurrentVersion, ".")
				milestone, err := strconv.Atoi(split[0])
				if err != nil {
					return nil, skerr.Wrapf(err, "invalid milestone number")
				}
				if v.Channel == branchBeta {
					rv.Beta = &Branch{
						Milestone: milestone,
						Number:    number,
						Ref:       ReleaseBranchRef(number),
					}
				} else if v.Channel == branchStable {
					rv.Stable = &Branch{
						Milestone: milestone,
						Number:    number,
						Ref:       ReleaseBranchRef(number),
					}
				}
			}
			if rv.Beta != nil {
				rv.Main = &Branch{
					// TODO(borenet): Is this reliable? Is
					// there a better way to find it?
					Milestone: rv.Beta.Milestone + 1,
					Number:    0,
					Ref:       RefMain,
				}
			}
			if err := rv.Validate(); err != nil {
				return nil, err
			}
			return rv, nil
		}
	}
	return nil, skerr.Fmt("No branches found for OS %q", os)
}

// Client is a wrapper for Get which facilitates testing.
type Client interface {
	// Get retrieves the current Branches.
	Get(context.Context) (*Branches, error)
}

// client implements Client.
type client struct {
	*http.Client
}

// NewClient returns a Client instance.
func NewClient(c *http.Client) Client {
	return &client{
		Client: c,
	}
}

// See documentation for Client interface.
func (c *client) Get(ctx context.Context) (*Branches, error) {
	return Get(ctx, c.Client)
}
