package roller

import (
	"fmt"
	"net/url"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/sklog"
)

// GetSheriff retrieves the current sheriff list.
func GetSheriff(metricsName string, sheriffSources, backupSheriffs []string) ([]string, error) {
	tags := map[string]string{
		"roller": metricsName,
	}
	m := metrics2.GetInt64Metric("autoroll_get_sheriff_success", tags)
	allEmails := []string{}
	for _, s := range sheriffSources {
		emails, err := getSheriffHelper(s)
		if err != nil {
			sklog.Errorf("Failed to retrieve sheriff(s): %s", err)
			m.Update(0)
			if len(backupSheriffs) == 0 {
				return nil, fmt.Errorf("Failed to retrieve sheriffs and no backup sheriffs supplied!  %s", err)
			}
			return backupSheriffs, nil
		}
		allEmails = append(allEmails, emails...)
	}
	m.Update(1)
	return allEmails, nil
}

// Helper for loading the sheriff list.
func getSheriffHelper(sheriffConfig string) ([]string, error) {
	// If the passed-in sheriffConfig doesn't look like a URL, it's probably an
	// email address. Use it directly.
	if _, err := url.ParseRequestURI(sheriffConfig); err != nil {
		if strings.Count(sheriffConfig, "@") == 1 {
			return []string{sheriffConfig}, nil
		} else {
			return nil, fmt.Errorf("Sheriff must be an email address or a valid URL; %q doesn't look like either.", sheriffConfig)
		}
	}
	return rotations.FromURL(httputils.NewTimeoutClient(), sheriffConfig)
}
