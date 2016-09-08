// Package parser provides a parser for various types of metrics formats.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/ragemon/go/store"
	"go.skia.org/infra/ragemon/go/ts"
)

// PlainText parses input that contains one structured key and a single value
// per line, i.e.:
//
//   ,config=565, 103
//   ,config=8888, 204
//
// Keys are not validated, that is presumed to happen at a different level.
func PlainText(s string) ([]store.Measurement, error) {
	now := time.Now()
	ret := []store.Measurement{}
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		parts := strings.Split(l, " ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid input line format: %q", l)
		}
		key := parts[0]
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Invalid value format: %q", l)
		}
		ret = append(ret, store.Measurement{
			Key: key,
			Point: ts.Point{
				Timestamp: now.Unix(),
				Value:     value,
			},
		})
	}
	return ret, nil
}
