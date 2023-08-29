// audits package checks for audit issues and automatically files bugs.
package audits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/issues"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	packageFileName     = "package.json"
	packageLockFileName = "package-lock.json"

	issueTitleTmpl = "npm audit found high severity issues in %s’s package.json file"
	issueBodyTmpl  = `npm audit found high severity issues in %s’s [package.json](%s) file.

This issue was automatically filed by the npm-audit-mirror framework (see [go/sk-npm-audit-mirror](http://go/sk-npm-audit-mirror) for more information).
`
)

// File new audit issues 1 hour after the last one was closed.
var fileAuditIssueAfterThreshold = time.Hour

// NpmProjectAudit implements the types.ProjectAudit interface.
type NpmProjectAudit struct {
	projectName         string
	repoURL             string
	gitBranch           string
	packageFilesDir     string
	workDir             string
	gitilesRepo         gitiles.GitilesRepo
	dbClient            types.NpmDB
	issueTrackerConfig  *config.IssueTrackerConfig
	issueTrackerService types.IIssueTrackerService
}

// NewNpmProjectAudit periodically downloads package.json/package-lock.json from gitiles
// and runs audit on it.
func NewNpmProjectAudit(ctx context.Context, projectName, repoURL, gitBranch, packageFilesDir, workDir, serviceAccountFilePath string, httpClient *http.Client, dbClient types.NpmDB, issueTrackerConfig *config.IssueTrackerConfig) (types.ProjectAudit, error) {
	gitilesRepo := gitiles.NewRepo(repoURL, httpClient)

	// Instantiate issueTrackerService only if we have a issueTrackerConfig.
	var issueTrackerService *issues.IssueTrackerService
	var err error
	if issueTrackerConfig != nil {
		issueTrackerService, err = issues.NewIssueTrackerService(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	return &NpmProjectAudit{
		projectName:         projectName,
		repoURL:             repoURL,
		gitBranch:           gitBranch,
		packageFilesDir:     packageFilesDir,
		workDir:             workDir,
		gitilesRepo:         gitilesRepo,
		dbClient:            dbClient,
		issueTrackerConfig:  issueTrackerConfig,
		issueTrackerService: issueTrackerService,
	}, nil
}

// StartAudit runs `npm audit` on the project's package.json/package-lock.json
// files. If there are any high severity issues reported then the following
// algorithm will be used:
// * Check in the DB to see if an audit issue has been filed.
// * If issue has not been filed:
//   - File a new issue and add it to the DB.
//
// * Else if issue has been filed:
//   - Check to see if the issue has been closed.
//   - If issue is closed:
//   - Check to see if the issue is closed more than fileAuditIssueAfterThreshold duration ago.
//   - If it is older then file a new issue and add it to the DB.
//   - Else do nothing.
//   - Else if issue is still open then do nothing.
//
// StartAudit implements the types.ProjectAudit interface.
func (a *NpmProjectAudit) StartAudit(ctx context.Context, pollInterval time.Duration) {
	liveness := metrics2.NewLiveness("npm_audit", map[string]string{
		"project": a.projectName,
	})

	go util.RepeatCtx(ctx, pollInterval, func(ctx context.Context) {
		a.oneAuditCycle(ctx, liveness)
	})
}

func (a *NpmProjectAudit) oneAuditCycle(ctx context.Context, liveness metrics2.Liveness) {
	sklog.Infof("Starting audit of %s", a.projectName)

	// Download package.json and package-lock.json
	for _, packageFile := range []string{packageFileName, packageLockFileName} {
		packageFilePath := path.Join(a.packageFilesDir, packageFile)
		dest := path.Join(a.workDir, packageFile)
		err := a.gitilesRepo.DownloadFileAtRef(ctx, packageFilePath, a.gitBranch, dest)
		if err != nil {
			sklog.Errorf("Could not download %s from %s/%s/%s: %s", packageFile, a.repoURL, a.gitBranch, a.packageFilesDir, err)
			return // return so that the liveness is not updated
		}
	}

	auditCmd := executil.CommandContext(ctx, "npm", "audit", "--json", "--audit-level=high", "--omit=dev")
	auditCmd.Dir = a.workDir
	sklog.Info(auditCmd.String())
	b, err := auditCmd.Output()
	if err != nil {
		// Ignore error here because "npm audit" exits with a non-0 code if any vulnerability is found.
	}

	var ao types.NpmAuditOutput
	if err := json.Unmarshal(b, &ao); err != nil {
		sklog.Errorf("Could not parse npm audit output for %s: %s", a.projectName, err)
		return // return so that the liveness is not updated
	}

	// Keep track of how many audit issues have high severity.
	highSeverityCounter := 0
	if issues, ok := ao.Metadata.Vulnerabilities["high"]; ok {
		highSeverityCounter = highSeverityCounter + issues
	}
	if issues, ok := ao.Metadata.Vulnerabilities["critical"]; ok {
		highSeverityCounter = highSeverityCounter + issues
	}

	sklog.Infof("Done with audit of %s. Found %d high severity issues.", a.projectName, highSeverityCounter)

	if highSeverityCounter > 0 && a.issueTrackerConfig != nil {
		// Check in the DB to see if an audit issue has been filed.
		ad, err := a.dbClient.GetFromDB(ctx, a.projectName)
		if err != nil {
			sklog.Errorf("Could not get audit data for %s from the DB: %s", a.projectName, err)
			return // return so that the liveness is not updated
		}

		if ad == nil {
			// Issue has not been filed yet. File one and add it to the DB.
			sklog.Infof("There is no audit data for project %s in firestore. File a new issue.", a.projectName)
			if err := a.fileAndPersistAuditIssue(ctx); err != nil {
				sklog.Errorf("Could not file and persist audit issue for %s: %s", a.projectName, err)
				return // return so that the liveness is not updated
			}
		} else {
			sklog.Infof("Found audit data in firestore for project %s: %+v", a.projectName, ad)
			// Query issue tracker to see if the issue is closed.
			existingIssue, err := a.issueTrackerService.GetIssue(ad.IssueId)
			if err != nil {
				sklog.Errorf("Could not query issue tracker for %d: %s", ad.IssueId, err)
				return // return so that the liveness is not updated
			}
			if existingIssue.ResolvedTime != "" {
				// Check to see when the issue was closed.
				closedTime, err := time.Parse(time.RFC3339, existingIssue.ResolvedTime)
				if err != nil {
					sklog.Errorf("Could not parse resolved time %s", existingIssue.ResolvedTime)
					return // return so that liveness is not updated
				}
				closedDuration := time.Now().UTC().Sub(closedTime)
				sklog.Infof("Previously filed audit issue %d was closed at %s which is %s ago.", existingIssue.IssueId, closedTime, closedDuration)

				if closedDuration > fileAuditIssueAfterThreshold {
					sklog.Infof("Filing new audit issue since audit issue %d was closed longer than the threshold %s.", existingIssue.IssueId, fileAuditIssueAfterThreshold)
					if err := a.fileAndPersistAuditIssue(ctx); err != nil {
						sklog.Errorf("Could not file and persist audit issue for %s: %s", a.projectName, err)
						return // return so that the liveness is not updated
					}
				}
			} else {
				sklog.Infof("Previously filed audit issue %d is still open. Do nothing.", existingIssue.IssueId)
			}
		}
	}

	liveness.Reset()
}

// fileAndPersistAuditIssue calls the issue tracker service to file a new issue
// and then adds that issue to the DB.
func (a *NpmProjectAudit) fileAndPersistAuditIssue(ctx context.Context) error {
	itc := a.issueTrackerConfig

	// Create a new issue.
	title := fmt.Sprintf(issueTitleTmpl, a.projectName)
	body := fmt.Sprintf(issueBodyTmpl, a.projectName, path.Join(a.gitilesRepo.URL(), "+show", "refs/heads/"+a.gitBranch, a.packageFilesDir, packageFileName))

	newIssue, err := a.issueTrackerService.MakeIssue(itc.Owner, title, body)
	if err != nil {
		return skerr.Wrapf(err, "Could not create an audit issue for project %s", a.projectName)
	}
	// Add new issue data to firestore.
	createdTime, err := time.Parse(time.RFC3339, newIssue.CreatedTime)
	if err != nil {
		return skerr.Wrapf(err, "could not parse %s", newIssue.CreatedTime)
	}
	if err := a.dbClient.PutInDB(ctx, a.projectName, newIssue.IssueId, createdTime.UTC()); err != nil {
		return skerr.Wrapf(err, "Could not put issue data into firestore for project %s", a.projectName)
	}
	sklog.Infof("Filed new audit issue %d for project %s and put it in DB.", newIssue.IssueId, a.projectName)

	return nil
}
