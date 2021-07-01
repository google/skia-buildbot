package parse

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/skerr"
)

var (
	ErrorInactiveExample = errors.New("Inactive example (ifdef'd out)")

	// registerFiddleRegex is used to parse the REG_FIDDLE macro found in the sample code.
	registerFiddleRegex = regexp.MustCompile(`REG_FIDDLE\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+)\)`)
)

// ParseCpp parses a Skia example and returns a FiddleContext that's ready to run.
//
// Returns ErrorInactiveExample is the example is ifdef'd out. Other errors
// indicate a failure to parse the code or options.
func ParseCpp(body string) (*types.FiddleContext, error) {
	if body[:3] == "#if" {
		return nil, ErrorInactiveExample
	}

	// Parse up the REG_FIDDLE macro values.
	match := registerFiddleRegex.FindStringSubmatch(body)
	if len(match) == 0 {
		return nil, skerr.Fmt("failed to find REG_FIDDLE macro")
	}
	width, err := strconv.Atoi(namedGroup(match, "width"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing width")
	}
	height, err := strconv.Atoi(namedGroup(match, "height"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing height")
	}
	source, err := strconv.Atoi(namedGroup(match, "source"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing source")
	}
	textonly := namedGroup(match, "textonly") == "true"

	// Extract the code.
	lines := strings.Split(body, "\n")

	var code []string
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
		return nil, skerr.Fmt("failed to find END FIDDLE")
	}

	ret := &types.FiddleContext{
		Name: namedGroup(match, "name"),
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

// namedGroup returns the match result of the named group. If an invalid group name is passed in,
// this function panics.
func namedGroup(match []string, name string) string {
	for i, groupName := range registerFiddleRegex.SubexpNames() {
		if i != 0 && name == groupName {
			return match[i]
		}
	}
	panic("Could not find group with name " + name)
}
