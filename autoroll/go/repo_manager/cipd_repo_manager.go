package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
)

const (
	cipdReWhitespace = `(\s*)`
	cipdReVersion    = `(\S+)`
	cipdRePackage    = `%s`
	cipdPackageTmpl  = `{%s'package'%s:%s'%s'%s,%s'version'%s:%s'%s'%s,%s}`
	cipdReTmpl       = `(?ms:%s)`

	cipdPackageUrlTmpl = "%s/p/%s/+/%s"

	cipdCommitMsgTmpl = `Roll %s from %s to %s

` + COMMIT_MSG_FOOTER_TMPL
)

var (
	cipdRegex = fmt.Sprintf(fmt.Sprintf(cipdReTmpl, cipdPackageTmpl), cipdReWhitespace, cipdReWhitespace, cipdReWhitespace, cipdRePackage, cipdReWhitespace, cipdReWhitespace, cipdReWhitespace, cipdReWhitespace, cipdReVersion, cipdReWhitespace, cipdReWhitespace)
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewCIPDRepoManager func(context.Context, *CIPDRepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newCIPDRepoManager
)

// CIPDRepoManagerConfig provides configuration for CIPDRepoManager.
type CIPDRepoManagerConfig struct {
	// Name of the CIPD package.
	Package string `json:"package"`

	// Ref which indicates the desired version of the CIPD package.
	PackageRef string `json:"packageRef"`

	// Name of the branch in the parent repo to roll into.
	ParentBranch string `json:"parentBranch"`

	// URL of the parent repo to roll into.
	ParentRepo string `json:"parentRepo"`
}

// Validate the config.
func (c *CIPDRepoManagerConfig) Validate() error {
	if c.Package == "" {
		return errors.New("Package is required.")
	}
	if c.ParentBranch == "" {
		return errors.New("ParentBranch is required.")
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	return nil
}

// cipdRepoManager is a struct used for rolling a CIPD package using DEPS.
type cipdRepoManager struct {
	*noCheckoutRepoManager
	cipdClient *cipd.Client
	pkg        string
	pkgRef     string
	pkgRegex   *regexp.Regexp
}

// newCIPDRepoManager returns a RepoManager instance which rolls a CIPD package
// using DEPS.
func newCIPDRepoManager(ctx context.Context, c *CIPDRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}
	pkgRegex, err := regexp.Compile(fmt.Sprintf(cipdRegex, c.Package))
	if err != nil {
		return nil, err
	}
	cipdClient, err := cipd.NewClient(client, path.Join(workdir, "cipd"))
	if err != nil {
		return nil, err
	}
	rv := &cipdRepoManager{
		cipdClient: cipdClient,
		pkg:        c.Package,
		pkgRef:     c.PackageRef,
		pkgRegex:   pkgRegex,
	}
	ncrmConfig := NoCheckoutRepoManagerConfig{
		CommonRepoManagerConfig: CommonRepoManagerConfig{
			ChildBranch:  "N/A",
			ChildPath:    "N/A",
			ParentBranch: c.ParentBranch,
		},
		ParentRepo: c.ParentRepo,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, ncrmConfig, workdir, g, serverURL, gitcookiesPath, client, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *cipdRepoManager) createRoll(ctx context.Context, from, to, serverURL, cqExtraTrybots string, emails []string) (string, map[string]string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Build commit message.
	commitMsg := fmt.Sprintf(cipdCommitMsgTmpl, rm.pkg, from, to, rm.serverURL)
	if cqExtraTrybots != "" {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, cqExtraTrybots)
	}
	tbr := "\nTBR="
	if len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
	}
	commitMsg += tbr

	// Update DEPS.
	var buf bytes.Buffer
	if err := rm.parentRepo.ReadFileAtRef("DEPS", rm.baseCommit, &buf); err != nil {
		return "", nil, err
	}
	newDeps := rm.pkgRegex.ReplaceAllString(buf.String(), fmt.Sprintf(cipdPackageTmpl, "$1", "$2", "$3", rm.pkg, "$4", "$5", "$6", "$7", to, "$9", "$10"))
	changes := map[string]string{
		"DEPS": newDeps,
	}

	return commitMsg, changes, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *cipdRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, []*revision.Revision, error) {
	// Find the current version in DEPS.
	var buf bytes.Buffer
	if err := rm.parentRepo.ReadFileAtRef("DEPS", baseCommit, &buf); err != nil {
		return "", "", nil, err
	}
	m := rm.pkgRegex.FindStringSubmatch(buf.String())
	if len(m) != 11 {
		return "", "", nil, errors.New("Could not find matching CIPD package in DEPS!")
	}
	lastRollRev := m[8]
	if lastRollRev == "" {
		return "", "", nil, errors.New("Couldn't find last roll rev!")
	}

	// Use CIPD to find the not-yet-rolled versions of the package. Note
	// that this just finds all versions of the package between the last
	// rolled version and the version currently pointed to by rm.pkgRef; we
	// can't know whether the ref we're tracking was ever actually applied
	// to any of the package instances in between.
	head, err := rm.cipdClient.ResolveVersion(ctx, rm.pkg, rm.pkgRef)
	if err != nil {
		return "", "", nil, err
	}
	iter, err := rm.cipdClient.ListInstances(ctx, rm.pkg)
	if err != nil {
		return "", "", nil, err
	}
	notRolledRevs := []*revision.Revision{}
	foundHead := false
	for {
		instances, err := iter.Next(ctx, 100)
		if err != nil {
			return "", "", nil, err
		}
		if len(instances) == 0 {
			break
		}
		for _, instance := range instances {
			id := instance.Pin.InstanceID
			if id == head.InstanceID {
				foundHead = true
			}
			if id == lastRollRev {
				break
			}
			if foundHead {
				notRolledRevs = append(notRolledRevs, &revision.Revision{
					Id:          id,
					Display:     instance.Pin.String(),
					Description: instance.Pin.String(),
					Timestamp:   time.Time(instance.RegisteredTs),
					URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.SERVICE_URL, rm.pkg, id),
				})
			}
		}
	}

	// Get the next roll revision.
	nextRollRev, err := rm.getNextRollRev(ctx, notRolledRevs, lastRollRev)
	if err != nil {
		return "", "", nil, err
	}

	return lastRollRev, nextRollRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *cipdRepoManager) FullChildHash(ctx context.Context, ver string) (string, error) {
	// We don't shorten versions for this type of roller.
	return ver, nil
}

// See documentation for RepoManager interface.
func (rm *cipdRepoManager) RolledPast(ctx context.Context, ver string) (bool, error) {
	requested, err := rm.cipdClient.Describe(ctx, rm.pkg, ver)
	if err != nil {
		return false, err
	}
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	lastRolled, err := rm.cipdClient.Describe(ctx, rm.pkg, rm.lastRollRev)
	if err != nil {
		return false, err
	}
	return !time.Time(requested.RegisteredTs).After(time.Time(lastRolled.RegisteredTs)), nil
}

// See documentation for RepoManager interface.
func (r *cipdRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, "", DEFAULT_REMOTE, "", []string{}, nil, nil)
}

// See documentation for RepoManager interface.
func (rm *cipdRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (rm *cipdRepoManager) ValidStrategies() []string {
	return []string{strategy.ROLL_STRATEGY_BATCH, strategy.ROLL_STRATEGY_SINGLE}
}
