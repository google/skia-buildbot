package chrome_branch

/*
        Package chrome_branch provides utilities for retrieving Chrome release
	branches.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"text/template"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	branchBeta   = "beta"
	branchStable = "stable"
	jsonUrl      = "https://omahaproxy.appspot.com/all.json"
	os           = "linux"
)

// Branches describes the mapping from Chrome release channel name to branch
// number.
type Branches struct {
	Beta   string `json:"beta"`
	Stable string `json:"stable"`
}

// Copy the Branches.
func (b *Branches) Copy() *Branches {
	return &Branches{
		Beta:   b.Beta,
		Stable: b.Stable,
	}
}

// Validate returns an error if the Branches are not valid.
func (b *Branches) Validate() error {
	if b.Beta == "" {
		return skerr.Fmt("Beta branch is missing.")
	}
	if b.Stable == "" {
		return skerr.Fmt("Stable branch is missing.")
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
	type osVersion struct {
		Os       string `json:"os"`
		Versions []struct {
			Branch  string `json:"true_branch"`
			Channel string `json:"channel"`
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
				if v.Channel == branchBeta {
					rv.Beta = v.Branch
				} else if v.Channel == branchStable {
					rv.Stable = v.Branch
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

// Execute the given template with the given Branches.
func Execute(tmpl string, b *Branches) (string, error) {
	t, err := template.New(uuid.New().String()).Parse(tmpl)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	t.Option("missingkey=error")
	var buf bytes.Buffer
	if err := t.Execute(&buf, b); err != nil {
		return "", skerr.Wrap(err)
	}
	return buf.String(), nil
}

// Manager caches the branch mapping.
type Manager interface {
	Update(context.Context) error
	Get() *Branches
	Execute(string) (string, error)
}

type manager struct {
	branches *Branches
	client   *http.Client
	mtx      sync.RWMutex
}

// New returns a Manager instance.
func New(ctx context.Context, c *http.Client) (*manager, error) {
	m := &manager{
		client: c,
	}
	if err := m.Update(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	return m, nil
}

// Update the branch mapping.
func (m *manager) Update(ctx context.Context) error {
	b, err := Get(ctx, m.client)
	if err != nil {
		return skerr.Wrap(err)
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.branches = b
	return nil
}

// Get the current branch mapping.
func (m *manager) Get() *Branches {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.branches.Copy()
}

// Execute the given template string using the current branch mapping.
func (m *manager) Execute(tmpl string) (string, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return Execute(tmpl, m.branches)
}

var _ Manager = &manager{}
