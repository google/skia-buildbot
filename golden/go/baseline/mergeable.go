package baseline

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/types"
)

func WriteMergeableBaseline(w io.Writer, baseLine types.TestExp) error {
	allLines := make([]string, 0, len(baseLine))
	for testName, digests := range baseLine {
		digestList := make([]string, len(digests))
		for d, label := range digests {
			digestList = append(digestList, combineDigestLabel(d, label))
		}
		sort.Strings(digestList)
		line := fmt.Sprintf("%s %s", testName, strings.Join(digestList, " "))
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

func ReadMergeableBaseline(r io.Reader) (types.TestExp, error) {
	lines, err := fileutil.ReadLinesFromReader(r)
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

	for _, line := range lines {
		testName, digests, err := parseLine(line)
		if err != nil {
			return nil, sklog.FmtErrorf("Error parsing line: %s", err)
		}

		if testName < previousTest {
			return nil, sklog.FmtErrorf("Testnames are not monotonically increasing: %s < %s", testName, previousTest)
		}

		if _, ok := ret[testName]; ok {
			return nil, sklog.FmtErrorf("Duplicate testname found: %s", testName)
		}

		ret[testName] = digests
	}

	return ret, nil
}

func parseLine(line string) (string, map[string]types.Label, error) {
	return "", nil, nil
}

var (
	labelToCh = map[types.Label]byte{
		types.UNTRIAGED: 'u',
		types.POSITIVE:  'p',
		types.NEGATIVE:  'n',
	}

	chToLabel = map[byte]types.Label{
		'u': types.UNTRIAGED,
		'p': types.POSITIVE,
		'n': types.NEGATIVE,
	}
)

func combineDigestLabel(digest string, label types.Label) string {
	return fmt.Sprintf("%s:%c")
}

func splitDigestLabel(string) (string, types.Label, error) {
	return "", types.UNTRIAGED, nil
}
