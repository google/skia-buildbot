package gerrit

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	ErrNotFound = errors.New("Requested item was not found")
)

const (
	TIME_FORMAT         = "2006-01-02 15:04:05.999999"
	GERRIT_CHROMIUM_URL = "https://chromium-review.googlesource.com"
	GERRIT_SKIA_URL     = "https://skia-review.googlesource.com"
	MAX_GERRIT_LIMIT    = 500

	AUTH_SCOPE = auth.SCOPE_GERRIT

	CHANGE_STATUS_ABANDONED = "ABANDONED"
	CHANGE_STATUS_DRAFT     = "DRAFT"
	CHANGE_STATUS_MERGED    = "MERGED"
	CHANGE_STATUS_NEW       = "NEW"

	// Gerrit labels.
	CODEREVIEW_LABEL            = "Code-Review"
	CODEREVIEW_LABEL_DISAPPROVE = -1
	CODEREVIEW_LABEL_NONE       = 0
	CODEREVIEW_LABEL_APPROVE    = 1

	// Chromium specific labels.
	COMMITQUEUE_LABEL         = "Commit-Queue"
	COMMITQUEUE_LABEL_NONE    = 0
	COMMITQUEUE_LABEL_DRY_RUN = 1
	COMMITQUEUE_LABEL_SUBMIT  = 2

	// Android specific labels.
	AUTOSUBMIT_LABEL                  = "Autosubmit"
	AUTOSUBMIT_LABEL_NONE             = 0
	AUTOSUBMIT_LABEL_SUBMIT           = 1
	PRESUBMIT_READY_LABEL             = "Presubmit-Ready"
	PRESUBMIT_VERIFIED_LABEL          = "Presubmit-Verified"
	PRESUBMIT_VERIFIED_LABEL_REJECTED = -1
	PRESUBMIT_VERIFIED_LABEL_RUNNING  = 0
	PRESUBMIT_VERIFIED_LABEL_ACCEPTED = 1

	URL_TMPL_CHANGE = "/changes/%d/detail?o=ALL_REVISIONS"

	// Kinds of patchsets.
	PATCHSET_KIND_MERGE_FIRST_PARENT_UPDATE = "MERGE_FIRST_PARENT_UPDATE"
	PATCHSET_KIND_NO_CHANGE                 = "NO_CHANGE"
	PATCHSET_KIND_NO_CODE_CHANGE            = "NO_CODE_CHANGE"
	PATCHSET_KIND_REWORK                    = "REWORK"
	PATCHSET_KIND_TRIVIAL_REBASE            = "TRIVIAL_REBASE"

	// extractReg is the regular expression used by ExtractIssueFromCommit.
	extractRegTmpl = `^\s*Reviewed-on:.*%s.*/([0-9]+)\s*$`
)

var (
	TRIVIAL_PATCHSET_KINDS = []string{
		PATCHSET_KIND_TRIVIAL_REBASE,
		PATCHSET_KIND_NO_CHANGE,
		PATCHSET_KIND_NO_CODE_CHANGE,
	}
)

// ChangeInfo contains information about a Gerrit issue.
type ChangeInfo struct {
	Id              string                 `json:"id"`
	Created         time.Time              `json:"-"`
	CreatedString   string                 `json:"created"`
	Updated         time.Time              `json:"-"`
	UpdatedString   string                 `json:"updated"`
	Submitted       time.Time              `json:"-"`
	SubmittedString string                 `json:"submitted"`
	Project         string                 `json:"project"`
	ChangeId        string                 `json:"change_id"`
	Subject         string                 `json:"subject"`
	Branch          string                 `json:"branch"`
	Committed       bool                   `json:"committed"`
	Revisions       map[string]*Revision   `json:"revisions"`
	Patchsets       []*Revision            `json:"-"`
	MoreChanges     bool                   `json:"_more_changes"`
	Issue           int64                  `json:"_number"`
	Labels          map[string]*LabelEntry `json:"labels"`
	Owner           *Owner                 `json:"owner"`
	Status          string                 `json:"status"`
}

// Find the set of non-trivial patchsets. Returns the Revisions in order of
// patchset number.
func (ci *ChangeInfo) GetNonTrivialPatchSets() []*Revision {
	allPatchSets := make([]int, 0, len(ci.Revisions))
	byNumber := make(map[int]*Revision, len(ci.Revisions))
	for _, rev := range ci.Revisions {
		allPatchSets = append(allPatchSets, int(rev.Number))
		byNumber[int(rev.Number)] = rev
	}
	sort.Ints(allPatchSets)
	rv := make([]*Revision, 0, len(ci.Revisions))
	for idx, num := range allPatchSets {
		rev := byNumber[num]
		// Skip the last patch set for merged CLs, since it is auto-
		// generated.
		if ci.Status == CHANGE_STATUS_MERGED && idx == len(allPatchSets)-1 {
			continue
		}
		if !util.In(rev.Kind, TRIVIAL_PATCHSET_KINDS) {
			rv = append(rv, rev)
		}
	}
	return rv
}

// The RelatedChangesInfo entity contains information about related changes.
type RelatedChangesInfo struct {
	Changes []*RelatedChangeAndCommitInfo `json:"changes"`
}

// RelatedChangeAndCommitInfo entity contains information about a related change and commit.
type RelatedChangeAndCommitInfo struct {
	ChangeId string `json:"change_id"`
	Issue    int64  `json:"_change_number"`
	Revision int64  `json:"_revision_number"`
	Status   string `json:"status"`
}

// IsClosed returns true iff the issue corresponding to the ChangeInfo is
// abandoned or merged.
func (c ChangeInfo) IsClosed() bool {
	return (c.Status == CHANGE_STATUS_ABANDONED ||
		c.Status == CHANGE_STATUS_MERGED)
}

// Owner gathers the owner information of a ChangeInfo instance. Some fields omitted.
type Owner struct {
	Email string `json:"email"`
}

type LabelEntry struct {
	All          []*LabelDetail
	Values       map[string]string
	DefaultValue int
}

type LabelDetail struct {
	Name  string
	Email string
	Date  string
	Value int
}

type FileInfo struct {
	Status        string `json:"status"`
	Binary        bool   `json:"binary"`
	OldPath       string `json:"old_path"`
	LinesInserted int    `json:"lines_inserted"`
	LinesDeleted  int    `json:"lines_deleted"`
	SizeDelta     int    `json:"size_delta"`
	Size          int    `json:"size"`
}

// Revision is the information associated with a patchset in Gerrit.
type Revision struct {
	ID            string    `json:"-"`
	Number        int64     `json:"_number"`
	CreatedString string    `json:"created"`
	Created       time.Time `json:"-"`
	Kind          string    `json:"kind"`
}

type GerritInterface interface {
	Abandon(*ChangeInfo, string) error
	AddComment(*ChangeInfo, string) error
	Approve(*ChangeInfo, string) error
	CreateChange(string, string, string, string) (*ChangeInfo, error)
	DeleteChangeEdit(*ChangeInfo) error
	DeleteFile(*ChangeInfo, string) error
	DisApprove(*ChangeInfo, string) error
	EditFile(*ChangeInfo, string, string) error
	ExtractIssueFromCommit(string) (int64, error)
	Files(issue int64, patch string) (map[string]*FileInfo, error)
	GetFileNames(issue int64, patch string) ([]string, error)
	GetIssueProperties(int64) (*ChangeInfo, error)
	GetPatch(int64, string) (string, error)
	GetRepoUrl() string
	GetTrybotResults(int64, int64) ([]*buildbucket.Build, error)
	GetUserEmail() (string, error)
	Initialized() bool
	IsBinaryPatch(issue int64, patch string) (bool, error)
	MoveFile(*ChangeInfo, string, string) error
	NoScore(*ChangeInfo, string) error
	PublishChangeEdit(*ChangeInfo) error
	RemoveFromCQ(*ChangeInfo, string) error
	Search(int, ...*SearchTerm) ([]*ChangeInfo, error)
	SendToCQ(*ChangeInfo, string) error
	SendToDryRun(*ChangeInfo, string) error
	SetCommitMessage(*ChangeInfo, string) error
	SetReview(*ChangeInfo, string, map[string]interface{}, []string) error
	SetTopic(string, int64) error
	TurnOnAuthenticatedGets()
	Url(int64) string
}

// Gerrit is an object used for interacting with the issue tracker.
type Gerrit struct {
	client               *http.Client
	buildbucketClient    *buildbucket.Client
	gitCookiesPath       string
	url                  string
	useAuthenticatedGets bool
	extractRegEx         *regexp.Regexp
}

// NewGerrit returns a new Gerrit instance. If gitCookiesPath is empty the
// instance will be in read-only mode and only return information available to
// anonymous users.
func NewGerrit(gerritUrl, gitCookiesPath string, client *http.Client) (*Gerrit, error) {
	parsedUrl, err := url.Parse(gerritUrl)
	if err != nil {
		return nil, sklog.FmtErrorf("Unable to parse gerrit URL: %s", err)
	}

	regExStr := fmt.Sprintf(extractRegTmpl, parsedUrl.Host)
	extractRegEx, err := regexp.Compile(regExStr)
	if err != nil {
		return nil, sklog.FmtErrorf("Unable to compile regular expression '%s'. Error: %s", regExStr, err)
	}

	if client == nil {
		client = httputils.NewTimeoutClient()
	}
	return &Gerrit{
		url:               gerritUrl,
		client:            client,
		buildbucketClient: buildbucket.NewClient(client),
		gitCookiesPath:    gitCookiesPath,
		extractRegEx:      extractRegEx,
	}, nil
}

// DefaultGitCookiesPath returns the default cookie file. The return value
// can be used as the input to NewGerrit. If it cannot be retrieved an
// error will be logged and the empty string is returned.
func DefaultGitCookiesPath() string {
	usr, err := user.Current()
	if err != nil {
		sklog.Errorf("Unable to retrieve default git cookies path")
		return ""
	}
	return filepath.Join(usr.HomeDir, ".gitcookies")
}

// GitCookieAuthDaemonPath returns the default path that git_cookie_authdaemon
// writes to. See infra/git_cookie_authdaemon
func GitCookieAuthDaemonPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", sklog.FmtErrorf("Unable to retrieve user for git_auth_deamon default cookie path.")
	}
	return filepath.Join(usr.HomeDir, ".git-credential-cache", "cookie"), nil
}

func parseTime(t string) time.Time {
	parsed, _ := time.Parse(TIME_FORMAT, t)
	return parsed
}

// Initialized returns false if the implementation of GerritInterface has not
// been initialized (i.e. it is a pointer to nil).
func (g *Gerrit) Initialized() bool {
	return g != nil
}

// TurnOnAuthenticatedGets makes all GET requests contain authentication
// cookies. By default only POST requests are automatically authenticated.
func (g *Gerrit) TurnOnAuthenticatedGets() {
	g.useAuthenticatedGets = true
}

// Url returns the url of the Gerrit issue identified by issueID or the
// base URL of the Gerrit instance if issueID is 0.
func (g *Gerrit) Url(issueID int64) string {
	if issueID == 0 {
		return g.url
	}
	return fmt.Sprintf("%s/c/%d", g.url, issueID)
}

type AccountDetails struct {
	AccountId int64  `json:"_account_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	UserName  string `json:"username"`
}

// GetUserEmail returns the Gerrit user's email address.
func (g *Gerrit) GetUserEmail() (string, error) {
	g.TurnOnAuthenticatedGets()
	url := "/accounts/self/detail"
	var account AccountDetails
	if err := g.get(url, &account, nil); err != nil {
		return "", fmt.Errorf("Failed to retrieve user: %s", err)
	}
	return account.Email, nil
}

// GetRepoUrl returns the url of the Googlesource repo.
func (g *Gerrit) GetRepoUrl() string {
	return strings.Replace(g.url, "-review", "", 1)
}

// ExtractIssueFromCommit returns the issue id by parsing the commit message of
// a landed commit. It expects the commit message to contain one line in this format:
//
//     Reviewed-on: https://skia-review.googlesource.com/999999
//
// where the digits at the end are the issue id.
func (g *Gerrit) ExtractIssueFromCommit(commitMsg string) (int64, error) {
	scanner := bufio.NewScanner(bytes.NewBuffer([]byte(commitMsg)))
	for scanner.Scan() {
		line := scanner.Text()
		result := g.extractRegEx.FindStringSubmatch(line)
		if len(result) == 2 {
			ret, err := strconv.ParseInt(result[1], 10, 64)
			if err != nil {
				return 0, sklog.FmtErrorf("Unable to parse issue id '%s'. Got error: %s", err)
			}

			return ret, nil
		}
	}
	return 0, sklog.FmtErrorf("Unable to extract issue id from commit message.")
}

// Fix up a ChangeInfo object, received via the Gerrit API, to contain all of
// the fields it is expected to contain. Returns the ChangeInfo object for
// convenience.
func fixupChangeInfo(ci *ChangeInfo) *ChangeInfo {
	// Set created, updated and submitted timestamps. Also set the committed flag.
	ci.Created = parseTime(ci.CreatedString)
	ci.Updated = parseTime(ci.UpdatedString)
	if ci.SubmittedString != "" {
		ci.Submitted = parseTime(ci.SubmittedString)
		ci.Committed = true
	}
	// Make patchset objects with the revision IDs and created timestamps.
	patchsets := make([]*Revision, 0, len(ci.Revisions))
	for id, r := range ci.Revisions {
		// Fill in the missing fields.
		r.ID = id
		r.Created = parseTime(r.CreatedString)
		patchsets = append(patchsets, r)
	}
	sort.Sort(revisionSlice(patchsets))
	ci.Patchsets = patchsets
	return ci
}

// GetIssueProperties returns a fully filled-in ChangeInfo object, as opposed to
// the partial data returned by Gerrit's search endpoint.
// If the given issue cannot be found ErrNotFound is returned as error.
func (g *Gerrit) GetIssueProperties(issue int64) (*ChangeInfo, error) {
	url := fmt.Sprintf(URL_TMPL_CHANGE, issue)
	fullIssue := &ChangeInfo{}
	if err := g.get(url, fullIssue, ErrNotFound); err != nil {
		// Pass ErrNotFound through unchanged so calling functions can check for it.
		if err == ErrNotFound {
			return nil, err
		}
		return nil, sklog.FmtErrorf("Failed to load details for issue %d: %v", issue, err)
	}
	return fixupChangeInfo(fullIssue), nil
}

// GetPatchsetIDs is a convenience function that returns the sorted list of patchset IDs.
func (c *ChangeInfo) GetPatchsetIDs() []int64 {
	ret := make([]int64, len(c.Patchsets))
	for idx, patchSet := range c.Patchsets {
		ret[idx] = patchSet.Number
	}
	return ret
}

// GetPatch returns the formatted patch for one revision. Documentation is here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-patch
func (g *Gerrit) GetPatch(issue int64, revision string) (string, error) {
	url := fmt.Sprintf("%s/changes/%d/revisions/%s/patch", g.url, issue, revision)
	resp, err := g.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %s: %s", url, err)
	}
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("Issue not found: %s", url)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Error retrieving %s: %d %s", url, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Could not read response body: %s", err)
	}

	data, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return "", fmt.Errorf("Could not base64 decode response body: %s", err)
	}
	// Extract out only the patch.
	tokens := strings.SplitN(string(data), "---", 2)
	if len(tokens) != 2 {
		return "", fmt.Errorf("Gerrit patch response was invalid: %s", string(data))
	}
	patch := tokens[1]
	return patch, nil
}

// CommitInfo captures information about the commit of a revision (patchset)
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#commit-info
type CommitInfo struct {
	Commit  string        `json:"commit"`
	Parents []*CommitInfo `json:"parents"`
}

// GetCommit retrieves the commit that corresponds to the patch identified by issue and revision.
// It allows to retrieve the parent commit on which the given patchset is based on.
// See: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-commit
func (g *Gerrit) GetCommit(issue int64, revision string) (*CommitInfo, error) {
	path := fmt.Sprintf("/changes/%d/revisions/%s/commit", issue, revision)
	ret := &CommitInfo{}
	err := g.get(path, ret, nil)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

type reviewer struct {
	Reviewer string `json:"reviewer"`
}

// setReview calls the Set Review endpoint of the Gerrit API to add messages and/or set labels for
// the latest patchset.
// API documentation: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
func (g *Gerrit) SetReview(issue *ChangeInfo, message string, labels map[string]interface{}, reviewers []string) error {
	postData := map[string]interface{}{
		"message": message,
		"labels":  labels,
	}

	if len(reviewers) > 0 {
		revs := make([]*reviewer, 0, len(reviewers))
		for _, r := range reviewers {
			revs = append(revs, &reviewer{
				Reviewer: r,
			})
		}
		postData["reviewers"] = revs
	}
	latestPatchset := issue.Patchsets[len(issue.Patchsets)-1]
	return g.postJson(fmt.Sprintf("/a/changes/%s/revisions/%s/review", issue.ChangeId, latestPatchset.ID), postData)
}

// AddComment adds a message to the issue.
func (g *Gerrit) AddComment(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{}, nil)
}

// Utility methods for interacting with the COMMITQUEUE_LABEL.

func (g *Gerrit) SendToDryRun(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN}, nil)
}

func (g *Gerrit) SendToCQ(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT}, nil)
}

func (g *Gerrit) RemoveFromCQ(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_NONE}, nil)
}

// Utility methods for interacting with the CODEREVIEW_LABEL.

func (g *Gerrit) Approve(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_APPROVE}, nil)
}

func (g *Gerrit) NoScore(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_NONE}, nil)
}

func (g *Gerrit) DisApprove(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_DISAPPROVE}, nil)
}

// Abandon abandons the issue with the given message.
func (g *Gerrit) Abandon(issue *ChangeInfo, message string) error {
	postData := map[string]interface{}{
		"message": message,
	}
	return g.postJson(fmt.Sprintf("/a/changes/%s/abandon", issue.ChangeId), postData)
}

// get retrieves the given sub URL and populates 'rv' with the result.
// If notFoundError is not nil it will be returned if the requested item doesn't
// exist.
func (g *Gerrit) get(suburl string, rv interface{}, notFoundError error) error {
	getURL := g.url + suburl
	if g.useAuthenticatedGets {
		getURL = g.url + "/a" + suburl
	}
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}

	if g.useAuthenticatedGets {
		if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
			return err
		}
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", getURL, err)
	}
	if resp.StatusCode == 404 {
		if notFoundError != nil {
			return notFoundError
		}
		return fmt.Errorf("Issue not found: %s", getURL)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error retrieving %s: %d %s", getURL, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Could not read response body: %s", err)
	}

	// Strip off the XSS protection chars.
	parts := strings.SplitN(string(body), "\n", 2)

	if len(parts) != 2 {
		return fmt.Errorf("Reponse invalid format.")
	}
	if err := json.Unmarshal([]byte(parts[1]), &rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return nil
}

func (g *Gerrit) post(suburl string, b []byte) error {
	req, err := http.NewRequest("POST", g.url+suburl, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return fmt.Errorf("Got status %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

func (g *Gerrit) postJson(suburl string, postData interface{}) error {
	b, err := json.Marshal(postData)
	if err != nil {
		return err
	}
	return g.post(suburl, b)
}

func (g *Gerrit) put(suburl string, b []byte) error {
	req, err := http.NewRequest("PUT", g.url+suburl, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return fmt.Errorf("Got status %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

func (g *Gerrit) putJson(suburl string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return g.put(suburl, b)
}

func (g *Gerrit) delete(suburl string) error {
	req, err := http.NewRequest("DELETE", g.url+suburl, nil)
	if err != nil {
		return err
	}

	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return fmt.Errorf("Got status %s (%d): %s", resp.Status, resp.StatusCode, string(respBytes))
	}
	return nil
}

type changeListSortable []*ChangeInfo

func (p changeListSortable) Len() int           { return len(p) }
func (p changeListSortable) Less(i, j int) bool { return p[i].Created.Before(p[j].Created) }
func (p changeListSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type revisionSlice []*Revision

func (r revisionSlice) Len() int           { return len(r) }
func (r revisionSlice) Less(i, j int) bool { return r[i].Created.Before(r[j].Created) }
func (r revisionSlice) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

// SearchTerm is a wrapper for search terms to pass into the Search method.
type SearchTerm struct {
	Key   string
	Value string
}

// SearchOwner is a SearchTerm used for filtering by issue owner.
// API documentation is here: https://review.openstack.org/Documentation/user-search.html
func SearchOwner(name string) *SearchTerm {
	return &SearchTerm{
		Key:   "owner",
		Value: name,
	}
}

func SearchCommit(commit string) *SearchTerm {
	return &SearchTerm{
		Key:   "commit",
		Value: commit,
	}
}

func SearchStatus(status string) *SearchTerm {
	return &SearchTerm{
		Key:   "status",
		Value: status,
	}
}

func SearchProject(project string) *SearchTerm {
	return &SearchTerm{
		Key:   "project",
		Value: project,
	}
}

func SearchLabel(label, value string) *SearchTerm {
	return &SearchTerm{
		Key:   "label",
		Value: fmt.Sprintf("%s=%s", label, value),
	}
}

// SearchModifiedAfter is a SearchTerm used for finding issues modified after
// a particular time.Time.
// API documentation is here: https://review.openstack.org/Documentation/user-search.html
func SearchModifiedAfter(after time.Time) *SearchTerm {
	return &SearchTerm{
		Key:   "after",
		Value: "\"" + strings.Trim(strings.Split(after.UTC().String(), "+")[0], " ") + "\"",
	}
}

// queryString encodes query parameters in the key:val[+key:val...] format specified here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
func queryString(terms []*SearchTerm) string {
	q := []string{}
	for _, t := range terms {
		q = append(q, fmt.Sprintf("%s:%s", t.Key, t.Value))
	}
	return strings.Join(q, " ")
}

// Sets a topic on the Gerrit change with the provided hash.
func (g *Gerrit) SetTopic(topic string, changeNum int64) error {
	putData := map[string]interface{}{
		"topic": topic,
	}
	b, err := json.Marshal(putData)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/a/changes/%d/topic", g.url, changeNum), bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Got status %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

// GetDependencies returns a slice of all dependencies around the specified change. See
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-related-changes
func (g *Gerrit) GetDependencies(changeNum int64, revision int) ([]*RelatedChangeAndCommitInfo, error) {
	data := RelatedChangesInfo{}
	err := g.get(fmt.Sprintf("/changes/%d/revisions/%d/related", changeNum, revision), &data, nil)
	if err != nil {
		return nil, err
	}
	return data.Changes, nil
}

// HasOpenDependency returns true if there is an active direct dependency of the specified change.
func (g *Gerrit) HasOpenDependency(changeNum int64, revision int) (bool, error) {
	dependencies, err := g.GetDependencies(changeNum, revision)
	if err != nil {
		return false, err
	}
	// Find the target change num in the chain of dependencies.
	targetChangeIdx := 0
	for idx, relatedChange := range dependencies {
		if relatedChange.Issue == changeNum {
			targetChangeIdx = idx
			break
		}
	}
	// See if the target change has an open dependency.
	if len(dependencies) > targetChangeIdx+1 {
		// The next change will be the direct dependency.
		dependency := dependencies[targetChangeIdx+1]
		if dependency.Status != CHANGE_STATUS_ABANDONED && dependency.Status != CHANGE_STATUS_MERGED {
			// If the dependency is not closed then it is an active dependency.
			return true, nil
		}
	}
	return false, nil
}

// Search returns a slice of Issues which fit the given criteria.
func (g *Gerrit) Search(limit int, terms ...*SearchTerm) ([]*ChangeInfo, error) {
	var issues changeListSortable
	for {
		data := make([]*ChangeInfo, 0)
		queryLimit := util.MinInt(limit-len(issues), MAX_GERRIT_LIMIT)
		skip := len(issues)

		q := url.Values{}
		q.Add("q", queryString(terms))
		q.Add("n", strconv.Itoa(queryLimit))
		q.Add("S", strconv.Itoa(skip))
		searchUrl := "/changes/?" + q.Encode()
		err := g.get(searchUrl, &data, nil)
		if err != nil {
			return nil, fmt.Errorf("Gerrit search failed: %v", err)
		}
		var moreChanges bool

		for _, issue := range data {
			// See if there are more changes available.
			moreChanges = issue.MoreChanges
			issues = append(issues, fixupChangeInfo(issue))
		}
		if len(issues) >= limit || !moreChanges {
			break
		}
	}

	sort.Sort(issues)
	return issues, nil
}

func (g *Gerrit) GetTrybotResults(issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return g.buildbucketClient.GetTrybotsForCL(issueID, patchsetID, "gerrit", g.url)
}

var revisionRegex = regexp.MustCompile("^[a-z0-9]+$")

// Files returns the files that were modified, added or deleted in a revision.
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-files
func (g *Gerrit) Files(issue int64, patch string) (map[string]*FileInfo, error) {
	if patch == "" {
		patch = "current"
	}
	if !revisionRegex.MatchString(patch) {
		return nil, fmt.Errorf("Invalid 'patch' value.")
	}
	url := fmt.Sprintf("/changes/%d/revisions/%s/files", issue, patch)
	files := map[string]*FileInfo{}
	if err := g.get(url, &files, ErrNotFound); err != nil {
		return nil, fmt.Errorf("Failed to list files for issue %d: %v", issue, err)
	}
	return files, nil
}

// GetFileNames returns the list of files for the given issue at the given patch. If
// patch is the empty string then the most recent patch is queried.
func (g *Gerrit) GetFileNames(issue int64, patch string) ([]string, error) {

	files, err := g.Files(issue, patch)
	if err != nil {
		return nil, err
	}
	// We only need the filenames.
	ret := []string{}
	for filename, _ := range files {
		ret = append(ret, filename)
	}
	return ret, nil
}

// IsBinaryPatch returns true if the patch contains any binary files.
func (g *Gerrit) IsBinaryPatch(issue int64, revision string) (bool, error) {
	files, err := g.Files(issue, revision)
	if err != nil {
		return false, err
	}
	for _, fileInfo := range files {
		if fileInfo.Binary {
			return true, nil
		}
	}
	return false, nil
}

// CodeReviewCache is an LRU cache for Gerrit Issues that polls in the background to determine if
// issues have been updated. If so it expells them from the cache to force a reload.
type CodeReviewCache struct {
	cache     *lru.Cache
	gerritAPI *Gerrit
	timeDelta time.Duration
	mutex     sync.Mutex
}

// NewCodeReviewCache returns a new cache for the given API instance, poll interval and maximum cache size.
func NewCodeReviewCache(gerritAPI *Gerrit, pollInterval time.Duration, cacheSize int) *CodeReviewCache {
	ret := &CodeReviewCache{
		cache:     lru.New(cacheSize),
		gerritAPI: gerritAPI,
		timeDelta: pollInterval * 2,
	}

	// Start the poller.
	go util.Repeat(pollInterval, nil, ret.poll)
	return ret
}

// Add an issue to the cache.
func (c *CodeReviewCache) Add(key int64, value *ChangeInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	sklog.Infof("\nAdding %d", key)
	c.cache.Add(key, value)
}

// Retrieve an issue from the cache.
func (c *CodeReviewCache) Get(key int64) (*ChangeInfo, bool) {
	sklog.Infof("\nGetting: %d", key)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if val, ok := c.cache.Get(key); ok {
		return val.(*ChangeInfo), true
	}
	return nil, false
}

// Poll Gerrit for all issues that have changed in the recent past.
func (c *CodeReviewCache) poll() {
	// Search for all keys that have changed in the last timeDelta duration.
	issues, err := c.gerritAPI.Search(10000, SearchModifiedAfter(time.Now().Add(-c.timeDelta)))
	if err != nil {
		sklog.Errorf("Error polling Gerrit: %s", err)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, issue := range issues {
		sklog.Infof("\nRemoving: %d", issue.Issue)
		c.cache.Remove(issue.Issue)
	}
}

// ContainsAny returns true if the provided ChangeInfo slice contains any
// change with the same issueID as id.
func ContainsAny(id int64, changes []*ChangeInfo) bool {
	for _, c := range changes {
		if id == c.Issue {
			return true
		}
	}
	return false
}

// Create a new Change in the given project, based on the given branch, and with
// the given subject line.
func (g *Gerrit) CreateChange(project, branch, subject, baseCommit string) (*ChangeInfo, error) {
	c := struct {
		Project    string `json:"project"`
		Subject    string `json:"subject"`
		Branch     string `json:"branch"`
		Topic      string `json:"topic"`
		Status     string `json:"status"`
		BaseCommit string `json:"base_commit"`
	}{
		Project:    project,
		Branch:     branch,
		Subject:    subject,
		Status:     "NEW",
		BaseCommit: baseCommit,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", g.url+"/a/changes/", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("Got status %s (%d): %s", resp.Status, resp.StatusCode, string(respBytes))
	}
	var ci ChangeInfo
	if err := json.NewDecoder(bytes.NewReader(respBytes[4:])).Decode(&ci); err != nil {
		return nil, err
	}
	return fixupChangeInfo(&ci), nil
}

// Modify the given file to have the given content. A ChangeEdit is created, if
// one is not already active. You must call PublishChangeEdit in order for the
// change to become a new patch set, otherwise it has no effect.
func (g *Gerrit) EditFile(ci *ChangeInfo, filepath, content string) error {
	url := g.url + fmt.Sprintf("/a/changes/%s/edit/%s", ci.Id, url.QueryEscape(filepath))
	b := []byte(content)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("Failed to create PUT request: %s", err)
	}

	if err := gitauth.AddAuthenticationCookie(g.gitCookiesPath, req); err != nil {
		return fmt.Errorf("Failed to add auth cookie: %s", err)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to execute request: %s", err)
	}
	defer util.Close(resp.Body)
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %s", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return fmt.Errorf("Got status %s (%d): %s", resp.Status, resp.StatusCode, string(respBytes))
	}
	return nil
}

// Move a given file. A ChangeEdit is created, if one is not already active.
// You must call PublishChangeEdit in order for the change to become a new patch
// set, otherwise it has no effect.
func (g *Gerrit) MoveFile(ci *ChangeInfo, oldPath, newPath string) error {
	data := struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}{
		OldPath: oldPath,
		NewPath: newPath,
	}
	return g.postJson(fmt.Sprintf("/a/changes/%s/edit", ci.Id), data)
}

// Delete the given file. A ChangeEdit is created, if one is not already active.
// You must call PublishChangeEdit in order for the change to become a new patch
// set, otherwise it has no effect.
func (g *Gerrit) DeleteFile(ci *ChangeInfo, filepath string) error {
	return g.delete(fmt.Sprintf("/a/changes/%s/edit/%s", ci.Id, url.QueryEscape(filepath)))
}

// Set the commit message for the ChangeEdit. A ChangeEdit is created, if one is
// not already active. You must call PublishChangeEdit in order for the change
// to become a new patch set, otherwise it has no effect.
func (g *Gerrit) SetCommitMessage(ci *ChangeInfo, msg string) error {
	m := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	url := fmt.Sprintf("/a/changes/%s/edit:message", ci.Id)
	return g.putJson(url, m)
}

// Publish the active ChangeEdit as a new patch set.
func (g *Gerrit) PublishChangeEdit(ci *ChangeInfo) error {
	msg := struct {
		Notify string `json:"notify,omitempty"`
	}{
		Notify: "ALL",
	}
	url := fmt.Sprintf("/a/changes/%s/edit:publish", ci.Id)
	return g.postJson(url, msg)
}

// Delete the active ChangeEdit, restoring the state to the last patch set.
func (g *Gerrit) DeleteChangeEdit(ci *ChangeInfo) error {
	return g.delete(fmt.Sprintf("/a/changes/%s/edit", ci.Id))
}
