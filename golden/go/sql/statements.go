package sql

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"go.skia.org/infra/go/jsonutils"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
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

func InPlaceholders(firstNumber, numElements int) (string, error) {
	if firstNumber <= 0 || numElements <= 0 {
		return "", skerr.Fmt("Cannot make InPlaceholders with firstNumber %d or %d elements", firstNumber, numElements)
	}

	values := strings.Builder{}
	// There are at most 5 bytes per value that need to be written
	values.Grow(5 * numElements)

	_, _ = values.WriteString("(")
	for argIdx := 0; argIdx < numElements; argIdx++ {
		if argIdx != 0 {
			_, _ = values.WriteString(",")
		}
		_, _ = values.WriteString("$")
		_, _ = values.WriteString(strconv.Itoa(argIdx + firstNumber))
	}
	_, _ = values.WriteString(")")
	return values.String(), nil
}

// SerializeMap returns the given map in JSON and the md5 of that json string. nil maps will be
// treated as empty maps.
func SerializeMap(m map[string]string) (string, []byte) {
	var jsonBytes []byte
	if len(m) == 0 {
		jsonBytes = []byte("{}")
	} else {
		jsonBytes = jsonutils.MarshalStringMap(m)
	}

	h := md5.Sum(jsonBytes)
	return string(jsonBytes), h[:]
}

// DigestToBytes returns the given digest as bytes. It returns an error if a missing digest is
// passed in
func DigestToBytes(d types.Digest) (Digest, error) {
	if len(d) == 0 {
		return nil, skerr.Fmt("Empty/missing digest passed in")
	}
	return hex.DecodeString(string(d))
}

// TraceValuesShards is the number of shards we use in the TraceValues Table.
const TraceValuesShards = 8

// ComputeTraceValueShard computes a shard in the range [0,TraceValuesShards) based off the given
// trace hash bytes. This shard range was chosen to be a small number (relative to the thousands
// of ranges that the TraceValues database occupies) that is larger than the number of nodes in the
// cockroachdb cluster. See the overall design doc for more explanation about data locality and
// sharding.
func ComputeTraceValueShard(traceHash TraceID) []byte {
	return []byte{traceHash[0] % TraceValuesShards}
}

type ExpectationsLabel int

const (
	FallbackToPrimaryBranch ExpectationsLabel = -1
	LabelUntriaged          ExpectationsLabel = 0
	LabelPositive           ExpectationsLabel = 1
	LabelNegative           ExpectationsLabel = 2
)

// ConvertLabelFromString converts the string form of a label to an int which is stored in the SQL
// database.
func ConvertLabelFromString(e expectations.Label) ExpectationsLabel {
	switch e {
	case expectations.Untriaged:
		return LabelUntriaged
	case expectations.Positive:
		return LabelPositive
	case expectations.Negative:
		return LabelNegative
	}
	panic("unknown label " + e)
}

// ConvertIgnoreRules turns a Paramset into a SQL clause that would match rows using a column
// named "keys". It is currently implemented with AND/OR clauses, but could potentially be done
// with UNION/INTERSECT depending on performance needs.
func ConvertIgnoreRules(rules []paramtools.ParamSet) (string, []interface{}) {
	if len(rules) == 0 {
		return "false", nil
	}
	conditions := make([]string, 0, len(rules))
	var arguments []interface{}
	argIdx := 1

	for _, rule := range rules {
		rule.Normalize()
		keys := make([]string, 0, len(rule))
		for key := range rule {
			keys = append(keys, key)
		}
		sort.Strings(keys) // sort the keys for determinism

		andParts := make([]string, 0, len(rules))
		for _, key := range keys {
			values := rule[key]
			subCondition := fmt.Sprintf("keys ->> $%d::STRING IN (", argIdx)
			argIdx++
			arguments = append(arguments, key)
			for i, value := range values {
				if i != 0 {
					subCondition += ", "
				}
				subCondition += fmt.Sprintf("$%d", argIdx)
				argIdx++
				arguments = append(arguments, value)
			}
			subCondition += ")"
			andParts = append(andParts, subCondition)
		}
		condition := "(" + strings.Join(andParts, " AND ") + ")"
		conditions = append(conditions, condition)
	}
	combined := "(" + strings.Join(conditions, "\nOR ") + ")"
	return combined, arguments
}

type ChangelistStatus int

const (
	StatusOpen      ChangelistStatus = 0
	StatusMerged    ChangelistStatus = 1
	StatusAbandoned ChangelistStatus = 2
)

// This formats the time to have millisecond precision in a format CockroachDB understands
// https://www.cockroachlabs.com/docs/v20.2/time.html#timetz
const sqlFormatString = "2006-01-02 15:04:05.000-07:00"

func FormatTime(t time.Time) string {
	return t.Format(sqlFormatString)
}
