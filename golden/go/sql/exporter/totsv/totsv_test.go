package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

type testTables struct {
	TableOne []tableOneRow
	TableTwo []tableTwoRow
}

type tableOneRow struct {
	ColumnOne string
	ColumnTwo int
}

func (r tableOneRow) ToTSV() string {
	return fmt.Sprintf("%s\t%d", r.ColumnOne, r.ColumnTwo)
}

type tableTwoRow struct {
	ColumnOne   int
	ColumnTwo   int
	ColumnThree int
}

func (r tableTwoRow) ToTSV() string {
	return fmt.Sprintf("%d\t%d\t%d", r.ColumnOne, r.ColumnTwo, r.ColumnThree)
}

func TestGenerateTSV_WellFormedInput_CorrectOutput(t *testing.T) {
	unittest.SmallTest(t)

	gen := generateTSV(testTables{
		TableOne: []tableOneRow{{
			ColumnOne: "first", ColumnTwo: 1,
		}, {
			ColumnOne: "second", ColumnTwo: 2,
		}},
		TableTwo: []tableTwoRow{{
			ColumnOne: 1, ColumnTwo: 2, ColumnThree: 3,
		}, {
			ColumnOne: 4, ColumnTwo: 5, ColumnThree: 6,
		}, {
			ColumnOne: 7, ColumnTwo: 8, ColumnThree: 9,
		}},
	})

	assert.Equal(t, map[string]string{
		"TableOne": `first	1
second	2
`,
		"TableTwo": `1	2	3
4	5	6
7	8	9
`,
	}, gen)
}

type malformedTable struct {
	TableOne []tableOneRow
	TableTwo tableTwoRow
}

func TestGenerateSQL_NonSliceField_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		generateTSV(malformedTable{
			TableOne: []tableOneRow{{}},
			TableTwo: tableTwoRow{},
		})
	})
}

type missingToTSVStructs struct {
	TableOne []tableOneRow
	TableTwo []string
}

func TestGenerateSQL_RowsDoNotImplementToTSV_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		generateTSV(missingToTSVStructs{
			TableOne: []tableOneRow{{}},
			TableTwo: []string{"oops", "no", "TSV"},
		})
	})
}
