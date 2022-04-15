package types

import (
	"context"
	"time"
)

// NpmDB is the interface implemented by all DB clients.
type NpmDB interface {
	// GetFromDB returns an NpmAuditData document snapshot from Firestore. If the
	// document is not found then (nil, nil) is returned.
	GetFromDB(ctx context.Context, key string) (*NpmAuditData, error)

	// PutInDB puts NpmAuditData into the DB. If the specified key already exists
	// then it is updated.
	PutInDB(ctx context.Context, key, issueName string, created time.Time) error
}

// ChecksManager helps callers perform checks on a particular project.
type ChecksManager interface {
	// PerformChecks returns False when a package fails checks and also returns a
	// descriptive reason why. Returns True when package passes all checks.
	// If an error is returned then False and an empty string will also be
	// returned.
	PerformChecks(packageRequestURL string) (bool, string, error)
}

// Check is the interface implemented by all checks.
type Check interface {
	// Name of the check.
	Name() string

	// PerformCheck runs the check on the specified package.
	// If the check fails then the return bool will be False and the string will
	// contain a reason explaining the failure.
	// If the check passwes then the return bool will be True and the string
	// will be empty.
	// If error is non-nil then bool will be False and reason will be empty.
	PerformCheck(packageName, packageVersion string, npmPackage *NpmPackage) (bool, string, error)
}

// ProjectAudit is the interface implemented by all project audits.
type ProjectAudit interface {
	// StartAudit starts the auditing of the project in a goroutine.
	StartAudit(ctx context.Context, pollInterval time.Duration)
}

// ProjectMirror is the interface implemented by all project mirrors.
type ProjectMirror interface {
	// Name of the project this mirror was created for.
	GetProjectName() string

	// StartMirror starts the project's mirror in a goroutine.
	StartMirror(ctx context.Context, port int) error

	// AddToDownloadedPackageTarballs adds the provided package to the
	// in-memory map of installed packages. This is done to avoid expensive
	// calls by calling the filesystem.
	AddToDownloadedPackageTarballs(packageTarballName string)

	// IsPackageTarballDownloaded checks to see whether the specified
	// tarball has already been downloaded by the mirror.
	IsPackageTarballDownloaded(packageTarballName string) bool
}

// PackageDetails is populated by parsing a packageRequestURL and used in
// checks_manager.
type PackageDetails struct {
	NameWithScope string
	ScopeName     string
	TarballName   string
	Version       string
}

// NpmPackage types to parse responses from the NPM global registry.
type NpmPackage struct {
	Time     map[string]string     `json:"time"`
	Versions map[string]NpmVersion `json:"versions"`
}
type NpmVersion struct {
	Dependencies map[string]string `json:"dependencies"`
	License      interface{}       `json:"license"`
}
type NpmPackageTime struct {
	Versions map[string]string
}

// Types used to parse output of the `npm audit` command.
type NpmAuditOutput struct {
	Advisories           map[string]Advisory `json:"advisories"`
	Metadata             NpmAuditMetadata    `json:"metadata"`
	Dependencies         string              `json:"dependencies"`
	DevDependencies      string              `json:"devDependencies"`
	OptionalDependencies string              `json:"optionalDependencies"`
	TotalDependencies    string              `json:"totalDependencies"`
}
type Advisory struct {
	Severity       string `json:"severity"`
	Recommendation string `json:"recommendation"`
	ModuleName     string `json:"module_name"`
}
type NpmAuditMetadata struct {
	Vulnerabilities map[string]int `json:"vulnerabilities"`
}

// NpmAuditData is the type that will be stored in the DB.
type NpmAuditData struct {
	// When the audit issue was created.
	Created time.Time `firestore:"created"`
	// The resource name of the Issue. Eg: "projects/skia/issues/13158".
	IssueName string `firestore:"issue_name"`
}
