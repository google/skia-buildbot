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

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// RefMain is the ref name for the main branch.
	RefMain = git.DefaultRef

	branchBeta      = "beta"
	branchStable    = "stable"
	branchStableCut = "stable_cut"
	jsonURL         = "https://chromiumdash.appspot.com/fetch_milestones"
	os              = "linux"
	refTmplRelease  = "refs/branch-heads/%d"
)

var versionRegex = regexp.MustCompile(`(\d+)\.\d+\.(\d+)\.\d+`)

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
	// Correnspoding V8 ref
	V8Branch string `json:"v8_branch"`
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
		V8Branch:  b.V8Branch,
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
	if b.V8Branch == "" {
		return skerr.Fmt("V8Branch is required.")
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)

	type milestone struct {
		Milestone      int    `json:"milestone"`
		ChromiumBranch string `json:"chromium_branch"`
		SchedulePhase  string `json:"schedule_phase"`
		V8Branch       string `json:"v8_branch"`
	}
	var milestones []milestone
	if err := json.NewDecoder(resp.Body).Decode(&milestones); err != nil {
		return nil, skerr.Wrap(err)
	}
	byPhase := map[string]*Branch{}
	byMilestone := map[int]*Branch{}
	for _, milestone := range milestones {
		branch := &Branch{}
		branch.Milestone = milestone.Milestone
		branch.V8Branch = milestone.V8Branch
		number, err := strconv.Atoi(milestone.ChromiumBranch)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid branch number %q for channel %q", milestone.ChromiumBranch, milestone.SchedulePhase)
		}
		branch.Number = number
		branch.Ref = ReleaseBranchRef(number)
		byPhase[milestone.SchedulePhase] = branch
		byMilestone[milestone.Milestone] = branch
	}
	rv := &Branches{}
	rv.Beta = byPhase[branchBeta]
	rv.Stable = byPhase[branchStable]
	if rv.Beta == nil {
		rv.Beta = byPhase[branchStableCut]
	}
	if rv.Beta == nil && rv.Stable != nil {
		rv.Beta = byMilestone[rv.Stable.Milestone+1]
	}
	if rv.Beta != nil {
		rv.Main = &Branch{
			// TODO(borenet): Is this reliable? Is
			// there a better way to find it?
			Milestone: rv.Beta.Milestone + 1,
			Number:    0,
			Ref:       RefMain,
			V8Branch:  RefMain,
		}
	}
	if err := rv.Validate(); err != nil {
		return nil, err
	}
	return rv, nil
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
