package baseline

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/types"
)

// WIP - Very experimental and probably not working yet. Do not use in production !
// This is a first draft and has not been tested in how it would do against actual merges.
// This implements a serialization format for expectations/baselines that can be merged
// automatically with Git with low probability of merge conflicts.
//
// Expectations are stored in a text file following this structure:
// 	- Each line contains the expectations for one test.
// 	- The tokens of each line are separated by exactly one white space.
// 	- The first token is the test name.
// 	- Each tokens following the test name are labeled digests.
// 	- Each labeled digest follows this format: <hex_encoded_md5_hash>:<label>, where
// 		hex_encoded_md5_hash is 32 characters long and label is one 'u', 'p', 'n'
// 		(short for 'untriaged', 'positive', 'negative')
// 	- All digests within a line are sorted in ascending order.
// 	- All test names within the file are sorted in ascending order.
//  - Empty lines and lines starting with '#' are ignored.
//
// Note: Labeling the digests might now be necessary, but we have it here so we don't lose any
// information when serializing -> deserializing.

var (
	// isMD5 is used to verify that the given string is a hex-encoded MD5 hash.
	isMD5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

	// validDigestLabel is used to verify that the digest pair follows the format
	// described above.
	validDigestLabel = regexp.MustCompile(`^([0-9a-f]{32}):(u|p|n)$`)

	// labelToCh maps a label to a character.
	labelToCh = map[types.Label]string{
		types.UNTRIAGED: "u",
		types.POSITIVE:  "p",
		types.NEGATIVE:  "n",
	}

	// chToLabel maps a character to a label.
	chToLabel = map[string]types.Label{
		"u": types.UNTRIAGED,
		"p": types.POSITIVE,
		"n": types.NEGATIVE,
	}
)

// WriteMergeableBaseline writes the given expectations to the provided Writer in a file
// format that should be easy to merge for git.
// The input is checked against these conditions:
//    - No empty test names are allowed
//    - All digests must be valid hex-encoded MD5 hashes (32 characters).
func WriteMergeableBaseline(w io.Writer, b types.TestExp) error {
	allLines := make([]string, 0, len(b))
	for testName, digests := range b {
		if testName == "" {
			return sklog.FmtErrorf("Received emtpy testname.")
		}

		digestLabelList := make([]string, 0, len(digests))
		for d, label := range digests {
			if !isMD5.MatchString(string(d)) {
				return sklog.FmtErrorf("Expected hex-encoded MD5 hash. Got: %q", d)
			}
			digestLabelList = append(digestLabelList, combineDigestLabel(d, label))
		}
		sort.Strings(digestLabelList)
		line := fmt.Sprintf("%s %s", testName, strings.Join(digestLabelList, " "))
		allLines = append(allLines, line)
	}
	sort.Strings(allLines)
	for _, line := range allLines {
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			return sklog.FmtErrorf("Error writing line to writer: %s", err)
		}
	}
	return nil
}

// ReadMergeableBaseline reads the expectations from the given reader, expecting the file format
// described above.
// It assumes that the given input file can be the result of Git merging two files that were
// previously written via the WriteMergeableBaseline function.
// It check that the input is consistent with the file format described above.
func ReadMergeableBaseline(r io.Reader) (types.TestExp, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, sklog.FmtErrorf("Error reading lines: %s", err)
	}

	if len(lines) == 0 {
		return types.TestExp{}, nil
	}

	previousTest, digests, err := parseLine(lines[0])
	if err != nil {
		return nil, sklog.FmtErrorf("Error parsing the first line: %s", err)
	}

	ret := types.TestExp{previousTest: digests}

	for _, line := range lines[1:] {
		testName, digests, err := parseLine(line)
		if err != nil {
			return nil, sklog.FmtErrorf("Error parsing line: %s", err)
		}

		if testName < previousTest {
			return nil, sklog.FmtErrorf("Testnames are not monotonically increasing: %s < %s is false", previousTest, testName)
		}

		if _, ok := ret[testName]; ok {
			return nil, sklog.FmtErrorf("Duplicate testname found: %s", testName)
		}

		ret[testName] = digests
	}

	return ret, nil
}

// readLines reads the content of r as lines.
// It filters out empty lines and lines that have "#" as a first character.
// It returns the lines without the trailing '\n' characters.
func readLines(r io.Reader) ([]string, error) {
	result := []string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		result = append(result, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// containsEmpty returns true if any of the strings in the given slice contains an empty string
// or a string with only space-like characters.
func containsEmpty(parts []string) bool {
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return true
		}
	}
	return false
}

// parseLine parses a single entry in the file and returns the test name and the mapping
// from digests to labels, which can be used directly to add to a baseline.
func parseLine(line string) (types.TestName, map[types.Digest]types.Label, error) {
	parts := strings.Split(line, " ")
	if containsEmpty(parts) {
		return "", nil, sklog.FmtErrorf("Tokens in line can only contain one separating space. Multiple found in %q", line)
	}

	// We need to have at least one digest
	if len(parts) < 2 {
		return "", nil, sklog.FmtErrorf("Expectations need to contain at least one image digest. Got: %q", line)
	}

	testName := parts[0]
	digests := make(map[types.Digest]types.Label, len(parts)-1)
	prev := ""
	for _, digestLabel := range parts[1:] {
		digest, label, err := splitDigestLabel(digestLabel)
		if err != nil {
			return "", nil, err
		}

		// Check if they are strictly monotonically increasing. This also covers the
		// case of duplicate digests.
		if string(digest) <= prev {
			return "", nil, sklog.FmtErrorf("Digests for test %q are not sorted or there are duplicates. Got sequence: %q %q", testName, prev, digest)
		}

		digests[digest] = label
		prev = string(digest)
	}
	return types.TestName(parts[0]), digests, nil
}

// combineDigestLabel combines the digest and the label into a string and is the 'inverse' of
// splitDigestLabel.
func combineDigestLabel(digest types.Digest, label types.Label) string {
	return fmt.Sprintf("%s:%s", digest, labelToCh[label])
}

// splitDigestLabel splits the digest and label encoded in a string by combineDigestLabel.
func splitDigestLabel(digestLabel string) (types.Digest, types.Label, error) {
	matchedGroups := validDigestLabel.FindStringSubmatch(digestLabel)
	if len(matchedGroups) != 3 {
		return "", types.UNTRIAGED, sklog.FmtErrorf("Invalid digest/label entry: %q", digestLabel)
	}

	return types.Digest(matchedGroups[1]), chToLabel[matchedGroups[2]], nil
}
