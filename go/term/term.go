package term

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"golang.org/x/term"
)

// TerminalWidthGetter is a function which returns the width of the terminal.
type TerminalWidthGetter func() int

// GetTerminalWidthFunc returns a TerminalWidthGetter which finds the width of
// the terminal or returns the given default if not running in a terminal.
func GetTerminalWidthFunc(defaultWidth int) TerminalWidthGetter {
	return func() int {
		terminalWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			return terminalWidth
		}
		return defaultWidth
	}
}

// TableConfig provides configuration for building textual tables.
type TableConfig struct {
	MaxLineWidth   int
	MaxColumnWidth int
	// GetTerminalWidth takes precedence over MaxLineWidth if provided.
	GetTerminalWidth TerminalWidthGetter
	// IncludeHeader indicates whether the first line of data is a header.
	IncludeHeader bool
	// JSONTagsAsHeaders indicates whether to use struct `json` tags as the
	// values for the header line. Implies IncludeHeader. Only used with
	// Structs().
	JSONTagsAsHeaders bool
	// TimeAsDiffs causes time.Time fields to be converted to human-readable
	// diffs from the current time.
	TimeAsDiffs bool
	// EmptyCollectionsBlank causes empty slices and maps to appear blank as
	// opposed to "[]" or "map[]".
	EmptyCollectionsBlank bool
}

// MakeTable creates a textual table from the given data, which is presumed to
// be broken into rows of cells.
func (c TableConfig) MakeTable(data [][]string) string {
	maxLineWidth := c.MaxLineWidth
	if c.GetTerminalWidth != nil {
		maxLineWidth = c.GetTerminalWidth()
	}
	numColumns := 0
	for _, row := range data {
		if numColumns < len(row) {
			numColumns = len(row)
		}
	}
	columnWidths := make([]int, numColumns)
	for _, row := range data {
		for colIdx, cell := range row {
			if columnWidths[colIdx] < len(cell) {
				columnWidths[colIdx] = len(cell)
			}
		}
	}
	if c.MaxColumnWidth > 0 {
		for idx, colWidth := range columnWidths {
			if colWidth > c.MaxColumnWidth {
				columnWidths[idx] = c.MaxColumnWidth
			}
		}
	}
	rv := make([]string, 0, len(data))
	for idx, row := range data {
		str := make([]string, 0, len(row))
		for colIdx, cell := range row {
			// If there are multiple lines in the string, keep only the first.
			cell = strings.Split(cell, "\n")[0]
			cell = strings.TrimSpace(cell)
			if c.MaxColumnWidth > 0 {
				cell = util.TruncateNoEllipses(cell, c.MaxColumnWidth)
			}
			padding := columnWidths[colIdx] - len(cell)
			cell = cell + strings.Repeat(" ", padding)
			str = append(str, cell)
		}
		line := strings.Join(str, " ")
		if maxLineWidth > 0 {
			line = util.TruncateNoEllipses(line, maxLineWidth)
		}
		line = strings.TrimSpace(line)
		rv = append(rv, line)

		if idx == 0 && (c.IncludeHeader || c.JSONTagsAsHeaders) {
			rv = append(rv, strings.Repeat("-", len(line)))
		}
	}
	return strings.Join(rv, "\n")
}

// valueToStrings converts a reflect.Value to at least one string, flattening
// nested structs as needed.
func (c TableConfig) valueToStrings(elem reflect.Value, isHeader bool, now time.Time) []string {
	typ := elem.Type()
	if typ.Kind() == reflect.Pointer {
		elem = elem.Elem()
		typ = elem.Type()
	}
	if typ == reflect.TypeOf(time.Time{}) {
		if c.TimeAsDiffs {
			ts := elem.Interface().(time.Time)
			return []string{human.Duration(now.Sub(ts))}
		} else {
			vals := elem.MethodByName("String").Call([]reflect.Value{})
			return []string{vals[0].String()}
		}
	} else if typ.Implements(reflect.TypeOf((*fmt.Stringer)(nil)).Elem()) {
		vals := elem.MethodByName("String").Call([]reflect.Value{})
		return []string{vals[0].String()}
	} else if typ.Kind() == reflect.Struct {
		row := make([]string, 0, elem.NumField())
		for f := 0; f < elem.NumField(); f++ {
			field := elem.Field(f)
			fieldTyp := typ.Field(f).Type
			if fieldTyp.Kind() == reflect.Pointer {
				fieldTyp = fieldTyp.Elem()
			}
			// Flatten nested structs, but treat time.Time as a scalar value.
			if fieldTyp.Kind() == reflect.Struct && fieldTyp != reflect.TypeOf(time.Time{}) {
				row = append(row, c.valueToStrings(field, isHeader, now)...)
			} else if isHeader {
				if c.JSONTagsAsHeaders {
					row = append(row, typ.Field(f).Tag.Get("json"))
				} else {
					row = append(row, typ.Field(f).Name)
				}
			} else {
				row = append(row, c.valueToStrings(field, isHeader, now)...)
			}
		}
		return row
	} else if c.EmptyCollectionsBlank &&
		(typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map) &&
		elem.Len() == 0 {
		return []string{""}
	} else {
		return []string{fmt.Sprintf("%v", elem)}
	}
}

// Structs creates a table from a slice of structs.
func (c TableConfig) Structs(ctx context.Context, structs interface{}) (string, error) {
	ts := now.Now(ctx)
	typ := reflect.TypeOf(structs)
	if typ.Kind() != reflect.Slice {
		return "", skerr.Fmt("invalid kind %s", typ.Kind())
	}
	if typ.Elem().Kind() != reflect.Struct && (typ.Elem().Kind() == reflect.Pointer && typ.Elem().Elem().Kind() != reflect.Struct) {
		return "", skerr.Fmt("expected a slice of structs but found %q", typ.Elem().Kind())
	}
	val := reflect.ValueOf(structs)
	data := make([][]string, 0, val.Len())
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		if i == 0 && (c.IncludeHeader || c.JSONTagsAsHeaders) {
			headerRow := c.valueToStrings(elem, true, ts)
			data = append(data, headerRow)
		}
		row := c.valueToStrings(elem, false, ts)
		data = append(data, row)
	}
	return c.MakeTable(data), nil
}
