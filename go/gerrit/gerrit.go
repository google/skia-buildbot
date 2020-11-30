package gerrit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
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
	CODEREVIEW_LABEL              = "Code-Review"
	CODEREVIEW_LABEL_DISAPPROVE   = -1
	CODEREVIEW_LABEL_NONE         = 0
	CODEREVIEW_LABEL_APPROVE      = 1
	CODEREVIEW_LABEL_SELF_APPROVE = 2 // Used by ANGLE, not Chromium or Skia.

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
	PRESUBMIT_READY_LABEL_NONE        = 0
	PRESUBMIT_READY_LABEL_ENABLE      = 1
	PRESUBMIT_VERIFIED_LABEL          = "Presubmit-Verified"
	PRESUBMIT_VERIFIED_LABEL_REJECTED = -1
	PRESUBMIT_VERIFIED_LABEL_RUNNING  = 0
	PRESUBMIT_VERIFIED_LABEL_ACCEPTED = 1

	// Some Gerrit hosts use the "Verified" label instead of "Presubmit-Verified".
	VERIFIED_LABEL          = "Verified"
	VERIFIED_LABEL_REJECTED = -1
	VERIFIED_LABEL_RUNNING  = 0
	VERIFIED_LABEL_ACCEPTED = 1

	URL_TMPL_CHANGE     = "/changes/%s/detail?o=ALL_REVISIONS"
	URL_COMMIT_MSG_HOOK = "/tools/hooks/commit-msg"

	// Kinds of patchsets.
	PATCHSET_KIND_MERGE_FIRST_PARENT_UPDATE = "MERGE_FIRST_PARENT_UPDATE"
	PATCHSET_KIND_NO_CHANGE                 = "NO_CHANGE"
	PATCHSET_KIND_NO_CODE_CHANGE            = "NO_CODE_CHANGE"
	PATCHSET_KIND_REWORK                    = "REWORK"
	PATCHSET_KIND_TRIVIAL_REBASE            = "TRIVIAL_REBASE"

	// authSuffix is added to the Gerrit API URL to force authentication.
	authSuffix = "/a"

	// extractReg is the regular expression used by ExtractIssueFromCommit.
	extractRegTmpl = `^\s*Reviewed-on:.*%s.*/([0-9]+)\s*$`

	// Change refs in Gerrit are of this form-
	//  refs/changes/46/4546/1
	//                |  |   |
	//                |  |   +-> Patch set.
	//                |  |
	//                |  +-> Issue ID.
	//                |
	//                +-> Last two digits of Issue ID.
	CHANGE_REF_PREFIX = "refs/changes/"
)

var (
	TRIVIAL_PATCHSET_KINDS = []string{
		PATCHSET_KIND_TRIVIAL_REBASE,
		PATCHSET_KIND_NO_CHANGE,
		PATCHSET_KIND_NO_CODE_CHANGE,
	}

	changeIdRegex = regexp.MustCompile(`\s*Change-Id:\s*(\w+)`)
)

// ChangeInfoMessage contains information about Gerrit messages.
type ChangeInfoMessage struct {
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

// ChangeInfo contains information about a Gerrit issue.
type ChangeInfo struct {
	Id              string              `json:"id"`
	Insertions      int                 `json:"insertions"`
	Deletions       int                 `json:"deletions"`
	Created         time.Time           `json:"-"`
	CreatedString   string              `json:"created"`
	Updated         time.Time           `json:"-"`
	UpdatedString   string              `json:"updated"`
	Submitted       time.Time           `json:"-"`
	SubmittedString string              `json:"submitted"`
	Project         string              `json:"project"`
	ChangeId        string              `json:"change_id"`
	Subject         string              `json:"subject"`
	Branch          string              `json:"branch"`
	Committed       bool                `json:"committed"`
	Messages        []ChangeInfoMessage `json:"messages"`
	Reviewers       struct {
		CC       []*Person `json:"CC"`
		Reviewer []*Person `json:"REVIEWER"`
	} `json:"reviewers"`
	Revisions      map[string]*Revision   `json:"revisions"`
	Patchsets      []*Revision            `json:"-"`
	MoreChanges    bool                   `json:"_more_changes"`
	Issue          int64                  `json:"_number"`
	Labels         map[string]*LabelEntry `json:"labels"`
	Owner          *Person                `json:"owner"`
	Status         string                 `json:"status"`
	WorkInProgress bool                   `json:"work_in_progress"`
}

// Find the set of non-trivial patchsets. Returns the Revisions in order of
// patchset number. Note that this is only correct for Chromium Gerrit instances
// because it makes Chromium-specific assumptions.
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
		// generated for Chromium projects.
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
func (ci *ChangeInfo) IsClosed() bool {
	return (ci.Status == CHANGE_STATUS_ABANDONED ||
		ci.Status == CHANGE_STATUS_MERGED)
}

// IsMerged returns true iff the issue corresponding to the ChangeInfo is
// merged.
func (ci *ChangeInfo) IsMerged() bool {
	return ci.Status == CHANGE_STATUS_MERGED
}

// GetAbandonReason returns the reason entered by the user that abandoned the change.
func (ci *ChangeInfo) GetAbandonReason(ctx context.Context) string {
	if ci.Status != CHANGE_STATUS_ABANDONED {
		// There is no abandon reason if the change isn't abandoned.
		return ""
	}
	for i := len(ci.Messages) - 1; i >= 0; i-- {
		msg := ci.Messages[i]
		if msg.Tag != "autogenerated:gerrit:abandon" {
			continue
		}
		if msg.Message == "Abandoned" {
			// An abandon reason wasn't provided.
			return ""
		}
		return strings.TrimPrefix(msg.Message, "Abandoned\n\n")
	}
	return ""
}

// Person describes a person in Gerrit.
type Person struct {
	AccountID int    `json:"_account_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
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
	Abandon(context.Context, *ChangeInfo, string) error
	AddComment(context.Context, *ChangeInfo, string) error
	AddCC(context.Context, *ChangeInfo, []string) error
	Approve(context.Context, *ChangeInfo, string) error
	Config() *Config
	CreateChange(context.Context, string, string, string, string) (*ChangeInfo, error)
	DeleteChangeEdit(context.Context, *ChangeInfo) error
	DeleteFile(context.Context, *ChangeInfo, string) error
	DisApprove(context.Context, *ChangeInfo, string) error
	DownloadCommitMsgHook(ctx context.Context, dest string) error
	EditFile(context.Context, *ChangeInfo, string, string) error
	ExtractIssueFromCommit(string) (int64, error)
	Files(ctx context.Context, issue int64, patch string) (map[string]*FileInfo, error)
	GetChange(ctx context.Context, id string) (*ChangeInfo, error)
	GetFileNames(ctx context.Context, issue int64, patch string) ([]string, error)
	GetIssueProperties(context.Context, int64) (*ChangeInfo, error)
	GetPatch(context.Context, int64, string) (string, error)
	GetRepoUrl() string
	GetTrybotResults(context.Context, int64, int64) ([]*buildbucketpb.Build, error)
	GetUserEmail(context.Context) (string, error)
	Initialized() bool
	IsBinaryPatch(ctx context.Context, issue int64, patch string) (bool, error)
	MoveFile(context.Context, *ChangeInfo, string, string) error
	NoScore(context.Context, *ChangeInfo, string) error
	PublishChangeEdit(context.Context, *ChangeInfo) error
	RemoveFromCQ(context.Context, *ChangeInfo, string) error
	Search(context.Context, int, bool, ...*SearchTerm) ([]*ChangeInfo, error)
	SelfApprove(context.Context, *ChangeInfo, string) error
	SendToCQ(context.Context, *ChangeInfo, string) error
	SendToDryRun(context.Context, *ChangeInfo, string) error
	SetCommitMessage(context.Context, *ChangeInfo, string) error
	SetReadyForReview(context.Context, *ChangeInfo) error
	SetReview(context.Context, *ChangeInfo, string, map[string]int, []string) error
	SetTopic(context.Context, string, int64) error
	Submit(context.Context, *ChangeInfo) error
	Url(int64) string
}

// Gerrit is an object used for interacting with the issue tracker.
type Gerrit struct {
	cfg               *Config
	client            *http.Client
	BuildbucketClient *buildbucket.Client
	apiUrl            string
	baseUrl           string
	repoUrl           string
	extractRegEx      *regexp.Regexp
}

// NewGerrit returns a new Gerrit instance.
func NewGerrit(gerritUrl string, client *http.Client) (*Gerrit, error) {
	return NewGerritWithConfig(CONFIG_CHROMIUM, gerritUrl, client)
}

// NewGerritWithConfig returns a new Gerrit instance which uses the given
// Config.
func NewGerritWithConfig(cfg *Config, gerritUrl string, client *http.Client) (*Gerrit, error) {
	parsedUrl, err := url.Parse(gerritUrl)
	if err != nil {
		return nil, skerr.Fmt("Unable to parse gerrit URL: %s", err)
	}

	regExStr := fmt.Sprintf(extractRegTmpl, parsedUrl.Host)
	extractRegEx, err := regexp.Compile(regExStr)
	if err != nil {
		return nil, skerr.Fmt("Unable to compile regular expression '%s'. Error: %s", regExStr, err)
	}

	if client == nil {
		return nil, skerr.Fmt("Gerrit requires a non-nil authenticated http.Client with the Gerrit scope.")
	}
	baseUrl := strings.TrimSuffix(gerritUrl, "/")
	return &Gerrit{
		cfg:               cfg,
		apiUrl:            baseUrl + authSuffix,
		baseUrl:           baseUrl,
		repoUrl:           strings.Replace(baseUrl, "-review", "", 1),
		client:            client,
		BuildbucketClient: buildbucket.NewClient(client),
		extractRegEx:      extractRegEx,
	}, nil
}

// Config returns the Config object used by this Gerrit.
func (g *Gerrit) Config() *Config {
	return g.cfg
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
		return "", skerr.Fmt("Unable to retrieve user for git_auth_deamon default cookie path.")
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

// Url returns the url of the Gerrit issue identified by issueID or the
// base URL of the Gerrit instance if issueID is 0.
func (g *Gerrit) Url(issueID int64) string {
	if issueID == 0 {
		return g.baseUrl
	}
	return fmt.Sprintf("%s/c/%d", g.baseUrl, issueID)
}

type AccountDetails struct {
	AccountId int64  `json:"_account_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	UserName  string `json:"username"`
}

// GetUserEmail returns the Gerrit user's email address.
func (g *Gerrit) GetUserEmail(ctx context.Context) (string, error) {
	url := "/accounts/self/detail"
	var account AccountDetails
	if err := g.get(ctx, url, &account, nil); err != nil {
		return "", fmt.Errorf("Failed to retrieve user: %s", err)
	}
	return account.Email, nil
}

// GetRepoUrl returns the url of the Googlesource repo.
func (g *Gerrit) GetRepoUrl() string {
	return g.repoUrl
}

// ExtractIssueFromCommit returns the issue id by parsing the commit message of
// a landed commit. It expects the commit message to contain one line in this format:
//
//     Reviewed-on: https://skia-review.googlesource.com/999999
//
// where the digits at the end are the issue id.
func (g *Gerrit) ExtractIssueFromCommit(commitMsg string) (int64, error) {
	scanner := bufio.NewScanner(strings.NewReader(commitMsg))
	for scanner.Scan() {
		line := scanner.Text()
		// Reminder, this regex has the review url (e.g. skia-review.googlesource.com) baked into it.
		result := g.extractRegEx.FindStringSubmatch(line)
		if len(result) == 2 {
			ret, err := strconv.ParseInt(result[1], 10, 64)
			if err != nil {
				return 0, skerr.Wrapf(err, "parsing issue id '%s'", result[1])
			}

			return ret, nil
		}
	}
	return 0, skerr.Fmt("unable to find Reviewed-on line")
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
func (g *Gerrit) GetIssueProperties(ctx context.Context, issue int64) (*ChangeInfo, error) {
	return g.GetChange(ctx, fmt.Sprintf("%d", issue))
}

// GetChange returns the ChangeInfo object for the given ID.
func (g *Gerrit) GetChange(ctx context.Context, id string) (*ChangeInfo, error) {
	url := fmt.Sprintf(URL_TMPL_CHANGE, id)
	fullIssue := &ChangeInfo{}
	if err := g.get(ctx, url, fullIssue, ErrNotFound); err != nil {
		// Pass ErrNotFound through unchanged so calling functions can check for it.
		if err == ErrNotFound {
			return nil, err
		}
		return nil, skerr.Fmt("Failed to load details for issue %q: %v", id, err)
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
func (g *Gerrit) GetPatch(ctx context.Context, issue int64, revision string) (string, error) {
	u := fmt.Sprintf("%s/changes/%d/revisions/%s/patch", g.apiUrl, issue, revision)
	resp, err := httputils.GetWithContext(ctx, g.client, u)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %s: %s", u, err)
	}
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("Issue not found: %s", u)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Error retrieving %s: %d %s", u, resp.StatusCode, resp.Status)
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
	Subject string        `json:"subject"`
	Message string        `json:"message"`
}

// GetCommit retrieves the commit that corresponds to the patch identified by issue and revision.
// It allows to retrieve the parent commit on which the given patchset is based on.
// See: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-commit
func (g *Gerrit) GetCommit(ctx context.Context, issue int64, revision string) (*CommitInfo, error) {
	path := fmt.Sprintf("/changes/%d/revisions/%s/commit", issue, revision)
	ret := &CommitInfo{}
	err := g.get(ctx, path, ret, nil)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

type reviewer struct {
	Reviewer string `json:"reviewer"`
}

// SetReview calls the Set Review endpoint of the Gerrit API to add messages and/or set labels for
// the latest patchset.
// API documentation: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
func (g *Gerrit) SetReview(ctx context.Context, issue *ChangeInfo, message string, labels map[string]int, reviewers []string) error {
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
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/revisions/%s/review", issue.ChangeId, latestPatchset.ID), postData)
}

type reviewerWithState struct {
	Reviewer string `json:"reviewer"`
	State    string `json:"state"`
}

// AddCC adds CCs to the issues.
func (g *Gerrit) AddCC(ctx context.Context, issue *ChangeInfo, ccList []string) error {
	ccs := make([]*reviewerWithState, 0, len(ccList))
	for _, c := range ccList {
		ccs = append(ccs, &reviewerWithState{
			Reviewer: c,
			State:    "CC",
		})
	}
	postData := map[string]interface{}{"reviewers": ccs}
	latestPatchset := issue.Patchsets[len(issue.Patchsets)-1]
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/revisions/%s/review", issue.ChangeId, latestPatchset.ID), postData)
}

// AddComment adds a message to the issue.
func (g *Gerrit) AddComment(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{}, nil)
}

// Utility methods for interacting with the COMMITQUEUE_LABEL.

func (g *Gerrit) SendToDryRun(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SetDryRunLabels, nil)
}

func (g *Gerrit) SendToCQ(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SetCqLabels, nil)
}

func (g *Gerrit) RemoveFromCQ(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.NoCqLabels, nil)
}

// Utility methods for interacting with the CODEREVIEW_LABEL.

func (g *Gerrit) Approve(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{CODEREVIEW_LABEL: CODEREVIEW_LABEL_APPROVE}, nil)
}

func (g *Gerrit) NoScore(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{CODEREVIEW_LABEL: CODEREVIEW_LABEL_NONE}, nil)
}

func (g *Gerrit) DisApprove(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{CODEREVIEW_LABEL: CODEREVIEW_LABEL_DISAPPROVE}, nil)
}

func (g *Gerrit) SelfApprove(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SelfApproveLabels, nil)
}

// Abandon abandons the issue with the given message.
func (g *Gerrit) Abandon(ctx context.Context, issue *ChangeInfo, message string) error {
	postData := map[string]interface{}{
		"message": message,
	}
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/abandon", issue.ChangeId), postData)
}

// get retrieves the given sub URL and populates 'rv' with the result.
// If notFoundError is not nil it will be returned if the requested item doesn't
// exist.
func (g *Gerrit) get(ctx context.Context, suburl string, rv interface{}, notFoundError error) error {
	getURL := g.apiUrl + suburl
	resp, err := httputils.GetWithContext(ctx, g.client, getURL)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", getURL, err)
	}
	if resp.StatusCode == 404 {
		if notFoundError != nil {
			return notFoundError
		}
		return fmt.Errorf("Issue not found: %s", getURL)
	}
	defer util.Close(resp.Body)
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Could not read response body: %s", err)
	}
	body := string(bodyBytes)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error retrieving %s: %d %s; response:\n%s", getURL, resp.StatusCode, resp.Status, body)
	}

	// Strip off the XSS protection chars.
	parts := strings.SplitN(body, "\n", 2)

	if len(parts) != 2 {
		return fmt.Errorf("Reponse invalid format; response:\n%s", body)
	}
	if err := json.Unmarshal([]byte(parts[1]), &rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s; response:\n%s", err, body)
	}
	return nil
}

func (g *Gerrit) post(ctx context.Context, suburl string, b []byte) error {
	resp, err := httputils.PostWithContext(ctx, g.client, g.apiUrl+suburl, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Could not read response body: %s", err)
	}
	body := string(bodyBytes)
	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		return fmt.Errorf("Got status %s (%d); response:\n%s", resp.Status, resp.StatusCode, body)
	}
	return nil
}

func (g *Gerrit) postJson(ctx context.Context, suburl string, postData interface{}) error {
	b, err := json.Marshal(postData)
	if err != nil {
		return err
	}
	return g.post(ctx, suburl, b)
}

func (g *Gerrit) put(ctx context.Context, suburl string, b []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, g.apiUrl+suburl, bytes.NewReader(b))
	if err != nil {
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

func (g *Gerrit) putJson(ctx context.Context, suburl string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return g.put(ctx, suburl, b)
}

func (g *Gerrit) delete(ctx context.Context, suburl string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, g.apiUrl+suburl, nil)
	if err != nil {
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

// SetTopic sets a topic on the Gerrit change with the provided id.
func (g *Gerrit) SetTopic(ctx context.Context, topic string, changeNum int64) error {
	putData := map[string]interface{}{
		"topic": topic,
	}
	b, err := json.Marshal(putData)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/changes/%d/topic", g.apiUrl, changeNum), bytes.NewReader(b))
	if err != nil {
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
func (g *Gerrit) GetDependencies(ctx context.Context, changeNum int64, revision int) ([]*RelatedChangeAndCommitInfo, error) {
	data := RelatedChangesInfo{}
	err := g.get(ctx, fmt.Sprintf("/changes/%d/revisions/%d/related", changeNum, revision), &data, nil)
	if err != nil {
		return nil, err
	}
	return data.Changes, nil
}

// HasOpenDependency returns true if there is an active direct dependency of the specified change.
func (g *Gerrit) HasOpenDependency(ctx context.Context, changeNum int64, revision int) (bool, error) {
	dependencies, err := g.GetDependencies(ctx, changeNum, revision)
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
func (g *Gerrit) Search(ctx context.Context, limit int, sortResults bool, terms ...*SearchTerm) ([]*ChangeInfo, error) {
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
		err := g.get(ctx, searchUrl, &data, nil)
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

	if sortResults {
		sort.Sort(issues)
	}
	return issues, nil
}

func (g *Gerrit) GetTrybotResults(ctx context.Context, issueID int64, patchsetID int64) ([]*buildbucketpb.Build, error) {
	return g.BuildbucketClient.GetTrybotsForCL(ctx, issueID, patchsetID, g.baseUrl)
}

// SetReadyForReview marks the change as ready for review (ie, not WIP).
func (g *Gerrit) SetReadyForReview(ctx context.Context, ci *ChangeInfo) error {
	return g.post(ctx, fmt.Sprintf("/changes/%d/ready", ci.Issue), []byte("{}"))
}

var revisionRegex = regexp.MustCompile("^[a-z0-9]+$")

// Files returns the files that were modified, added or deleted in a revision.
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-files
func (g *Gerrit) Files(ctx context.Context, issue int64, patch string) (map[string]*FileInfo, error) {
	if patch == "" {
		patch = "current"
	}
	if !revisionRegex.MatchString(patch) {
		return nil, fmt.Errorf("Invalid 'patch' value.")
	}
	url := fmt.Sprintf("/changes/%d/revisions/%s/files", issue, patch)
	files := map[string]*FileInfo{}
	if err := g.get(ctx, url, &files, ErrNotFound); err != nil {
		return nil, fmt.Errorf("Failed to list files for issue %d: %v", issue, err)
	}
	return files, nil
}

// GetFileNames returns the list of files for the given issue at the given patch. If
// patch is the empty string then the most recent patch is queried.
func (g *Gerrit) GetFileNames(ctx context.Context, issue int64, patch string) ([]string, error) {
	files, err := g.Files(ctx, issue, patch)
	if err != nil {
		return nil, err
	}
	// We only need the filenames.
	ret := []string{}
	for filename := range files {
		ret = append(ret, filename)
	}
	return ret, nil
}

// IsBinaryPatch returns true if the patch contains any binary files.
func (g *Gerrit) IsBinaryPatch(ctx context.Context, issue int64, revision string) (bool, error) {
	files, err := g.Files(ctx, issue, revision)
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

// Submit submits the Change.
func (g *Gerrit) Submit(ctx context.Context, ci *ChangeInfo) error {
	return g.post(ctx, fmt.Sprintf("/changes/%d/submit", ci.Issue), []byte("{}"))
}

// Download the commit message hook to the specified location.
func (g *Gerrit) DownloadCommitMsgHook(ctx context.Context, dest string) error {
	url := g.apiUrl + URL_COMMIT_MSG_HOOK
	resp, err := httputils.GetWithContext(ctx, g.client, url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", url, err)
	}
	defer util.Close(resp.Body)
	if err := util.WithWriteFile(dest, func(w io.Writer) error {
		_, err := io.Copy(w, resp.Body)
		return err
	}); err != nil {
		return err
	}
	return os.Chmod(dest, 0755)
}

// CodeReviewCache is an LRU cache for Gerrit Issues that polls in the background to determine if
// issues have been updated. If so it expels them from the cache to force a reload.
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
	issues, err := c.gerritAPI.Search(context.Background(), 10000, true, SearchModifiedAfter(time.Now().Add(-c.timeDelta)))
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

// CreateChange creates a new Change in the given project, based on the given branch, and with
// the given subject line.
func (g *Gerrit) CreateChange(ctx context.Context, project, branch, subject, baseCommit string) (*ChangeInfo, error) {
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
	resp, err := httputils.PostWithContext(ctx, g.client, g.apiUrl+"/changes/", "application/json", bytes.NewReader(b))
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

// EditFile modifies the given file to have the given content. A ChangeEdit is created, if
// one is not already active. You must call PublishChangeEdit in order for the
// change to become a new patch set, otherwise it has no effect.
func (g *Gerrit) EditFile(ctx context.Context, ci *ChangeInfo, filepath, content string) error {
	u := g.apiUrl + fmt.Sprintf("/changes/%s/edit/%s", ci.Id, url.QueryEscape(filepath))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("Failed to create PUT request: %s", err)
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

// MoveFile moves a given file. A ChangeEdit is created, if one is not already active.
// You must call PublishChangeEdit in order for the change to become a new patch
// set, otherwise it has no effect.
func (g *Gerrit) MoveFile(ctx context.Context, ci *ChangeInfo, oldPath, newPath string) error {
	data := struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}{
		OldPath: oldPath,
		NewPath: newPath,
	}
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/edit", ci.Id), data)
}

// DeleteFile deletes the given file. A ChangeEdit is created, if one is not already active.
// You must call PublishChangeEdit in order for the change to become a new patch
// set, otherwise it has no effect.
func (g *Gerrit) DeleteFile(ctx context.Context, ci *ChangeInfo, filepath string) error {
	return g.delete(ctx, fmt.Sprintf("/changes/%s/edit/%s", ci.Id, url.QueryEscape(filepath)))
}

// SetCommitMessage sets the commit message for the ChangeEdit. A ChangeEdit is created, if one is
// not already active. You must call PublishChangeEdit in order for the change
// to become a new patch set, otherwise it has no effect.
func (g *Gerrit) SetCommitMessage(ctx context.Context, ci *ChangeInfo, msg string) error {
	m := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	u := fmt.Sprintf("/changes/%s/edit:message", ci.Id)
	return g.putJson(ctx, u, m)
}

// PublishChangeEdit publishes the active ChangeEdit as a new patch set.
func (g *Gerrit) PublishChangeEdit(ctx context.Context, ci *ChangeInfo) error {
	msg := struct {
		Notify string `json:"notify,omitempty"`
	}{
		Notify: "ALL",
	}
	u := fmt.Sprintf("/changes/%s/edit:publish", ci.Id)
	return g.postJson(ctx, u, msg)
}

// DeleteChangeEdit deletes the active ChangeEdit, restoring the state to the last patch set.
func (g *Gerrit) DeleteChangeEdit(ctx context.Context, ci *ChangeInfo) error {
	return g.delete(ctx, fmt.Sprintf("/changes/%s/edit", ci.Id))
}

// SetLabel sets the given label on the ChangeInfo.
func SetLabel(ci *ChangeInfo, key string, value int) {
	labelEntry, ok := ci.Labels[key]
	if !ok {
		labelEntry = &LabelEntry{
			All: []*LabelDetail{},
		}
		ci.Labels[key] = labelEntry
	}
	labelEntry.All = append(labelEntry.All, &LabelDetail{
		Value: value,
	})
}

// SetLabels sets the given labels on the ChangeInfo.
func SetLabels(ci *ChangeInfo, labels map[string]int) {
	for key, value := range labels {
		SetLabel(ci, key, value)
	}
}

// UnsetLabel unsets the given label on the ChangeInfo.
func UnsetLabel(ci *ChangeInfo, key string, value int) {
	labelEntry, ok := ci.Labels[key]
	if !ok {
		return
	}
	newEntries := make([]*LabelDetail, 0, len(labelEntry.All))
	for _, details := range labelEntry.All {
		if details.Value != value {
			newEntries = append(newEntries, details)
		}
	}
	labelEntry.All = newEntries
}

// UnsetLabels unsets the given labels on the ChangeInfo.
func UnsetLabels(ci *ChangeInfo, labels map[string]int) {
	for key, value := range labels {
		UnsetLabel(ci, key, value)
	}
}

// ParseChangeId parses the change ID out of the given commit message.
func ParseChangeId(msg string) (string, error) {
	for _, line := range strings.Split(msg, "\n") {
		m := changeIdRegex.FindStringSubmatch(line)
		if m != nil && len(m) == 2 {
			return m[1], nil
		}
	}
	return "", skerr.Fmt("Failed to parse Change-Id from commit message.")
}
