// Pieces of standalone interrogation common across platforms--or at least testable on arbitrary
// platforms.
package standalone

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// Split a macOS version like 1.2.3 into an array of versions of all precisions, like ["Mac",
// "Mac-1", "Mac-1.2", "Mac-1.2.3"].
func macVersionsOfAllPrecisions(version string) []string {
	subversions := strings.Split(version, ".")
	ret := []string{"Mac", "Mac-" + subversions[0]}
	for i, subversion := range subversions[1:] {
		ret = append(ret, ret[i+1]+"."+subversion)
	}
	return ret
}

// intelModel extracts the CPU model name from its display name and returns it. For example, it
// pulls "i7-9750H" out of "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz". Returns "" if extraction
// fails.
func intelModel(brandString string) string {
	regexes := []*regexp.Regexp{
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2} [vV]\d) `),
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2}) `),
		regexp.MustCompile(` ([A-Z]\d{4}[A-Z]{0,2}) `),
		regexp.MustCompile(` ((:?[A-Z][a-z]+ )+GCE)`),
	}
	for _, regex := range regexes {
		if matches := regex.FindStringSubmatch(brandString); len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}

// cpusCore is the brains behind the CPUs() function, broken off for testing.
func cpusCore(arch string, vendor string, brandString string) ([]string, error) {
	// gopsutil can spit out i386, i686, arm, aarch64, ia64, or x86_64 under Windows; "" as a
	// fallback; whatever uname's utsname.machine struct member (a string) has on POSIX. On our lab
	// Macs, that's arm64 or x64_86 (exposed by uname -m). Our lab Linuxes: x86_64.
	arch_to_isa := map[string]string{
		"x86_64":  "x86",
		"amd64":   "x86",
		"i386":    "x86",
		"i686":    "x86",
		"aarch64": "arm64",
		"arm64":   "arm64",
	}
	isa, ok := arch_to_isa[arch]
	if !ok {
		return nil, skerr.Fmt("host had unknown architecture: %s", arch)
	}

	var bitness string
	// Fall back to int-size test if the width isn't explicit in the arch name:
	if strings.HasSuffix(arch, "64") || strconv.IntSize > 1<<32 {
		bitness = "64"
	} else {
		bitness = "32"
	}
	ret := []string{isa, fmt.Sprintf("%s-%s", isa, bitness)}

	var model string
	if vendor == "GenuineIntel" {
		model = intelModel(brandString)
	} else {
		model = strings.ReplaceAll(brandString, " ", "_")
	}
	if len(model) > 0 {
		ret = append(ret, fmt.Sprintf("%s-%s-%s", isa, bitness, model))
	}

	return ret, nil
}
