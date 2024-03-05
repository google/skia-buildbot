package sqlutil

import (
	"strconv"
	"strings"
)

// ValuesPlaceholders returns a set of SQL placeholder numbers grouped for use in an INSERT
// statement. For example, ValuesPlaceholders(2,3) returns ($1, $2), ($3, $4), ($5, $6)
// It panics if either param is <= 0.
func ValuesPlaceholders(valuesPerRow, numRows int) string {
	if valuesPerRow <= 0 || numRows <= 0 {
		panic("Cannot make ValuesPlaceholder with 0 rows or 0 values per row")
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
	return values.String()
}

// Returns a where clause with placeholders where each column value
// is ANDed and each row is ORed
// Args:
//
//	cols: List of column names
//	numRows: Number of rows
//
// For example, if cols = ["name", "city"] and numRows = 2, return value
// would be (name=$1 AND city=$2) OR (name=$3 AND city=$4)
func WherePlaceholders(cols []string, numRows int) string {
	if len(cols) <= 0 || numRows <= 0 {
		panic("Cannot make WherePlaceHolders with 0 cols or 0 rows")
	}
	response := strings.Builder{}
	for rowIdx := 1; rowIdx <= numRows; rowIdx += 1 {
		response.WriteString("(")
		for colIdx := 1; colIdx <= len(cols); colIdx++ {
			response.WriteString(cols[colIdx-1])
			response.WriteString("=")
			response.WriteString("$")
			response.WriteString(strconv.Itoa((rowIdx-1)*len(cols) + colIdx))
			if colIdx != len(cols) {
				response.WriteString(" AND ")
			}
		}
		response.WriteString(")")
		if rowIdx < numRows {
			response.WriteString(" OR ")
		}
	}
	return response.String()
}
