package parse

import (
	"errors"
	"fmt"
	"regexp"

	"go.skia.org/infra/fiddlek/go/types"
)

var (
	ErrorInactiveExample = errors.New("Inactive example (ifdef'd out)")
	re                   = regexp.MustCompile(`REG_FIDDLE\((?P<name>\w+),\s+(?P<width>\w+),\s+(?P<height>\w+),\s+(?P<textonly>\w+),\s+(?P<source>\w+)\)`)
)

func parseCpp(body string) (*types.FiddleContext, error) {
	if body[:3] == "#if" {
		return nil, ErrorInactiveExample
	}
	match := re.FindStringSubmatch(body)
	if len(match) != 6 {
		return nil, fmt.Errorf("Failed to find REG_FIDDLE macro.")
	}
	return nil, nil
	/*
		ret := &types.FiddleContext{}
		lines := strings.Split(body, "\n")
	*/
}
