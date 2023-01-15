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
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/time/rate"
)

var (
	// ErrNotFound indicates that the requested item was not found.
	ErrNotFound = errors.New("Requested item was not found")
)

const (
	// TimeFormat is the timestamp format used by the Gerrit API.
	TimeFormat = "2006-01-02 15:04:05.999999"
	// GerritSkiaURL is the URL of Skia's Gerrit instance.
	GerritSkiaURL        = "https://skia-review.googlesource.com"
	maxSearchResultLimit = 500

	// AuthScope is the auth scope needed to use the Gerrit API.
	AuthScope = auth.ScopeGerrit

	// ChangeStatusAbandoned indicates the the change is abandoned.
	ChangeStatusAbandoned = "ABANDONED"
	// ChangeStatusMerged indicates the the change is merged.
	ChangeStatusMerged = "MERGED"
	// ChangeStatusNew indicates the the change is new.
	ChangeStatusNew = "NEW"
	// ChangeStatusNew indicates the the change is open.
	ChangeStatusOpen = "OPEN"

	// LabelCodeReview is the label used for code review.
	LabelCodeReview = "Code-Review"
	// LabelCodeReviewDisapprove indicates code review disapproval.
	LabelCodeReviewDisapprove = -1
	// LabelCodeReviewNone indicates that the change has not been code reviewed.
	LabelCodeReviewNone = 0
	// LabelCodeReviewApprove indicates code review approval.
	LabelCodeReviewApprove = 1
	// LabelCodeReviewSelfApprove indicates code review self-approval.
	LabelCodeReviewSelfApprove = 2

	// LabelCommitQueue is the label used for the commit queue.
	LabelCommitQueue = "Commit-Queue"
	// LabelCommitQueueNone indicates that the commit queue is not running for
	// this change.
	LabelCommitQueueNone = 0
	// LabelCommitQueueDryRun indicates that the commit queue should run in dry
	// run mode for this change.
	LabelCommitQueueDryRun = 1
	// LabelCommitQueueSubmit indicates that the commit queue should run for
	// this change.
	LabelCommitQueueSubmit = 2

	// LabelAndroidAutoSubmit indicates whether the change should be submitted
	// when it is approved.  For Android hosts only.
	LabelAndroidAutoSubmit = "Autosubmit"
	// LabelAndroidAutoSubmitNone indicates that the change should not be
	// submitted when it is approved.
	LabelAndroidAutoSubmitNone = 0
	// LabelAndroidAutoSubmitSubmit indicates that the change should be
	// submitted when it is approved.
	LabelAndroidAutoSubmitSubmit = 1

	// LabelChromiumAutoSubmit indicates whether the change should be submitted
	// when it is approved.  For Chromium hosts only.
	LabelChromiumAutoSubmit = "Auto-Submit"
	// LabelChromiumAutoSubmitNone indicates that the change should not be
	// submitted when it is approved.
	LabelChromiumAutoSubmitNone = 0
	// LabelChromiumAutoSubmitSubmit indicates that the change should be
	// submitted when it is approved.
	LabelChromiumAutoSubmitSubmit = 1

	// LabelPresubmitReady indicates whether the presubmit checks should run for
	// this change.
	LabelPresubmitReady = "Presubmit-Ready"
	// LabelPresubmitReadyNone indicates that the presubmit checks should not
	// run for this change.
	LabelPresubmitReadyNone = 0
	// LabelPresubmitReadyEnable indicates that the presubmit checks should run
	// for this change.
	LabelPresubmitReadyEnable = 1

	// LabelPresubmitVerified indicates whether the presubmit checks ran
	// successfully for this change.
	LabelPresubmitVerified = "Presubmit-Verified"
	// LabelPresubmitVerifiedRejected indicates that the presubmit checks failed
	// for this change.
	LabelPresubmitVerifiedRejected = -1
	// LabelPresubmitVerifiedRunning indicates that the presubmit checks have
	// not finished for this change.
	LabelPresubmitVerifiedRunning = 0
	// LabelPresubmitVerifiedAccepted indicates that the presubmit checks
	// succeeded for this change.
	LabelPresubmitVerifiedAccepted = 2

	// LabelVerified indicates whether the presubmit checks ran successfully for
	// this change.
	LabelVerified = "Verified"
	// LabelVerifiedRejected indicates that the presubmit checks failed for this
	// change.
	LabelVerifiedRejected = -1
	// LabelVerifiedRunning indicates that the presubmit checks have not
	// finished for this change.
	LabelVerifiedRunning = 0
	// LabelVerifiedAccepted indicates that the presubmit checks succeeded for
	// this change.
	LabelVerifiedAccepted = 1

	// LabelBotCommit indicates self-approval by a trusted bot.
	LabelBotCommit = "Bot-Commit"
	// LabelBotCommitNone indicates that the change is not self-approved by a
	// trusted bot.
	LabelBotCommitNone = 0
	// LabelBotCommitApproved indicates that the change is self-approved by a
	// trusted bot.
	LabelBotCommitApproved = 1

	// URLTmplChange is the template for a change URL.
	URLTmplChange = "/changes/%s/detail?o=ALL_REVISIONS&o=SUBMITTABLE"

	urlCommitMsgHook = "/tools/hooks/commit-msg"

	// Kinds of patchsets.
	PatchSetKindMergeFirstParentUpdate = "MERGE_FIRST_PARENT_UPDATE"
	PatchSetKindNoChange               = "NO_CHANGE"
	PatchSetKindNoCodeChange           = "NO_CODE_CHANGE"
	PatchSetKindCodeChange             = "CODE_CHANGE"
	PatchSetKindRework                 = "REWORK"
	PatchSetKindTrivialRebase          = "TRIVIAL_REBASE"

	// authSuffix is added to the Gerrit API URL to force authentication.
	authSuffix = "/a"

	// extractReg is the regular expression used by ExtractIssueFromCommit.
	extractRegTmpl = `^\s*Reviewed-on:.*%s.*/([0-9]+)\s*$`

	// ChangeRefPrefix is the prefix used by change refs in Gerrit, which are of
	// this form:
	//
	//  refs/changes/46/4546/1
	//                |  |   |
	//                |  |   +-> Patch set.
	//                |  |
	//                |  +-> Issue ID.
	//                |
	//                +-> Last two digits of Issue ID.
	ChangeRefPrefix = "refs/changes/"

	// ErrMergeConflict as a substring of an error message indicates that a
	// merge conflict occurred.
	ErrMergeConflict = "conflict during merge"

	// ErrUnsubmittedDependend as a substring of an error message indicates
	// that a dependend CL has not been submitted yet.
	ErrUnsubmittedDependend = "Depends on change that was not submitted"

	// ErrNoChanges as a substring of an error message indicates that there were
	// no changes to apply. Generally we can ignore this error.
	ErrNoChanges = "no changes were made"

	// These were copied from the defaults used by gitfs:
	// https://gerrit.googlesource.com/gitfs/+show/59c1163fd1737445281f2339399b2b986b0d30fe/gitiles/client.go#102
	// Hopefully they apply to Gerrit as well.
	defaultMaxQPS   = 4.0
	defaultMaxBurst = 40

	// Gerrit's magic path for the commit message. See:
	// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#file-id
	CommitMsgFileName = "/COMMIT_MSG"

	// HTTP header used to enable tracing.
	HeaderTracing = "X-Gerrit-Trace"
)

var (
	TrivialPatchSetKinds = []string{
		PatchSetKindTrivialRebase,
		PatchSetKindNoChange,
		PatchSetKindNoCodeChange,
	}

	changeIdRegex = regexp.MustCompile(`\s*Change-Id:\s*(\w+)`)
)

// The different notify options supported by Gerrit. See the notify property
// in https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#review-input
type NotifyOption string

const NotifyNone NotifyOption = "NONE"
const NotifyOwner NotifyOption = "OWNER"
const NotifyOwnerReviewers NotifyOption = "OWNER_REVIEWERS"
const NotifyAll NotifyOption = "ALL"
const NotifyDefault NotifyOption = ""

// The different recipient types supported by Gerrit.
// https://gerrit-review.googlesource.com/Documentation/user-notify.html#recipient-types
type RecipientType string

const RecipientTo = "TO"
const RecipientCC = "CC"
const RecipientBCC = "BCC"

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
	Revisions            map[string]*Revision   `json:"revisions"`
	Patchsets            []*Revision            `json:"-"`
	MoreChanges          bool                   `json:"_more_changes"`
	Issue                int64                  `json:"_number"`
	Labels               map[string]*LabelEntry `json:"labels"`
	Owner                *Person                `json:"owner"`
	Status               string                 `json:"status"`
	Submittable          bool                   `json:"submittable"`
	Topic                string                 `json:"topic"`
	WorkInProgress       bool                   `json:"work_in_progress"`
	CherrypickOfChange   int                    `json:"cherry_pick_of_change"`
	CherrypickOfPatchSet int                    `json:"cherry_pick_of_patch_set"`
}

// GetNonTrivialPatchSets finds the set of non-trivial patchsets. Returns the
// Revisions in order of patchset number. Note that this is only correct for
// Chromium Gerrit instances because it makes Chromium-specific assumptions.
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
		if ci.Status == ChangeStatusMerged && idx == len(allPatchSets)-1 {
			continue
		}
		if !util.In(rev.Kind, TrivialPatchSetKinds) {
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
	return (ci.Status == ChangeStatusAbandoned ||
		ci.Status == ChangeStatusMerged)
}

// IsMerged returns true iff the issue corresponding to the ChangeInfo is
// merged.
func (ci *ChangeInfo) IsMerged() bool {
	return ci.Status == ChangeStatusMerged
}

// GetAbandonReason returns the reason entered by the user that abandoned the change.
func (ci *ChangeInfo) GetAbandonReason(ctx context.Context) string {
	if ci.Status != ChangeStatusAbandoned {
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

// LabelEntry describes a label set on a Change in Gerrit.
type LabelEntry struct {
	All          []*LabelDetail
	Values       map[string]string
	DefaultValue int
}

// LabelDetail provides details about a label set on a Change in Gerrit.
type LabelDetail struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Date      string `json:"date"`
	Value     int    `json:"value"`
	AccountID int    `json:"_account_id"`
}

// FileInfoStatus is the type of 'Status' in FileInfo.
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#file-info
type FileInfoStatus string

const (
	FileAdded     FileInfoStatus = "A"
	FileCopied    FileInfoStatus = "C"
	FileDeleted   FileInfoStatus = "D"
	FileModified  FileInfoStatus = ""
	FileRenamed   FileInfoStatus = "R"
	FileRewritten FileInfoStatus = "W"
)

// AllFileInfoStatus is all valid values of type FileInfoStatus.
var AllFileInfoStatus = []FileInfoStatus{
	FileAdded,
	FileCopied,
	FileDeleted,
	FileModified,
	FileRenamed,
	FileRewritten,
}

// FileInfo provides information about changes to a File in Gerrit.
type FileInfo struct {
	Status        FileInfoStatus `json:"status"`
	Binary        bool           `json:"binary"`
	OldPath       string         `json:"old_path"`
	LinesInserted int            `json:"lines_inserted"`
	LinesDeleted  int            `json:"lines_deleted"`
	SizeDelta     int            `json:"size_delta"`
	Size          int            `json:"size"`
}

// Revision is the information associated with a patchset in Gerrit.
type Revision struct {
	ID            string    `json:"-"`
	Number        int64     `json:"_number"`
	CreatedString string    `json:"created"`
	Created       time.Time `json:"-"`
	Kind          string    `json:"kind"`
	Ref           string    `json:"ref"`
}

// GerritInterface describes interactions with a Gerrit host.
type GerritInterface interface {
	Abandon(context.Context, *ChangeInfo, string) error
	AddComment(context.Context, *ChangeInfo, string) error
	AddCC(context.Context, *ChangeInfo, []string) error
	Approve(context.Context, *ChangeInfo, string) error
	Config() *Config
	CreateChange(context.Context, string, string, string, string) (*ChangeInfo, error)
	DeleteChangeEdit(context.Context, *ChangeInfo) error
	DeleteFile(context.Context, *ChangeInfo, string) error
	DeleteVote(context.Context, int64, string, int, NotifyOption, bool) error
	Disapprove(context.Context, *ChangeInfo, string) error
	DownloadCommitMsgHook(ctx context.Context, dest string) error
	EditFile(context.Context, *ChangeInfo, string, string) error
	ExtractIssueFromCommit(string) (int64, error)
	Files(ctx context.Context, issue int64, patch string) (map[string]*FileInfo, error)
	GetChange(ctx context.Context, id string) (*ChangeInfo, error)
	GetCommit(ctx context.Context, issue int64, revision string) (*CommitInfo, error)
	GetContent(context.Context, int64, string, string) (string, error)
	GetFileNames(ctx context.Context, issue int64, patch string) ([]string, error)
	GetFilesToContent(ctx context.Context, issue int64, revision string) (map[string]string, error)
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
	Rebase(context.Context, *ChangeInfo, string, bool) error
	RemoveFromCQ(context.Context, *ChangeInfo, string) error
	Search(context.Context, int, bool, ...*SearchTerm) ([]*ChangeInfo, error)
	SelfApprove(context.Context, *ChangeInfo, string) error
	SendToCQ(context.Context, *ChangeInfo, string) error
	SendToDryRun(context.Context, *ChangeInfo, string) error
	SetCommitMessage(context.Context, *ChangeInfo, string) error
	SetReadyForReview(context.Context, *ChangeInfo) error
	SetReview(context.Context, *ChangeInfo, string, map[string]int, []string, NotifyOption, NotifyDetails, string, int, []*AttentionSetInput) error
	SetTopic(context.Context, string, int64) error
	SetTraceIDPrefix(traceIdPrefix string)
	Submit(context.Context, *ChangeInfo) error
	SubmittedTogether(context.Context, *ChangeInfo) ([]*ChangeInfo, int, error)
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
	rl                *rate.Limiter
	traceIdPrefix     string
}

// NewGerrit returns a new Gerrit instance.
func NewGerrit(gerritUrl string, client *http.Client) (*Gerrit, error) {
	return NewGerritWithConfig(ConfigChromium, gerritUrl, client)
}

// NewGerritWithConfig returns a new Gerrit instance which uses the given
// Config.
func NewGerritWithConfig(cfg *Config, gerritUrl string, client *http.Client) (*Gerrit, error) {
	return NewGerritWithConfigAndRateLimits(cfg, gerritUrl, client, defaultMaxQPS, defaultMaxBurst)
}

// NewGerritWithConfigAndRateLimits returns a new Gerrit instance which uses the given
// Config and rate limit options.
func NewGerritWithConfigAndRateLimits(cfg *Config, gerritUrl string, client *http.Client, maxQPS float64, maxBurst int) (*Gerrit, error) {
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
		rl:                rate.NewLimiter(rate.Limit(maxQPS), maxBurst),
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
	parsed, _ := time.Parse(TimeFormat, t)
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

// AccountDetails provides details about an account in Gerrit.
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
	url := fmt.Sprintf(URLTmplChange, id)
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
func (ci *ChangeInfo) GetPatchsetIDs() []int64 {
	ret := make([]int64, len(ci.Patchsets))
	for idx, patchSet := range ci.Patchsets {
		ret[idx] = patchSet.Number
	}
	return ret
}

// GetPatch returns the formatted patch for one revision. Documentation is here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-patch
func (g *Gerrit) GetPatch(ctx context.Context, issue int64, revision string) (string, error) {
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return "", err
	}

	u := fmt.Sprintf("%s/changes/%d/revisions/%s/patch", g.apiUrl, issue, revision)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.doRequest(req)
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
	Author  *Person       `json:"author"`
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

// GetContent gets the content of a file from the specified revision.
// See: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-content
func (g *Gerrit) GetContent(ctx context.Context, issue int64, revision string, filePath string) (string, error) {
	// Encode the filePath to convert paths like /COMMIT_MSG into %2FCOMMIT_MSG.
	filePath = url.QueryEscape(filePath)
	u := fmt.Sprintf("%s/changes/%d/revisions/%s/files/%s/content", g.apiUrl, issue, revision, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %s: %s", u, err)
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
	return string(data), nil
}

// GetFilesToContent returns a map of files in the specified issue+revision to
// their content.
func (g *Gerrit) GetFilesToContent(ctx context.Context, issue int64, revision string) (map[string]string, error) {
	filesToContent := map[string]string{}
	files, err := g.GetFileNames(ctx, issue, revision)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		content, err := g.GetContent(ctx, issue, revision, f)
		if err != nil {
			// Deleted files are expected to return 404s. Actual http.StatusNotFound
			// message should be "404 Not Found", but httputils.GetWithContext wraps
			// 404s as errors that contain the text "status code 404". So check for
			// both strings to be safe.
			if strings.Contains(err.Error(), "404 Not Found") || strings.Contains(err.Error(), "status code 404") {
				content = ""
			} else {
				return nil, err
			}
		}
		filesToContent[f] = content
	}
	return filesToContent, err
}

type Reviewer struct {
	Reviewer string `json:"reviewer"`
}
type NotifyInfo struct {
	Accounts []string `json:"accounts"`
}

type NotifyDetails map[RecipientType]*NotifyInfo

type AttentionSetInput struct {
	User   string `json:"user"`
	Reason string `json:"reason"`
}

// SetReview calls the Set Review endpoint of the Gerrit API to add messages and/or set labels for
// the latest patchset.
// API documentation: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
// notifyDetails contains additional information about whom to notify about the update. See details in
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#review-input
// onBehalfOf is expected to be the accountId the review should be posted on
// behalf of. Set to 0 to not use this functionality.
// attentionSetInputs contains details for adding users to the attension set. See details in
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#attention-set-input
func (g *Gerrit) SetReview(ctx context.Context, issue *ChangeInfo, message string, labels map[string]int, reviewers []string, notify NotifyOption, notifyDetails NotifyDetails, tag string, onBehalfOf int, attentionSetInputs []*AttentionSetInput) error {
	postData := map[string]interface{}{
		"message": message,
		"labels":  labels,
	}
	if notify != NotifyDefault {
		postData["notify"] = notify
	}
	if notifyDetails != nil {
		postData["notify_details"] = notifyDetails
	}
	if tag != "" {
		postData["tag"] = tag
	}
	if onBehalfOf != 0 {
		postData["on_behalf_of"] = onBehalfOf
	}
	if len(attentionSetInputs) > 0 {
		postData["add_to_attention_set"] = attentionSetInputs
	}

	if len(reviewers) > 0 {
		revs := make([]*Reviewer, 0, len(reviewers))
		for _, r := range reviewers {
			revs = append(revs, &Reviewer{
				Reviewer: r,
			})
		}
		postData["reviewers"] = revs
	}
	latestPatchset := issue.Patchsets[len(issue.Patchsets)-1]
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/revisions/%s/review", FullChangeId(issue), latestPatchset.ID), postData)
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
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/revisions/%s/review", FullChangeId(issue), latestPatchset.ID), postData)
}

// AddComment adds a message to the issue.
func (g *Gerrit) AddComment(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{}, nil, "", nil, "", 0, nil)
}

// Utility methods for interacting with the COMMITQUEUE_LABEL.

// SendToDryRun sets the Commit Queue dry run labels on the Change.
func (g *Gerrit) SendToDryRun(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SetDryRunLabels, nil, "", nil, "", 0, nil)
}

// SendToCQ sets the Commit Queue labels on the Change.
func (g *Gerrit) SendToCQ(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SetCqLabels, nil, "", nil, "", 0, nil)
}

// RemoveFromCQ unsets the Commit Queue labels on the Change.
func (g *Gerrit) RemoveFromCQ(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.NoCqLabels, nil, "", nil, "", 0, nil)
}

// Utility methods for interacting with the CODEREVIEW_LABEL.

// Approve sets the Code Review label to indicate approval.
func (g *Gerrit) Approve(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{LabelCodeReview: LabelCodeReviewApprove}, nil, "", nil, "", 0, nil)
}

// NoScore unsets the Code Review label.
func (g *Gerrit) NoScore(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{LabelCodeReview: LabelCodeReviewNone}, nil, "", nil, "", 0, nil)
}

// Disapprove sets the Code Review label to indicate disapproval.
func (g *Gerrit) Disapprove(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, map[string]int{LabelCodeReview: LabelCodeReviewDisapprove}, nil, "", nil, "", 0, nil)
}

// SelfApprove sets the Code Review label to indicate self-approval.
func (g *Gerrit) SelfApprove(ctx context.Context, issue *ChangeInfo, message string) error {
	return g.SetReview(ctx, issue, message, g.cfg.SelfApproveLabels, nil, "", nil, "", 0, nil)
}

// Abandon abandons the issue with the given message.
func (g *Gerrit) Abandon(ctx context.Context, issue *ChangeInfo, message string) error {
	postData := map[string]interface{}{
		"message": message,
	}
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/abandon", FullChangeId(issue)), postData)
}

// get retrieves the given sub URL and populates 'rv' with the result.
// If notFoundError is not nil it will be returned if the requested item doesn't
// exist.
func (g *Gerrit) get(ctx context.Context, suburl string, rv interface{}, notFoundError error) error {
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	getURL := g.apiUrl + suburl
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.doRequest(req)
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiUrl+suburl, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.doRequest(req)
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, g.apiUrl+suburl, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.doRequest(req)
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, g.apiUrl+suburl, nil)
	if err != nil {
		return err
	}
	resp, err := g.doRequest(req)
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

func (r revisionSlice) Len() int { return len(r) }
func (r revisionSlice) Less(i, j int) bool {
	if !util.TimeIsZero(r[i].Created) && !util.TimeIsZero(r[j].Created) {
		return r[i].Created.Before(r[j].Created)
	}
	return r[i].Number < r[j].Number
}
func (r revisionSlice) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

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

// SearchCommit is a SearchTerm used for filtering by commit.
func SearchCommit(commit string) *SearchTerm {
	return &SearchTerm{
		Key:   "commit",
		Value: commit,
	}
}

// SearchStatus is a SearchTerm used for filtering by status.
func SearchStatus(status string) *SearchTerm {
	return &SearchTerm{
		Key:   "status",
		Value: status,
	}
}

// SearchProject is a SearchTerm used for filtering by project.
func SearchProject(project string) *SearchTerm {
	return &SearchTerm{
		Key:   "project",
		Value: project,
	}
}

// SearchBranch is a SearchTerm used for filtering by branch.
func SearchBranch(branch string) *SearchTerm {
	return &SearchTerm{
		Key:   "branch",
		Value: branch,
	}
}

// SearchTopic is a SearchTerm used for filtering by topic.
func SearchTopic(topic string) *SearchTerm {
	return &SearchTerm{
		Key:   "topic",
		Value: topic,
	}
}

// SearchLabel is a SearchTerm used for filtering by label.
func SearchLabel(label, value string) *SearchTerm {
	return &SearchTerm{
		Key:   "label",
		Value: fmt.Sprintf("%s=%s", label, value),
	}
}

// SearchCherrypickOf is a SearchTerm used for finding all cherrypicks of the specified change.
func SearchCherrypickOf(changeNum int) *SearchTerm {
	return &SearchTerm{
		Key:   "cherrypickof",
		Value: fmt.Sprintf("%d", changeNum),
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

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
	resp, err := g.doRequest(req)
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
		if dependency.Status != ChangeStatusAbandoned && dependency.Status != ChangeStatusMerged {
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
		queryLimit := util.MinInt(limit-len(issues), maxSearchResultLimit)
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

// GetTrybotResults retrieves the trybot results for the given change from
// BuildBucket.
func (g *Gerrit) GetTrybotResults(ctx context.Context, issueID int64, patchsetID int64) ([]*buildbucketpb.Build, error) {
	return g.BuildbucketClient.GetTrybotsForCL(ctx, issueID, patchsetID, g.baseUrl, nil)
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
	return g.post(ctx, fmt.Sprintf("/changes/%s/submit", FullChangeId(ci)), []byte("{}"))
}

// The SubmittedTogetherInfo entity contains information about submitted
// together changes.
type SubmittedTogetherInfo struct {
	Changes           []*ChangeInfo `json:"changes"`
	NonVisibleChanges int           `json:"non_visible_changes"`
}

// SubmittedTogether returns list of all changes which are submitted when
// Submit is called for this change, including the current change itself.
// If the user calling the API does not have access to some changes then
// non_visible_changes will be > 0.
func (g *Gerrit) SubmittedTogether(ctx context.Context, ci *ChangeInfo) ([]*ChangeInfo, int, error) {
	var submittedTogetherInfo *SubmittedTogetherInfo
	if err := g.get(ctx, fmt.Sprintf("/changes/%s/submitted_together?o=NON_VISIBLE_CHANGES", FullChangeId(ci)), &submittedTogetherInfo, nil); err != nil {
		return nil, -1, fmt.Errorf("Failed to retrieve submitted_together issues: %s", err)
	}
	return submittedTogetherInfo.Changes, submittedTogetherInfo.NonVisibleChanges, nil
}

// DownloadCommitMsgHook downloads the commit message hook to the specified
// location.
func (g *Gerrit) DownloadCommitMsgHook(ctx context.Context, dest string) error {
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	url := g.apiUrl + urlCommitMsgHook
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := g.doRequest(req)
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

// Rebase the given change onto the given optional base.  If not provided, the
// change is rebased onto the target branch.  If allowConflicts is true, the
// rebase succeeds even if there are conflicts, in which case the patch set will
// contain git conflict markers.
func (g *Gerrit) Rebase(ctx context.Context, ci *ChangeInfo, base string, allowConflicts bool) error {
	postData := map[string]interface{}{
		"base":            base,
		"allow_conflicts": allowConflicts,
	}
	return g.postJson(ctx, fmt.Sprintf("/changes/%s/rebase", FullChangeId(ci)), postData)
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

// Get retrieves an issue from the cache.
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return nil, err
	}

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiUrl+"/changes/", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.doRequest(req)
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
	// Respect the rate limit.
	if err := g.rl.Wait(ctx); err != nil {
		return err
	}

	u := g.apiUrl + fmt.Sprintf("/changes/%s/edit/%s", ci.Id, url.QueryEscape(filepath))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("Failed to create PUT request: %s", err)
	}
	resp, err := g.doRequest(req)
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

// DeleteVote deletes a single vote from a change. Documentation is here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#delete-vote
func (g *Gerrit) DeleteVote(ctx context.Context, changeNum int64, labelID string, accountID int, notify NotifyOption, ignoreAutomaticAttentionSetRules bool) error {
	msg := struct {
		Notify                           NotifyOption `json:"notify,omitempty"`
		IgnoreAutomaticAttentionSetRules bool         `json:"ignore_automatic_attention_set_rules,omitempty"`
	}{
		Notify:                           notify,
		IgnoreAutomaticAttentionSetRules: ignoreAutomaticAttentionSetRules,
	}
	u := fmt.Sprintf("/changes/%d/reviewers/%d/votes/%s/delete", changeNum, accountID, labelID)
	return g.postJson(ctx, u, msg)
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

// FullChangeId returns the most specific representation of the specified
// change. See
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id
func FullChangeId(ci *ChangeInfo) string {
	project := ci.Project
	// Encode the project to convert names like chromium/src into chromium%2Fsrc.
	project = url.QueryEscape(project)

	branch := ci.Branch
	// Do not include refs/heads/ when constructing the full change Id.
	branch = strings.TrimPrefix(branch, "refs/heads/")
	// Encode the branch to convert names like chrome/m90 into chrome%2Fm90.
	branch = url.QueryEscape(branch)
	return fmt.Sprintf("%s~%s~%s", project, branch, ci.ChangeId)
}

// SetTraceIDPrefix enables tracing for all requests, with the given prefix.
// The full trace ID consists of the prefix and the timestamp of the request.
// If an empty string is provided, tracing is disabled. It is recommended to use
// an issue number for the trace ID, eg. "issue/123"
func (g *Gerrit) SetTraceIDPrefix(traceIdPrefix string) {
	g.traceIdPrefix = traceIdPrefix
}

// doRequest executes the given http.Request. It is a thin wrapper around
// g.client.Do().
func (g *Gerrit) doRequest(req *http.Request) (*http.Response, error) {
	var traceID string
	if g.traceIdPrefix != "" {
		traceID = fmt.Sprintf("%s-%d", g.traceIdPrefix, time.Now().UnixNano())
		req.Header.Add(HeaderTracing, traceID)
	}
	resp, err := g.client.Do(req)
	if traceID != "" {
		err = skerr.Wrapf(err, "trace ID %q", traceID)
	}
	return resp, err
}
