package parse

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/fiddlek/go/types"
)

var (
	ErrorInactiveExample = errors.New("Inactive example (ifdef'd out)")

	// re is used to parse the REG_FIDDLE macro found in the sample code.
	re = regexp.MustCompile(`REG_FIDDLE\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+)\)`)
)

const (
	// The indices into the capture groups in the 're' regexp.
	NAME     = 1
	WIDTH    = 2
	HEIGHT   = 3
	TEXTONLY = 4
	SOURCE   = 5
)

// parseCpp parses a Skia example and returns a FiddleContext that's ready to run.
//
// Returns ErrorInactiveExample is the example is ifdef'd out. Other errors
// indicate a failure to parse the code or options.
func ParseCpp(body string) (*types.FiddleContext, error) {
	if body[:3] == "#if" {
		return nil, ErrorInactiveExample
	}

	// Parse up the REG_FIDDLE macro values.
	match := re.FindStringSubmatch(body)
	if len(match) != 6 {
		return nil, fmt.Errorf("Failed to find REG_FIDDLE macro.")
	}
	width, err := strconv.Atoi(match[WIDTH])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse width: %s", err)
	}
	height, err := strconv.Atoi(match[HEIGHT])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse height: %s", err)
	}
	source, err := strconv.Atoi(match[SOURCE])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse source: %s", err)
	}
	textonly := match[TEXTONLY] == "true"

	// Extract the code.
	lines := strings.Split(body, "\n")

	code := []string{}
	foundREG := false
	foundEnd := false
	for _, line := range lines {
		if !foundREG {
			if strings.HasPrefix(line, "REG_FIDDLE(") {
				foundREG = true
			}
			continue
		}
		if strings.Contains(line, "END FIDDLE") {
			foundEnd = true
			break
		}
		code = append(code, line)
	}

	if !foundEnd {
		return nil, fmt.Errorf("Failed to find END FIDDLE.")
	}

	ret := &types.FiddleContext{
		Name: match[NAME],
		Code: strings.Join(code, "\n"),
		Options: types.Options{
			Width:    width,
			Height:   height,
			Source:   source,
			TextOnly: textonly,
		},
	}
	return ret, nil
}
