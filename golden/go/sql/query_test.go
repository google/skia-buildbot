package sql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

// Locally, this ran at about 30ms per operation for 36400 Traces. It can use the indexes and
// appears to do so in parallel.
func BenchmarkUnionIntersect_AllKeysHaveMultipleValues(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
WITH
Fruit AS (
    SELECT trace_id FROM Traces WHERE keys -> 'fruit' = '"apple"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'fruit' = '"fig"'
),
Callsign AS (
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"foxtrot"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"tango"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"alfa"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"does not exist"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"charlie"'
),
Province AS (
    SELECT trace_id FROM Traces WHERE keys -> 'province' = '"Ontario"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'province' = '"Something"'
)
SELECT COUNT(*) FROM (
SELECT * FROM Fruit
INTERSECT
SELECT * FROM Callsign
INTERSECT
SELECT * FROM Province) x;`)
		_ = row.Scan(&count)
		if count != 2*4*2*1 {
			panic("wrong")
		}
	}
}

// Locally, this ran at about 30ms per operation for 36400 Traces. It can use the indexes and
// appears to do so in parallel.
func BenchmarkUnionIntersect_SomeKeysHaveASingleValue(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
WITH
Callsign AS (
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"foxtrot"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"tango"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"alfa"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"does not exist"'
    UNION
    SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"charlie"'
)
SELECT COUNT(*) FROM (
SELECT trace_id FROM Traces WHERE keys -> 'fruit' = '"apple"'
INTERSECT
SELECT * FROM Callsign
INTERSECT
SELECT trace_id FROM Traces WHERE keys -> 'province' = '"Ontario"') x;`)
		_ = row.Scan(&count)
		if count != 1*4*1*2 {
			panic("wrong")
		}
	}
}

// Locally, this ran at about 10ms per operation for 36400 Traces. It can use the indexes and
// appears to do so in parallel.
func BenchmarkUnionIntersect_AllKeysHaveASingleValue(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
SELECT COUNT(*) FROM (
SELECT trace_id FROM Traces WHERE keys -> 'fruit' = '"apple"'
INTERSECT
SELECT trace_id FROM Traces WHERE keys -> 'callsign' = '"foxtrot"'
INTERSECT
SELECT trace_id FROM Traces WHERE keys -> 'province' = '"Ontario"') x;`)
		_ = row.Scan(&count)
		if count != 1*1*1*2 {
			panic("wrong")
		}
	}
}

// Locally, this ran at about 100ms per operation for 36400 Traces. It has to do a full table scan.
func BenchmarkAndIN_AllKeysHaveMultipleValues(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
SELECT count(trace_id) FROM Traces where keys -> 'fruit' IN('"apple"','"fig"')
    AND keys -> 'callsign' IN('"foxtrot"','"tango"', '"alfa"', '"does not exist"', '"charlie"')
    AND keys -> 'province' IN('"Ontario"','"Something"');`)
		_ = row.Scan(&count)
		if count != 2*4*1*2 {
			panic("wrong")
		}
	}
}

// Locally, this ran at about 80ms per operation for 36400 Traces. It did a zig-zag join on the
// two single item keys and then a full search over that.
func BenchmarkAndIN_SomeKeysHaveASingleValue(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
SELECT count(trace_id) FROM Traces where keys -> 'fruit' = '"apple"'
    AND keys -> 'callsign' IN('"foxtrot"','"tango"', '"alfa"', '"does not exist"', '"charlie"')
    AND keys -> 'province' = '"Ontario"';`)
		_ = row.Scan(&count)
		if count != 1*4*1*2 {
			panic("wrong")
		}
	}
}

// Locally, this ran at about 70ms per operation for 36400 Traces. It did a zig-zag join on the
//  keys and then a lookup join on that
func BenchmarkAndIN_AllKeysHaveASingleValue(b *testing.B) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, b)
	traces := makeTestTraceRows()
	require.NoError(b, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		Traces: traces,
	}))
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		row := db.QueryRow(ctx, `
SELECT count(trace_id) FROM Traces where keys -> 'fruit' = '"apple"'
    AND keys -> 'callsign' = '"foxtrot"'
    AND keys -> 'province' = '"Ontario"';`)
		_ = row.Scan(&count)
		if count != 1*1*1*2 {
			panic("wrong")
		}
	}
}

const (
	fruitKey    = "fruit"
	callsignKey = "callsign"
	provinceKey = "province"
	numberKey   = "number"
	letterKey   = "letter"
)

// makeTestTraceRows makes many traces filled with keys of arbitrary data that is meant to
// be somewhat representative of the keys in Gold.
func makeTestTraceRows() []schema.TraceRow {
	// https://en.wikibooks.org/wiki/Wikijunior:Fruit_Alphabet
	fruits := []string{"apple", "apricots", "avocados", "banana", "boysenberry", "blueberry",
		"cherry", "cantaloupe", "clementine", "date", "dewberry", "dragon fruit", "elderberry",
		"eggfruit", "evergreen huckleberry", "entawak", "fig", "farkleberry, finger lime",
		"grapefruit", "grapes", "gooseberries", "guava", "Honeydew", "Hackberry", "imbe",
		"Jackfruit", "Jambolan", "Kiwi", "Kumquat", "Lime", "Lemon", "Longan", "Lychee", "Loquat",
		"Mango", "Mulberry", "Melon", "Nectarine", "Olive", "Orange", "Papaya", "Persimmon",
		"Paw Paw", "Prickly Pear", "Peach", "Pomegranate", "Pineapple", "Passion Fruit",
		"Quince", "Quararibea cordata", "Rambutan", "Raspberry", "Rose Hip", "Star Fruit",
		"Strawberry", "Tomato", "Tangerine", "Tamarind", "Ugli Fruit", "Uniq Fruit", "Ugni",
		"Vanilla Bean", "Voavanga", "Watermelon", "Wolfberry", "Xigua", "Ximenia", "Xango",
		"Yangmei", "Zig Zag vine fruit",
	}
	// https://en.wikipedia.org/wiki/NATO_phonetic_alphabet
	callsign := []string{"alfa", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliett", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo",
		"sierra", "tango", "uniform", "victor", "whiskey", "x-ray", "yankee", "zulu"}
	// https://en.wikipedia.org/wiki/Provinces_and_territories_of_Canada
	province := []string{"Alberta", "British Columbia", "Manitoba", "New Brunswick",
		"Newfoundland and Labrador", "Nova Scotia", "Ontario", "Prince Edward Island", "Quebec",
		"Saskatchewan"}
	numbers := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight",
		"nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen",
		"eighteen", "nineteen", "many"}

	var rv []schema.TraceRow
	addTraceRow := func(p paramtools.Params) {
		_, traceID := sql.SerializeMap(p)
		_, groupingID := sql.SerializeMap(paramtools.Params{
			types.CorpusField: "a_corpus",
			fruitKey:          p[fruitKey],
		})
		rv = append(rv, schema.TraceRow{
			TraceID:              traceID,
			Corpus:               "a_corpus",
			GroupingID:           groupingID,
			Keys:                 p,
			MatchesAnyIgnoreRule: schema.NBFalse,
		})
	}
	for _, f := range fruits {
		for _, c := range callsign {
			for _, p := range province {
				idx := len(f)
				if idx >= len(numbers) {
					idx = len(numbers) - 1
				}
				addTraceRow(paramtools.Params{
					types.CorpusField: "a_corpus",
					fruitKey:          f,
					callsignKey:       c,
					provinceKey:       p,
					numberKey:         numbers[idx],
				})
				addTraceRow(paramtools.Params{
					types.CorpusField: "a_corpus",
					fruitKey:          f,
					callsignKey:       c,
					provinceKey:       p,
					letterKey:         strings.Repeat(string(f[0]), 6),
				})
			}
		}
	}
	return rv
}
