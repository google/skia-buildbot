package blacklist

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	MAX_NAME_CHARS = 50
)

var (
	DEFAULT_RULES = []*Rule{
		&Rule{
			AddedBy: "BuildScheduler",
			BuilderPatterns: []string{
				buildbot.TRYBOT_PATTERN,
			},
			Commits:     []string{},
			Description: "Trybots are scheduled through Rietveld or the Commit Queue.",
			Name:        "Trybots",
		},
		&Rule{
			AddedBy: "BuildScheduler",
			BuilderPatterns: []string{
				"^Housekeeper-Nightly-RecreateSKPs_Canary$",
				"^Housekeeper-Weekly-RecreateSKPs$",
				"^Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-CT_DM_1m_SKPs$",
			},
			Commits:     []string{},
			Description: "Bots which the Build Scheduler should not schedule because they run on a timer.",
			Name:        "Timed Bots",
		},
		&Rule{
			AddedBy: "BuildScheduler",
			BuilderPatterns: []string{
				"^Infra-PerCommit$",
			},
			Commits:     []string{"355d0d378d1b9f2df9abe9fd4a73348d9b13471b"},
			Description: "Infra-PerCommit is broken at this revision.",
			Name:        "Infra-PerCommit@355d0d3",
		},
	}

	ERR_NO_SUCH_RULE = fmt.Errorf("No such rule.")
)

// Blacklist is a struct which contains rules specifying builds which should
// not be scheduled.
type Blacklist struct {
	backingFile string
	Rules       map[string]*Rule `json:"rules"`
	mtx         sync.RWMutex
}

// Match determines whether the given builder/commit pair matches one of the
// Rules in the Blacklist.
func (b *Blacklist) Match(builder, commit string) bool {
	return b.MatchRule(builder, commit) != ""
}

// MatchRule determines whether the given builder/commit pair matches one of the
// Rules in the Blacklist. Returns the name of the matched Rule or the empty
// string if no Rules match.
func (b *Blacklist) MatchRule(builder, commit string) string {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	for _, rule := range b.Rules {
		if rule.Match(builder, commit) {
			return rule.Name
		}
	}
	return ""
}

// ensureDefaults adds the necessary default blacklist rules if necessary.
func (b *Blacklist) ensureDefaults() error {
	for _, rule := range DEFAULT_RULES {
		if err := b.removeRule(rule.Name); err != nil {
			if err.Error() != ERR_NO_SUCH_RULE.Error() {
				return err
			}
		}
		if err := b.addRule(rule); err != nil {
			return err
		}
	}
	return nil
}

// writeOut writes the Blacklist to its backing file. Assumes that the caller
// holds a write lock.
func (b *Blacklist) writeOut() error {
	f, err := os.Create(b.backingFile)
	if err != nil {
		return err
	}
	defer util.Close(f)
	return json.NewEncoder(f).Encode(b)
}

// Add adds a new Rule to the Blacklist.
func (b *Blacklist) AddRule(r *Rule, repos *gitinfo.RepoMap) error {
	if err := ValidateRule(r, repos); err != nil {
		return err
	}
	return b.addRule(r)
}

// addRule adds a new Rule to the Blacklist.
func (b *Blacklist) addRule(r *Rule) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if _, ok := b.Rules[r.Name]; ok {
		return fmt.Errorf("Blacklist already contains a rule named %q", r.Name)
	}
	b.Rules[r.Name] = r
	if err := b.writeOut(); err != nil {
		delete(b.Rules, r.Name)
		return err
	}
	return nil
}

// validateCommit returns an error if the commit is not valid.
func validateCommit(c string, repos *gitinfo.RepoMap) error {
	repo, err := repos.RepoForCommit(c)
	if err != nil {
		return fmt.Errorf("Failed to validate commit %q: %s", c, err)
	}
	r, err := repos.Repo(repo)
	if err != nil {
		return fmt.Errorf("Failed to retrieve repo for commit %q: %s", c, err)
	}
	h, err := r.FullHash(c)
	if err != nil {
		return fmt.Errorf("Failed to validate commit %q: %s", c, err)
	}
	if h != c {
		return fmt.Errorf("%q is not a valid commit.", c)
	}
	return nil
}

// NewCommitRangeRule creates a new Rule which covers a range of commits.
func NewCommitRangeRule(name, user, description string, builderPatterns []string, startCommit, endCommit string, repos *gitinfo.RepoMap) (*Rule, error) {
	if err := validateCommit(startCommit, repos); err != nil {
		return nil, err
	}
	if err := validateCommit(endCommit, repos); err != nil {
		return nil, err
	}
	r, err := repos.RepoForCommit(startCommit)
	if err != nil {
		return nil, err
	}
	repo, err := repos.Repo(r)
	if err != nil {
		return nil, err
	}
	commits, err := repo.RevList(fmt.Sprintf("%s..%s", startCommit, endCommit))
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("No commits in range %s..%s", startCommit, endCommit)
	}

	// `git rev-list ${startCommit}..${endCommit}` returns a list of commits
	// which does not include startCommit but does include endCommit. For
	// blacklisting rules, we want to include startCommit and not endCommit.
	// The rev-list command returns commits in order of newest to oldest, so
	// we remove the first element of the slice (endCommit), and append
	// startCommit to the end.
	commits = append(commits[1:], startCommit)
	if util.In(endCommit, commits) {
		return nil, fmt.Errorf("Failed to adjust commit range; still includes endCommit.")
	}
	if !util.In(startCommit, commits) {
		return nil, fmt.Errorf("Failed to adjust commit range; does not include startCommit.")
	}

	rule := &Rule{
		AddedBy:         user,
		BuilderPatterns: builderPatterns,
		Commits:         commits,
		Description:     description,
		Name:            name,
	}
	if err := ValidateRule(rule, repos); err != nil {
		return nil, err
	}
	return rule, nil
}

// removeRule removes the Rule from the Blacklist.
func (b *Blacklist) removeRule(name string) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	r, ok := b.Rules[name]
	if !ok {
		return ERR_NO_SUCH_RULE
	}
	delete(b.Rules, name)
	if err := b.writeOut(); err != nil {
		b.Rules[name] = r
		return err
	}
	return nil
}

// RemoveRule removes the Rule from the Blacklist.
func (b *Blacklist) RemoveRule(name string) error {
	for _, r := range DEFAULT_RULES {
		if r.Name == name {
			return fmt.Errorf("Cannot remove built-in rule %q", name)
		}
	}
	return b.removeRule(name)
}

// Rule is a struct which indicates a specific build or set of builds which
// should not be scheduled.
//
// BuilderPatterns consists of regular expressions used to match builders
// which should not be triggered according to this Rule.
//
// Commits are simply commit hashes for which the rule applies. If the list is
// empty, the Rule applies for all commits.
//
// A Rule should specify BuilderPatterns or Commits or both.
type Rule struct {
	AddedBy         string   `json:"added_by"`
	BuilderPatterns []string `json:"builder_patterns"`
	Commits         []string `json:"commits"`
	Description     string   `json:"description"`
	Name            string   `json:"name"`
}

// ValidateRule returns an error if the given Rule is not valid.
func ValidateRule(r *Rule, repos *gitinfo.RepoMap) error {
	if r.Name == "" {
		return fmt.Errorf("Rules must have a name.")
	}
	if len(r.Name) > MAX_NAME_CHARS {
		return fmt.Errorf("Rule names must be shorter than %d characters. Use the Description field for detailed information.", MAX_NAME_CHARS)
	}
	if r.AddedBy == "" {
		return fmt.Errorf("Rules must have an AddedBy user.")
	}
	if len(r.BuilderPatterns) == 0 && len(r.Commits) == 0 {
		return fmt.Errorf("Rules must include a builder pattern and/or a commit/range.")
	}
	for _, c := range r.Commits {
		if err := validateCommit(c, repos); err != nil {
			return err
		}
	}
	return nil
}

// matchBuilder determines whether the builder portion of the Rule matches.
func (r *Rule) matchBuilder(builder string) bool {
	// If no builders are specified, then the rule applies for ALL builders.
	if len(r.BuilderPatterns) == 0 {
		return true
	}
	// If any pattern matches the builder, then the rule applies.
	for _, b := range r.BuilderPatterns {
		match, err := regexp.MatchString(b, builder)
		if err != nil {
			glog.Warningf("Rule regexp returned error for input %q: %s: %s", builder, b, err)
			return false
		}
		if match {
			return true
		}
	}
	return false
}

// matchCommit determines whether the commit portion of the Rule matches.
func (r *Rule) matchCommit(commit string) bool {
	// If no commit is specified, then the rule applies for ALL commits.
	k := len(r.Commits)
	if k == 0 {
		return true
	}
	// If at least one commit is specified, do simple string comparisons.
	for _, c := range r.Commits {
		if commit == c {
			return true
		}
	}
	return false
}

// Match returns true iff the Rule matches the given builder and commit.
func (r *Rule) Match(builder, commit string) bool {
	return r.matchBuilder(builder) && r.matchCommit(commit)
}

// FromFile returns a Blacklist instance based on the given file. If the file
// does not exist, the Blacklist will be empty and will attempt to use the file
// for writing.
func FromFile(file string) (*Blacklist, error) {
	b := &Blacklist{
		backingFile: file,
		mtx:         sync.RWMutex{},
	}
	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			b.Rules = map[string]*Rule{}
			if err := b.writeOut(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		defer util.Close(f)
		if err := json.NewDecoder(f).Decode(b); err != nil {
			return nil, err
		}
	}
	if err := b.ensureDefaults(); err != nil {
		return nil, err
	}
	return b, nil
}
