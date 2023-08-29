package examiner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/issues"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	// Packages that have been created less than a week ago will be flagged.
	packageCreatedTimeCutoff = time.Hour * 24 * 7

	// File new examiner issues once a week.
	fileExaminerIssueAfterThreshold = time.Hour * 24 * 7

	issueTitleTmpl = "Package %s in %s’s package-lock.json was recently republished"
	issueBodyTmpl  = `Package %s in %s’s package-lock.json was recently republished.
This could indicate that the package was deleted and maliciously republished and may pose a security risk (see b/40044481 for context).
Please take a look at the package and remove from your dependencies if necessary.

This issue was automatically filed by the npm-audit-mirror framework (see [go/sk-npm-audit-mirror](http://go/sk-npm-audit-mirror) for more information).
`
)

// DownloadedPackagesExaminer implements types.DownloadedPackagesExaminer
type DownloadedPackagesExaminer struct {
	trustedScopes       []string
	httpClient          *http.Client
	dbClient            types.NpmDB
	projectMirror       types.ProjectMirror
	issueTrackerConfig  *config.IssueTrackerConfig
	issueTrackerService types.IIssueTrackerService
}

// NewDownloadedPackagesExaminer returns an instance of DownloadedPackagesExaminer.
func NewDownloadedPackagesExaminer(ctx context.Context, trustedScopes []string, httpClient *http.Client, dbClient types.NpmDB, projectMirror types.ProjectMirror, issueTrackerConfig *config.IssueTrackerConfig, serviceAccountFilePath string) (types.DownloadedPackagesExaminer, error) {
	// Instantiate issueTrackerService only if we have a issueTrackerConfig.
	var issueTrackerService *issues.IssueTrackerService
	var err error
	if issueTrackerConfig != nil {
		issueTrackerService, err = issues.NewIssueTrackerService(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	return &DownloadedPackagesExaminer{
		trustedScopes:       trustedScopes,
		httpClient:          httpClient,
		dbClient:            dbClient,
		projectMirror:       projectMirror,
		issueTrackerConfig:  issueTrackerConfig,
		issueTrackerService: issueTrackerService,
	}, nil
}

// StartExamination will examine all the downloaded packages of the mirror.
// For each downloaded package:
// * Check to see if it has a trusted scope. If it does then continue with next package.
// * Check the package against the global NPM registry to see if has been created less than a week ago.
// * If above check is true then:
//   - Check in the DB to see if an examiner issue has been filed for this project+package.
//   - If issue has not been filed:
//   - File a new issue and add it to the DB.
//   - Else if issue has been filed:
//   - Check to see if the issue has been closed.
//   - If issue is closed:
//   - Check to see if the issue is closed more than fileExaminerIssueAfterThreshold duration ago.
//   - If it is older then file a new issue and add it to the DB.
//   - Else do nothing.
//   - Else if issue is still open then do nothing.
func (dpe *DownloadedPackagesExaminer) StartExamination(ctx context.Context, pollInterval time.Duration) {
	liveness := metrics2.NewLiveness("npm_examiner", map[string]string{
		"project": dpe.projectMirror.GetProjectName(),
	})

	go util.RepeatCtx(ctx, pollInterval, func(ctx context.Context) {
		dpe.oneExaminationCycle(ctx, liveness)
	})
}

func (dpe *DownloadedPackagesExaminer) oneExaminationCycle(ctx context.Context, liveness metrics2.Liveness) {
	projectName := dpe.projectMirror.GetProjectName()
	sklog.Infof("Starting examination of %s", projectName)

	packages, err := dpe.projectMirror.GetDownloadedPackageNames()
	if err != nil {
		sklog.Errorf("Could not get downloaded packages details for %s: %s", projectName, err)
		return // return so that liveness is not updated.
	}

	for _, p := range packages {
		// Check for trusted scopes.
		hasTrustedScope := false
		for _, trustedScope := range dpe.trustedScopes {
			if strings.HasPrefix(p, trustedScope) {
				sklog.Infof("The package %s has the trusted scope %s. Skipping downloaded package examination.", p, trustedScope)
				hasTrustedScope = true
				break
			}
		}
		if hasTrustedScope {
			continue
		}

		// Examine the package by hitting the global NPM repository.
		packageDetails, err := dpe.getPackageDetailsFromGlobalRepo(p)
		if err != nil {
			sklog.Errorf("Could not get package details of %s: %s", p, err)
			return // return so that liveness is not updated.
		}
		// See if the package was created < 7 days ago.
		createdTime := packageDetails.Time["created"]
		t, err := time.Parse(time.RFC3339, createdTime)
		if err != nil {
			sklog.Errorf("Failed to RFC3339 parse %s for package %s", createdTime, p)
			return // return so that liveness is not updated.
		}

		diff := time.Now().Sub(t)
		if diff < packageCreatedTimeCutoff {
			message := fmt.Sprintf("In project %s package %s was created %s time ago. This is less than 1 week. This could be a malicious deletion+republish.", projectName, p, diff)
			if dpe.issueTrackerConfig != nil {
				if err := dpe.runBugFilingLogic(ctx, projectName, p); err != nil {
					sklog.Errorf("Could not run the bug filing logic for project %s and package %s: %s", projectName, p, err)
					return // return so that the liveness is not updated
				}
			} else {
				// If the issue tracker config was not provided this is still
				// important enough to log as an error message.
				sklog.Error(message)
			}
		}
	}

	liveness.Reset()
	sklog.Infof("Done with one examination cycle of the downloaded packages of %s", projectName)
}

// runBugFilingLogic runs this algorithm:
// * Check in the DB to see if an examiner issue has been filed for this project+package.
// * If issue has not been filed:
//   - File a new issue and add it to the DB.
//
// * Else if issue has been filed:
//   - Check to see if the issue has been closed.
//   - If issue is closed:
//   - Check to see if the issue is closed more than fileExaminerIssueAfterThreshold duration ago.
//   - If it is older then file a new issue and add it to the DB.
//   - Else do nothing.
//
// * Else if issue is still open then do nothing.
func (dpe *DownloadedPackagesExaminer) runBugFilingLogic(ctx context.Context, projectName, packageName string) error {
	// Construct key to use in the DB. Package names can contain "/" so sanitize the name.
	sanitizedPackageName := strings.Replace(packageName, "/", "_", -1)
	projectPackageKey := fmt.Sprintf("%s_%s", projectName, sanitizedPackageName)

	// Check in the DB to see if an examiner issue has been filed.
	dbData, err := dpe.dbClient.GetFromDB(ctx, projectPackageKey)
	if err != nil {
		return fmt.Errorf("Could not get examiner data for %s from the DB: %s", projectPackageKey, err)
	}

	if dbData == nil {
		// Issue has not been filed yet. File one and add it to the DB.
		sklog.Infof("There is no examiner data for project+package %s in firestore. File a new issue.", projectPackageKey)
		if err := dpe.fileAndPersistExaminerIssue(ctx, packageName, projectPackageKey); err != nil {
			return fmt.Errorf("Could not file and persist examiner issue for %s: %s", projectPackageKey, err)
		}
	} else {
		sklog.Infof("Found examiner data in firestore for project+package %s: %+v", projectPackageKey, dbData)
		// Query issue tracker to see if the issue is closed.
		existingIssue, err := dpe.issueTrackerService.GetIssue(dbData.IssueId)
		if err != nil {
			return fmt.Errorf("Could not query issue tracker for %d: %s", dbData.IssueId, err)
		}
		if existingIssue.ResolvedTime != "" {
			// Check to see when the issue was closed.
			closedTime, err := time.Parse(time.RFC3339, existingIssue.ResolvedTime)
			if err != nil {
				return fmt.Errorf("Could not parse resolved time %s", existingIssue.ResolvedTime)
			}
			closedDuration := time.Now().UTC().Sub(closedTime)
			sklog.Infof("Previously filed examiner issue %d was closed at %s which is %s ago.", existingIssue.IssueId, closedTime, closedDuration)

			if closedDuration > fileExaminerIssueAfterThreshold {
				sklog.Infof("Filing new examiner issue since last issue %d was closed longer than the threshold %s.", existingIssue.IssueId, fileExaminerIssueAfterThreshold)
				if err := dpe.fileAndPersistExaminerIssue(ctx, packageName, projectPackageKey); err != nil {
					return fmt.Errorf("Could not file and persist examiner issue for %s: %s", projectPackageKey, err)
				}
			}
		} else {
			sklog.Infof("Previously filed examiner issue %d is still open. Do nothing.", existingIssue.IssueId)
		}
	}
	return nil
}

func (dpe *DownloadedPackagesExaminer) getPackageDetailsFromGlobalRepo(packageName string) (*types.NpmPackage, error) {
	viewNpmURL := fmt.Sprintf("https://registry.npmjs.org/%s", packageName)
	r, err := dpe.httpClient.Get(viewNpmURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error getting response from %s", viewNpmURL)
	}
	defer r.Body.Close()

	var npmPackage types.NpmPackage
	if err := json.NewDecoder(r.Body).Decode(&npmPackage); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode response from %s", viewNpmURL)
	}
	return &npmPackage, nil
}

// fileAndPersistExaminerIssue calls the issue tracker service to file a new
// issue and then adds that issue to the DB.
func (dpe *DownloadedPackagesExaminer) fileAndPersistExaminerIssue(ctx context.Context, packageName, projectPackageKey string) error {
	itc := dpe.issueTrackerConfig
	projectName := dpe.projectMirror.GetProjectName()

	// Create a new issue.
	title := fmt.Sprintf(issueTitleTmpl, packageName, projectName)
	body := fmt.Sprintf(issueBodyTmpl, packageName, projectName)

	newIssue, err := dpe.issueTrackerService.MakeIssue(itc.Owner, title, body)
	if err != nil {
		return skerr.Wrapf(err, "Could not create an issue for project %s and package %s", projectName, packageName)
	}
	// Add new issue data to firestore.
	createdTime, err := time.Parse(time.RFC3339, newIssue.CreatedTime)
	if err != nil {
		return skerr.Wrapf(err, "could not parse %s", newIssue.CreatedTime)
	}
	if err := dpe.dbClient.PutInDB(ctx, projectPackageKey, newIssue.IssueId, createdTime.UTC()); err != nil {
		return skerr.Wrapf(err, "Could not put issue data into firestore for project %s and package %s", projectName, packageName)
	}
	sklog.Infof("Filed new issue from downloaded_packages_examiner %d for project %s and package %s and put it in DB.", newIssue.IssueId, projectName, packageName)

	return nil
}
