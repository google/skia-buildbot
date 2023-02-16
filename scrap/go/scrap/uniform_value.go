// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
)

var (
	uniformFloatScalarRegex = regexp.MustCompile(`^float([234]?)$`)
	uniformFloatMatrixRegex = regexp.MustCompile(`^float([234])x([234])$`)
)

// uniformValue represents a single instance of a uniform definition
// in a shader script. For example a definition like:
//
// uniform float3 iColor;
//
// will have a Name of "iColor", and a Type of "float3".
type uniformValue struct {
	Name string
	Type string
}

// numFloats will return the number of floats needed to represent a
// uniformValue.
func (u uniformValue) numFloats() (int, error) {
	if u.Type == "float" {
		return 1, nil
	}
	m := uniformFloatScalarRegex.FindStringSubmatch(u.Type)
	if m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		return n, nil
	}
	if m = uniformFloatMatrixRegex.FindStringSubmatch(u.Type); m != nil {
		x, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		y, err := strconv.Atoi(m[2])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		return x * y, nil
	}
	return 0, skerr.Fmt("Can't determine size of %q", u.Type)
}

// floatSliceToString will convert a slice of float32's to a string
// representation. e.g. []float{0, 1,2,3} will become "0.0f, 1.0f, 2.0f, 3.0f"
func floatSliceToString(floats []float32) string {
	strs := make([]string, len(floats))
	for i := 0; i < len(floats); i++ {
		strs[i] = fmt.Sprintf("%g", floats[i]) // A C++ float literal.
		if !strings.Contains(strs[i], ".") {
			strs[i] = strs[i] + ".0"
		}
		strs[i] = strs[i] + "f"
	}
	return strings.Join(strs, ", ")
}

// getCppDefinitionString return a string, which is valid C++ code, to define
// a uniform value in a builder.
func (u uniformValue) getCppDefinitionString(vals []float32, suffix string) (string, error) {
	switch u.Type {
	case "float":
		return fmt.Sprintf(`builder%s.uniform("%s") = %s;`, suffix, u.Name, floatSliceToString(vals[0:1])), nil
	case "float2":
		return fmt.Sprintf(`builder%s.uniform("%s") = SkV2{%s};`, suffix, u.Name, floatSliceToString(vals[0:2])), nil
	case "float3":
		return fmt.Sprintf(`builder%s.uniform("%s") = SkV3{%s};`, suffix, u.Name, floatSliceToString(vals[0:3])), nil
	case "float4":
		return fmt.Sprintf(`builder%s.uniform("%s") = SkV4{%s};`, suffix, u.Name, floatSliceToString(vals[0:4])), nil
	case "float2x2":
		return fmt.Sprintf(`builder%s.uniform("%s") = SkV4{%s};`, suffix, u.Name, floatSliceToString(vals[0:4])), nil
	}
	return "", skerr.Fmt("Unsupported uniform type %q", u.Type)
}
