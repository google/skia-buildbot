package skip_tasks

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	// Collection name for skip-tasks entries.
	collection = "skip-tasks"

	// We'll perform this many attempts for a given request.
	defaultAttempts = 3

	// Timeouts for various requests.
	timeoutGet = 60 * time.Second
	timeoutPut = 10 * time.Second

	maxNameChars = 50
)

var (
	errNoSuchRule = fmt.Errorf("No such rule.")
)

// DB is a struct which contains rules specifying tasks which should
// not be scheduled.
type DB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
	mtx    sync.RWMutex
	rules  map[string]*Rule
}

// NewWithParams returns a DB instance backed by Firestore, using the given params.
func NewWithParams(ctx context.Context, project, instance string, ts oauth2.TokenSource) (*DB, error) {
	client, err := firestore.NewClient(ctx, project, firestore.APP_TASK_SCHEDULER, instance, ts)
	if err != nil {
		return nil, err
	}
	return New(ctx, client)
}

// New returns a DB instance backed by the given firestore.Client.
func New(ctx context.Context, client *firestore.Client) (*DB, error) {
	b := &DB{
		client: client,
		coll:   client.Collection(collection),
	}
	if err := b.Update(ctx); err != nil {
		util.LogErr(b.Close())
		return nil, err
	}
	return b, nil
}

// Close closes the database.
func (b *DB) Close() error {
	if b != nil {
		return b.client.Close()
	}
	return nil
}

// Update updates the local view of the skip rules to match the remote DB.
func (b *DB) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "skiptasks_Update")
	defer span.End()
	if b == nil {
		return nil
	}
	rules := map[string]*Rule{}
	q := b.coll.Query
	if err := b.client.IterDocs(ctx, "GetSkipTasksEntries", "", q, defaultAttempts, timeoutGet, func(doc *fs.DocumentSnapshot) error {
		var r Rule
		if err := doc.DataTo(&r); err != nil {
			return err
		}
		rules[r.Name] = &r
		return nil
	}); err != nil {
		return err
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.rules = rules
	return nil
}

// AutoUpdate starts a goroutine which automatically updates the DB as changes
// occur. Starts the goroutine and returns immediately. The goroutine exits when
// the given context expires.
func (b *DB) AutoUpdate(ctx context.Context) {
	go func() {
		// TODO(borenet): The QuerySnapshotChannel will stop if it
		// encounters any error. We should add retry with backoff,
		// either here or in the go/firestore package.
		for snap := range firestore.QuerySnapshotChannel(ctx, b.coll.Query) {
			sklog.Infof("Received skip_tasks update")
			docs, err := snap.Documents.GetAll()
			if err != nil {
				sklog.Errorf("Failed to retrieve documents from query snapshot: %s", err)
				continue
			}
			rules := make(map[string]*Rule, len(docs))
			for _, doc := range docs {
				var r Rule
				if err := doc.DataTo(&r); err != nil {
					sklog.Errorf("Failed to decode document %s from query snapshot: %s", doc.Ref.ID, err)
					continue
				}
				rules[r.Name] = &r
			}
			b.mtx.Lock()
			b.rules = rules
			b.mtx.Unlock()
		}
	}()
}

// Match determines whether the given taskSpec/commit pair matches one of the
// Rules in the DB.
func (b *DB) Match(taskSpec, commit string) bool {
	return b.MatchRule(taskSpec, commit) != ""
}

// MatchRule determines whether the given taskSpec/commit pair matches one of
// the Rules in the DB. Returns the name of the matched Rule or the empty string
// if no Rules match.
func (b *DB) MatchRule(taskSpec, commit string) string {
	if b == nil {
		return ""
	}
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	for _, rule := range b.rules {
		if rule.Match(taskSpec, commit) {
			return rule.Name
		}
	}
	return ""
}

// Add adds a new Rule to the DB.
func (b *DB) AddRule(r *Rule, repos repograph.Map) error {
	if b == nil {
		return errors.New("DB is nil; cannot add rules.")
	}
	if err := ValidateRule(r, repos); err != nil {
		return err
	}
	return b.addRule(r)
}

// addRule adds a new Rule to the DB.
func (b *DB) addRule(r *Rule) (rvErr error) {
	ref := b.coll.Doc(r.Name)
	if _, err := b.client.Create(context.TODO(), ref, r, defaultAttempts, timeoutPut); err != nil {
		return err
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.rules[r.Name] = r
	return nil
}

// NewCommitRangeRule creates a new Rule which covers a range of commits.
func NewCommitRangeRule(ctx context.Context, name, user, description string, taskSpecPatterns []string, startCommit, endCommit string, repos repograph.Map) (*Rule, error) {
	_, repoName, _, err := repos.FindCommit(startCommit)
	if err != nil {
		return nil, err
	}
	_, repo2, _, err := repos.FindCommit(endCommit)
	if err != nil {
		return nil, err
	}
	if repo2 != repoName {
		return nil, fmt.Errorf("Commit %s is in a different repo (%s) from %s (%s)", endCommit, repo2, startCommit, repoName)
	}
	repo, ok := repos[repoName]
	if !ok {
		return nil, fmt.Errorf("Unknown repo %s", repoName)
	}
	commits, err := repo.RevList(startCommit, endCommit)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("No commits in range %s..%s", startCommit, endCommit)
	}

	// `git rev-list ${startCommit}..${endCommit}` returns a list of commits
	// which does not include startCommit but does include endCommit. For
	// skip rules, we want to include startCommit and not endCommit.
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
		AddedBy:          user,
		TaskSpecPatterns: taskSpecPatterns,
		Commits:          commits,
		Description:      description,
		Name:             name,
	}
	if err := ValidateRule(rule, repos); err != nil {
		return nil, err
	}
	return rule, nil
}

// RemoveRule removes the Rule from the DB.
func (b *DB) RemoveRule(id string) error {
	if b == nil {
		return errors.New("DB is nil; cannot remove rules.")
	}
	ref := b.coll.Doc(id)
	if _, err := b.client.Delete(context.TODO(), ref, defaultAttempts, timeoutPut); err != nil {
		return err
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()
	delete(b.rules, id)
	return nil
}

// GetRules returns a slice containing all of the Rules in the DB.
func (b *DB) GetRules() []*Rule {
	if b == nil {
		return []*Rule{}
	}
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	rv := make([]*Rule, 0, len(b.rules))
	for _, r := range b.rules {
		rv = append(rv, r.Copy())
	}
	sort.Sort(rules(rv))
	return rv
}

// Rule is a struct which indicates a specific task or set of tasks which
// should not be scheduled.
//
// TaskSpecPatterns consists of regular expressions used to match taskSpecs
// which should not be triggered according to this Rule.
//
// Commits are simply commit hashes for which the rule applies. If the list is
// empty, the Rule applies for all commits.
//
// A Rule should specify TaskSpecPatterns or Commits or both.
//
// TODO(borenet): Add an explicit ID field and a timestamp.
type Rule struct {
	AddedBy          string   `json:"added_by"`
	TaskSpecPatterns []string `json:"task_spec_patterns"`
	Commits          []string `json:"commits"`
	Description      string   `json:"description"`
	Name             string   `json:"name"`
}

type rules []*Rule

func (r rules) Len() int {
	return len(r)
}
func (r rules) Less(a, b int) bool {
	return r[a].Name < r[b].Name
}
func (r rules) Swap(a, b int) {
	r[a], r[b] = r[b], r[a]
}

// ValidateRule returns an error if the given Rule is not valid.
func ValidateRule(r *Rule, repos repograph.Map) error {
	if r.Name == "" {
		return errors.New("Rules must have a name.")
	}
	if len(r.Name) > maxNameChars {
		return fmt.Errorf("Rule names must be shorter than %d characters. Use the Description field for detailed information.", maxNameChars)
	}
	if r.AddedBy == "" {
		return errors.New("Rules must have an AddedBy user.")
	}
	if len(r.TaskSpecPatterns) == 0 && len(r.Commits) == 0 {
		return errors.New("Rules must include a taskSpec pattern and/or a commit/range.")
	}
	for _, c := range r.Commits {
		if _, _, _, err := repos.FindCommit(c); err != nil {
			return err
		}
	}
	return nil
}

// matchTaskSpec determines whether the taskSpec portion of the Rule matches.
func (r *Rule) matchTaskSpec(taskSpec string) bool {
	// If no taskSpecs are specified, then the rule applies for ALL taskSpecs.
	if len(r.TaskSpecPatterns) == 0 {
		return true
	}
	// If any pattern matches the taskSpec, then the rule applies.
	for _, b := range r.TaskSpecPatterns {
		match, err := regexp.MatchString(b, taskSpec)
		if err != nil {
			sklog.Warningf("Rule regexp returned error for input %q: %s: %s", taskSpec, b, err)
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

// Match returns true iff the Rule matches the given taskSpec and commit.
func (r *Rule) Match(taskSpec, commit string) bool {
	return r.matchTaskSpec(taskSpec) && r.matchCommit(commit)
}

// Copy returns a deep copy of the Rule.
func (r *Rule) Copy() *Rule {
	return &Rule{
		AddedBy:          r.AddedBy,
		TaskSpecPatterns: util.CopyStringSlice(r.TaskSpecPatterns),
		Commits:          util.CopyStringSlice(r.Commits),
		Description:      r.Description,
		Name:             r.Name,
	}
}
