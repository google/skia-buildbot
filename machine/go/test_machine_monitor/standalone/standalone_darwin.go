package standalone

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
)

// OSVersions returns the macOS version in all possible precisions. For example, 10.5.7 would yield
// ["Mac-10", "Mac-10.5", "Mac-10.5.7"].
func OSVersions(ctx context.Context) ([]string, error) {
	_, _, platformVersion, err := host.PlatformInformation()
	if err != nil {
		return []string{}, skerr.Wrapf(err, "Failed to get macOS version.")
	}
	return versionsOfAllPrecisions(platformVersion), nil
}

// Split a macOS version like 1.2.3 into an array of versions of all precisions, like ["Mac",
// "Mac-1", "Mac-1.2", "Mac-1.2.3"].
func versionsOfAllPrecisions(version string) []string {
	subversions := strings.Split(version, ".")
	ret := []string{"Mac", "Mac-" + subversions[0]}
	for i, subversion := range subversions[1:] {
		ret = append(ret, ret[i]+"."+subversion)
	}
	return ret
}

// CPUs returns the model of CPU on the host, in various precisions, e.g. ["x86", "x86-64",
// "x86-64-i5-5350U"].
//
// TODO(erikrose): Implement.
func CPUs(ctx context.Context) ([]string, error) {
	var ret []string
	return ret, nil
}
