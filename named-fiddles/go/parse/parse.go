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
	registerFiddleRegex         = regexp.MustCompile(`REG_FIDDLE\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+)\)`)
	registerAnimatedFiddleRegex = regexp.MustCompile(`REG_FIDDLE_ANIMATED\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+),\s+(?P<duration>\S+)\)`)
	registerSRGBFiddleRegex     = regexp.MustCompile(`REG_FIDDLE_SRGB\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+),\s+(?P<duration>\S+),\s+(?P<usefloat16>\w+)\)`)
)

// ParseCpp parses a Skia example and returns a FiddleContext that's ready to run.
//
// Returns ErrorInactiveExample is the example is ifdef'd out. Other errors
// indicate a failure to parse the code or options.
func ParseCpp(body string) (*types.FiddleContext, error) {
	if body[:3] == "#if" {
		return nil, ErrorInactiveExample
	}

	fCtx, err := tryParsingAsNormalFiddle(body)
	if err == failedToMatchErr {
		fCtx, err = tryParsingAsAnimatedFiddle(body)
		if err == failedToMatchErr {
			fCtx, err = tryParsingAsSRGBFiddle(body)
			if err == failedToMatchErr {
				return nil, skerr.Fmt("failed to find any REG_FIDDLE* macro")
			}
		}
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Extract the code.
	lines := strings.Split(body, "\n")

	var code []string
	foundREG := false
	foundEnd := false
	for _, line := range lines {
		if !foundREG {
			if strings.HasPrefix(line, "REG_FIDDLE") {
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
	fCtx.Code = strings.Join(code, "\n")

	return fCtx, nil
}

var failedToMatchErr = errors.New("failed to match")

func tryParsingAsNormalFiddle(body string) (*types.FiddleContext, error) {
	// Parse up the REG_FIDDLE macro values.
	match := registerFiddleRegex.FindStringSubmatch(body)
	if len(match) == 0 {
		return nil, failedToMatchErr
	}
	namedGroup := func(name string) string {
		for i, groupName := range registerFiddleRegex.SubexpNames() {
			if i != 0 && name == groupName {
				return match[i]
			}
		}
		panic("Could not find group with name " + name)
	}
	width, err := strconv.Atoi(namedGroup("width"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing width")
	}
	height, err := strconv.Atoi(namedGroup("height"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing height")
	}
	source, err := strconv.Atoi(namedGroup("source"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing source")
	}
	textonly := namedGroup("textonly") == "true"
	return &types.FiddleContext{
		Name: namedGroup("name"),
		Options: types.Options{
			Width:    width,
			Height:   height,
			Source:   source,
			TextOnly: textonly,
		},
	}, nil
}

func tryParsingAsAnimatedFiddle(body string) (*types.FiddleContext, error) {
	// Parse up the REG_FIDDLE_ANIMATED macro values.
	match := registerAnimatedFiddleRegex.FindStringSubmatch(body)
	if len(match) == 0 {
		return nil, failedToMatchErr
	}
	namedGroup := func(name string) string {
		for i, groupName := range registerAnimatedFiddleRegex.SubexpNames() {
			if i != 0 && name == groupName {
				return match[i]
			}
		}
		panic("Could not find group with name " + name)
	}
	width, err := strconv.Atoi(namedGroup("width"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing width")
	}
	height, err := strconv.Atoi(namedGroup("height"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing height")
	}
	source, err := strconv.Atoi(namedGroup("source"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing source")
	}
	duration, err := strconv.ParseFloat(namedGroup("duration"), 64)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing duration")
	}
	textonly := namedGroup("textonly") == "true"
	return &types.FiddleContext{
		Name: namedGroup("name"),
		Options: types.Options{
			Width:    width,
			Height:   height,
			Source:   source,
			TextOnly: textonly,
			Duration: duration,
		},
	}, nil
}

func tryParsingAsSRGBFiddle(body string) (*types.FiddleContext, error) {
	// Parse up the REG_FIDDLE_SRGB macro values.
	match := registerSRGBFiddleRegex.FindStringSubmatch(body)
	if len(match) == 0 {
		return nil, failedToMatchErr
	}
	namedGroup := func(name string) string {
		for i, groupName := range registerSRGBFiddleRegex.SubexpNames() {
			if i != 0 && name == groupName {
				return match[i]
			}
		}
		panic("Could not find group with name " + name)
	}
	width, err := strconv.Atoi(namedGroup("width"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing width")
	}
	height, err := strconv.Atoi(namedGroup("height"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing height")
	}
	source, err := strconv.Atoi(namedGroup("source"))
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing source")
	}
	duration, err := strconv.ParseFloat(namedGroup("duration"), 64)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing duration")
	}
	textonly := namedGroup("textonly") == "true"
	useFloat16 := namedGroup("usefloat16") == "true"
	return &types.FiddleContext{
		Name: namedGroup("name"),
		Options: types.Options{
			Width:    width,
			Height:   height,
			Source:   source,
			TextOnly: textonly,
			Duration: duration,
			F16:      useFloat16,
		},
	}, nil
}
