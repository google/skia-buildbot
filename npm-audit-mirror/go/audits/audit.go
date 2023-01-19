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
	"go.skia.org/infra/go/monorail/v3"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	packageFileName     = "package.json"
	packageLockFileName = "package-lock.json"

	highVulnerabilityKey = "high"

	issueSummaryTmpl     = "npm audit found high severity issues in %s’s package.json file"
	issueDescriptionTmpl = "npm audit found high severity issues in %s’s package.json file here: %s\\n\\nThis issue was automatically filed by the npm-audit-mirror framework (see go/sk-npm-audit-mirror for more information)."
	defaultIssueType     = "Task"
	defaultIssueStatus   = "Assigned"
	defaultIssuePriority = "High"

	defaultCCUser = "rmistry@google.com"
)

// File new audit issues 1 hour after the last one was closed.
var fileAuditIssueAfterThreshold = time.Hour

// NpmProjectAudit implements the types.ProjectAudit interface.
type NpmProjectAudit struct {
	projectName     string
	repoURL         string
	gitBranch       string
	packageFilesDir string
	workDir         string
	gitilesRepo     gitiles.GitilesRepo
	dbClient        types.NpmDB
	monorailConfig  *config.MonorailConfig
	monorailService monorail.IMonorailService
}

// NewNpmProjectAudit periodically downloads package.json/package-lock.json from gitiles
// and runs audit on it.
func NewNpmProjectAudit(ctx context.Context, projectName, repoURL, gitBranch, packageFilesDir, workDir, serviceAccountFilePath string, httpClient *http.Client, dbClient types.NpmDB, monorailConfig *config.MonorailConfig) (types.ProjectAudit, error) {
	gitilesRepo := gitiles.NewRepo(repoURL, httpClient)

	// Instantiate monorailService only if we have a monorailConfig.
	var monorailService *monorail.MonorailService
	var err error
	if monorailConfig != nil {
		monorailService, err = monorail.New(ctx, serviceAccountFilePath)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	return &NpmProjectAudit{
		projectName:     projectName,
		repoURL:         repoURL,
		gitBranch:       gitBranch,
		packageFilesDir: packageFilesDir,
		workDir:         workDir,
		gitilesRepo:     gitilesRepo,
		dbClient:        dbClient,
		monorailConfig:  monorailConfig,
		monorailService: monorailService,
	}, nil
}

// StartAudit runs `npm audit` on the project's package.json/package-lock.json
// files. If there are any high severity issues reported then the following
// algorithm will be used:
// * Check in the DB to see if an audit issue has been filed.
// * If issue has not been filed:
//   * File a new issue and add it to the DB.
// * Else if issue has been filed:
//   * Check to see if the issue has been closed.
//   * If issue is closed:
//     * Check to see if the issue is closed more than fileAuditIssueAfterThreshold duration ago.
//     * If it is older then file a new issue and add it to the DB.
//     * Else do nothing.
//   * Else if issue is still open then do nothing.
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

	if highSeverityCounter > 0 && a.monorailConfig != nil {
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
			// Query monorail to see if the issue is closed.
			existingIssue, err := a.monorailService.GetIssue(ad.IssueName)
			if err != nil {
				sklog.Errorf("Could not query monorail for %s: %s", ad.IssueName, err)
				return // return so that the liveness is not updated
			}
			if !existingIssue.ClosedTime.IsZero() {
				// Check to see when the issue was closed.
				closedDuration := time.Now().UTC().Sub(existingIssue.ClosedTime)
				sklog.Infof("Previously filed audit issue %s was closed at %s which is %s ago.", existingIssue.Name, existingIssue.ClosedTime, closedDuration)

				if closedDuration > fileAuditIssueAfterThreshold {
					sklog.Infof("Filing new audit issue since audit issue %s was closed longer than the threshold %s.", existingIssue.Name, fileAuditIssueAfterThreshold)
					if err := a.fileAndPersistAuditIssue(ctx); err != nil {
						sklog.Errorf("Could not file and persist audit issue for %s: %s", a.projectName, err)
						return // return so that the liveness is not updated
					}
				}
			} else {
				sklog.Infof("Previously filed audit issue %s is still open. Do nothing.", existingIssue.Name)
			}
		}
	}

	liveness.Reset()
}

// fileAndPersistAuditIssue calls the monorail service to file a new monorail
// issue and then adds that issue to the DB.
func (a *NpmProjectAudit) fileAndPersistAuditIssue(ctx context.Context) error {
	mc := a.monorailConfig

	// Create a new monorail issue.
	summary := fmt.Sprintf(issueSummaryTmpl, a.projectName)
	description := fmt.Sprintf(issueDescriptionTmpl, a.projectName, path.Join(a.gitilesRepo.URL(), "+show", "refs/heads/"+a.gitBranch, a.packageFilesDir, packageFileName))

	// Always file issues with the Restrict-View-Google label.
	labels := []string{monorail.RestrictViewGoogleLabelName}
	labels = append(labels, mc.Labels...)

	newIssue, err := a.monorailService.MakeIssue(mc.InstanceName, mc.Owner, summary, description, defaultIssueStatus, defaultIssuePriority, defaultIssueType, labels, mc.ComponentDefIDs, []string{defaultCCUser})
	if err != nil {
		return skerr.Wrapf(err, "Could not create an audit issue for project %s", a.projectName)
	}
	// Add new issue data to firestore.
	if err := a.dbClient.PutInDB(ctx, a.projectName, newIssue.Name, newIssue.CreatedTime.UTC()); err != nil {
		return skerr.Wrapf(err, "Could not put issue data into firestore for project %s", a.projectName)
	}
	sklog.Infof("Filed new audit issue %s and put it in DB.", newIssue.Name)

	return nil
}
