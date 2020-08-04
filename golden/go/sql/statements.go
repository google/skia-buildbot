package sql

import (
	"crypto/md5"
	"encoding/json"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
)

func ValuesPlaceholders(valuesPerRow, numRows int) (string, error) {
	if valuesPerRow <= 0 || numRows <= 0 {
		return "", skerr.Fmt("Cannot make ValuesPlaceholder with 0 rows or 0 values per row")
	}
	values := strings.Builder{}
	// There are at most 5 bytes per value that need to be written
	values.Grow(5 * valuesPerRow * numRows)
	// All WriteString calls below return nil errors, as specified in the documentation of
	// strings.Builder, so it is safe to ignore them.
	for argIdx := 1; argIdx <= valuesPerRow*numRows; argIdx += valuesPerRow {
		if argIdx != 1 {
			_, _ = values.WriteString(",")
		}
		_, _ = values.WriteString("(")
		for i := 0; i < valuesPerRow; i++ {
			if i != 0 {
				_, _ = values.WriteString(",")
			}
			_, _ = values.WriteString("$")
			_, _ = values.WriteString(strconv.Itoa(argIdx + i))
		}
		_, _ = values.WriteString(")")
	}
	return values.String(), nil
}

// SerializeMap returns the given map in JSON and the md5 of that json string.
func SerializeMap(m map[string]string) (string, []byte, error) {
	str, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	h := md5.Sum(str)
	return string(str), h[:], err
}
