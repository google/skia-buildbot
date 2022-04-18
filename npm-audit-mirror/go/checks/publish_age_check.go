package checks

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	publishTimeCuttoff = time.Hour * 24 * 7 // 1 week.
)

func NewPublishAgeCheck() types.Check {
	return &PublishAgeCheck{}
}

// PublishAgeCheck implements the types.Checks interface.
type PublishAgeCheck struct{}

// Name implements the types.Checks interface.
func (lc *PublishAgeCheck) Name() string {
	return "PublishAgeCheck"
}

// PerformCheck implements the types.Checks interface.
func (lc *PublishAgeCheck) PerformCheck(packageName, packageVersion string, npmPackage *types.NpmPackage) (bool, string, error) {

	packageTime := npmPackage.Time[packageVersion]
	t, err := time.Parse(time.RFC3339, packageTime)
	if err != nil {
		return false, "", skerr.Wrapf(err, "Failed to RFC3339 parse %s for package %s with version %s", packageTime, packageName, packageVersion)
	}

	diff := time.Now().Sub(t)
	if diff < publishTimeCuttoff {
		// We cannot allow this package to be downloaded.
		rejectionReason := fmt.Sprintf("Package %s with version %s was created %s time ago. This is less than 1 week and so failed the audit.", packageName, packageVersion, diff.Round(time.Hour))
		sklog.Info(rejectionReason)
		return false, rejectionReason, nil
	}

	return true, "", nil
}
