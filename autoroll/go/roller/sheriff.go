package roller

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sheriff_endpoints"
	"go.skia.org/infra/go/sklog"
)

// Update the current sheriff list.
func getSheriff(parentName, childName, metricsName string, sheriffSources, backupSheriffs []string) ([]string, error) {
	tags := map[string]string{
		"roller": metricsName,
	}
	m := metrics2.GetInt64Metric("autoroll_get_sheriff_success", tags)
	allEmails := []string{}
	for _, s := range sheriffSources {
		emails, err := sheriff_endpoints.GetSheriffEmails(s)
		if err != nil {
			sklog.Errorf("Failed to retrieve sheriff(s): %s", err)
			m.Update(0)
			if len(backupSheriffs) == 0 {
				return nil, fmt.Errorf("Failed to retrieve sheriffs and no backup sheriffs supplied!  %s", err)
			}
			return backupSheriffs, nil
		}
		// TODO(borenet): Do we need this any more?
		if strings.Contains(parentName, "Chromium") && childName != "WebRTC" && childName != "Perfetto" {
			for i, s := range emails {
				emails[i] = strings.Replace(s, "google.com", "chromium.org", 1)
			}
		}
		allEmails = append(allEmails, emails...)
	}
	m.Update(1)
	return allEmails, nil
}
