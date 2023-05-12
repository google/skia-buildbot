package term

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
)

func TestMakeTable_ColumnsEqual(t *testing.T) {
	require.Equal(t, `a1blah a2           a3 a4
b1     b2           b3 b4
c1     c2dssdafa3w3 c3 c4`, TableConfig{}.MakeTable([][]string{
		{"a1blah", "a2", "a3", "a4"},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2dssdafa3w3", "c3", "c4"},
	}))
}

func TestMakeTable_UnevenRows(t *testing.T) {
	require.Equal(t, `a1 a2 a3 a4 a5
b1 b2 b3 b4
c1 c2 c3
d1 d2
e1

g1 g2`, TableConfig{}.MakeTable([][]string{
		{"a1", "a2", "a3", "a4", "a5"},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2", "c3"},
		{"d1", "d2"},
		{"e1"},
		{},
		{"g1", "g2"},
	}))
}

func TestMakeTable_MaxLineWidth(t *testing.T) {
	require.Equal(t, `a1blah a2 a3 a4longl
b1     b2 b3 b4
c1     c2 c3 c4longl`, TableConfig{MaxLineWidth: 20}.MakeTable([][]string{
		{"a1blah", "a2", "a3", "a4longlinelength"},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2", "c3", "c4longlinelength"},
	}))
}

func TestMakeTable_TerminalWidthGetter(t *testing.T) {
	require.Equal(t, `a1blah a2 a3 a4longl
b1     b2 b3 b4
c1     c2 c3 c4longl`,
		TableConfig{
			MaxLineWidth:     100, // Ignored in favor of GetTerminalWidth.
			GetTerminalWidth: func() int { return 20 },
		}.MakeTable([][]string{
			{"a1blah", "a2", "a3", "a4longlinelength"},
			{"b1", "b2", "b3", "b4"},
			{"c1", "c2", "c3", "c4longlinelength"},
		}))
}

func TestMakeTable_MaxColumnWidth(t *testing.T) {
	require.Equal(t, `a1blah a2 a3 a4long
b1     b2 b3 b4
c1     c2 c3 c4long`, TableConfig{MaxColumnWidth: 6}.MakeTable([][]string{
		{"a1blah", "a2", "a3", "a4longlinelength"},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2", "c3", "c4longlinelength"},
	}))
}

func TestMakeTable_MultilineStrings(t *testing.T) {
	require.Equal(t, `a1blah a2 a3 this
b1     b2 b3 b4
c1     c2 c3 c4longlinelength`, TableConfig{}.MakeTable([][]string{
		{"a1blah", "a2", "a3", `this
string
has

multiple

lines`},
		{"b1", "b2", "b3", "b4"},
		{"c1", "c2", "c3", "c4longlinelength"},
	}))
}

func TestStructs(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string            `json:"string"`
		Int    int64             `json:"int"`
		Float  float64           `json:"float"`
		Array  [3]string         `json:"array"`
		Slice  []string          `json:"slice"`
		Map    map[string]string `json:"map"`
	}{
		{
			String: "a1",
			Int:    1,
			Float:  1.2,
			Array:  [3]string{"a1", "a2", "a3"},
			Slice:  []string{"a1", "a2", "a3"},
			Map:    map[string]string{"ak1": "av1", "ak2": "av2"},
		},
		{
			String: "b1",
			Int:    2,
			Float:  2.2,
			Array:  [3]string{"b1", "b2", "b3"},
			Slice:  []string{"b1", "b2", "b3"},
			Map:    map[string]string{"bk1": "bv1", "bk2": "bv2"},
		},
		{
			String: "c1",
			Int:    3,
			Float:  3.2,
			Array:  [3]string{"c1", "c2", "c3"},
			Slice:  []string{"c1", "c2", "c3"},
			Map:    map[string]string{"ck1": "cv1", "ck2": "cv2"},
		},
	}
	expect := `a1 1 1.2 [a1 a2 a3] [a1 a2 a3] map[ak1:av1 ak2:av2]
b1 2 2.2 [b1 b2 b3] [b1 b2 b3] map[bk1:bv1 bk2:bv2]
c1 3 3.2 [c1 c2 c3] [c1 c2 c3] map[ck1:cv1 ck2:cv2]`
	actual, err := TableConfig{}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_IncludeHeader(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string            `json:"string"`
		Int    int64             `json:"int"`
		Float  float64           `json:"float"`
		Array  [3]string         `json:"array"`
		Slice  []string          `json:"slice"`
		Map    map[string]string `json:"map"`
	}{
		{
			String: "a1",
			Int:    1,
			Float:  1.2,
			Array:  [3]string{"a1", "a2", "a3"},
			Slice:  []string{"a1", "a2", "a3"},
			Map:    map[string]string{"ak1": "av1", "ak2": "av2"},
		},
		{
			String: "b1",
			Int:    2,
			Float:  2.2,
			Array:  [3]string{"b1", "b2", "b3"},
			Slice:  []string{"b1", "b2", "b3"},
			Map:    map[string]string{"bk1": "bv1", "bk2": "bv2"},
		},
		{
			String: "c1",
			Int:    3,
			Float:  3.2,
			Array:  [3]string{"c1", "c2", "c3"},
			Slice:  []string{"c1", "c2", "c3"},
			Map:    map[string]string{"ck1": "cv1", "ck2": "cv2"},
		},
	}
	expect := `String Int Float Array      Slice      Map
------------------------------------------
a1     1   1.2   [a1 a2 a3] [a1 a2 a3] map[ak1:av1 ak2:av2]
b1     2   2.2   [b1 b2 b3] [b1 b2 b3] map[bk1:bv1 bk2:bv2]
c1     3   3.2   [c1 c2 c3] [c1 c2 c3] map[ck1:cv1 ck2:cv2]`
	actual, err := TableConfig{IncludeHeader: true}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_IncludeHeader_JSONTagsAsHeaders(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string            `json:"string"`
		Int    int64             `json:"int"`
		Float  float64           `json:"float"`
		Array  [3]string         `json:"array"`
		Slice  []string          `json:"slice"`
		Map    map[string]string `json:"map"`
	}{
		{
			String: "a1",
			Int:    1,
			Float:  1.2,
			Array:  [3]string{"a1", "a2", "a3"},
			Slice:  []string{"a1", "a2", "a3"},
			Map:    map[string]string{"ak1": "av1", "ak2": "av2"},
		},
		{
			String: "b1",
			Int:    2,
			Float:  2.2,
			Array:  [3]string{"b1", "b2", "b3"},
			Slice:  []string{"b1", "b2", "b3"},
			Map:    map[string]string{"bk1": "bv1", "bk2": "bv2"},
		},
		{
			String: "c1",
			Int:    3,
			Float:  3.2,
			Array:  [3]string{"c1", "c2", "c3"},
			Slice:  []string{"c1", "c2", "c3"},
			Map:    map[string]string{"ck1": "cv1", "ck2": "cv2"},
		},
	}
	expect := `string int float array      slice      map
------------------------------------------
a1     1   1.2   [a1 a2 a3] [a1 a2 a3] map[ak1:av1 ak2:av2]
b1     2   2.2   [b1 b2 b3] [b1 b2 b3] map[bk1:bv1 bk2:bv2]
c1     3   3.2   [c1 c2 c3] [c1 c2 c3] map[ck1:cv1 ck2:cv2]`
	actual, err := TableConfig{JSONTagsAsHeaders: true}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_Pointers(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []*struct {
		String string            `json:"string"`
		Int    int64             `json:"int"`
		Float  float64           `json:"float"`
		Array  [3]string         `json:"array"`
		Slice  []string          `json:"slice"`
		Map    map[string]string `json:"map"`
	}{
		{
			String: "a1",
			Int:    1,
			Float:  1.2,
			Array:  [3]string{"a1", "a2", "a3"},
			Slice:  []string{"a1", "a2", "a3"},
			Map:    map[string]string{"ak1": "av1", "ak2": "av2"},
		},
		{
			String: "b1",
			Int:    2,
			Float:  2.2,
			Array:  [3]string{"b1", "b2", "b3"},
			Slice:  []string{"b1", "b2", "b3"},
			Map:    map[string]string{"bk1": "bv1", "bk2": "bv2"},
		},
		{
			String: "c1",
			Int:    3,
			Float:  3.2,
			Array:  [3]string{"c1", "c2", "c3"},
			Slice:  []string{"c1", "c2", "c3"},
			Map:    map[string]string{"ck1": "cv1", "ck2": "cv2"},
		},
	}
	expect := `a1 1 1.2 [a1 a2 a3] [a1 a2 a3] map[ak1:av1 ak2:av2]
b1 2 2.2 [b1 b2 b3] [b1 b2 b3] map[bk1:bv1 bk2:bv2]
c1 3 3.2 [c1 c2 c3] [c1 c2 c3] map[ck1:cv1 ck2:cv2]`
	actual, err := TableConfig{}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_NestedStructs(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	type nested struct {
		Int   int64             `json:"int"`
		Float float64           `json:"float"`
		Array [3]string         `json:"array"`
		Slice []string          `json:"slice"`
		Map   map[string]string `json:"map"`
	}
	data := []struct {
		String string `json:"string"`
		Nested nested `json:"nested"`
	}{
		{
			String: "a1",
			Nested: nested{
				Int:   1,
				Float: 1.2,
				Array: [3]string{"a1", "a2", "a3"},
				Slice: []string{"a1", "a2", "a3"},
				Map:   map[string]string{"ak1": "av1", "ak2": "av2"},
			},
		},
		{
			String: "b1",
			Nested: nested{
				Int:   2,
				Float: 2.2,
				Array: [3]string{"b1", "b2", "b3"},
				Slice: []string{"b1", "b2", "b3"},
				Map:   map[string]string{"bk1": "bv1", "bk2": "bv2"},
			},
		},
		{
			String: "c1",
			Nested: nested{
				Int:   3,
				Float: 3.2,
				Array: [3]string{"c1", "c2", "c3"},
				Slice: []string{"c1", "c2", "c3"},
				Map:   map[string]string{"ck1": "cv1", "ck2": "cv2"},
			},
		},
	}
	expect := `String Int Float Array      Slice      Map
------------------------------------------
a1     1   1.2   [a1 a2 a3] [a1 a2 a3] map[ak1:av1 ak2:av2]
b1     2   2.2   [b1 b2 b3] [b1 b2 b3] map[bk1:bv1 bk2:bv2]
c1     3   3.2   [c1 c2 c3] [c1 c2 c3] map[ck1:cv1 ck2:cv2]`
	actual, err := TableConfig{IncludeHeader: true}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_Time(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string    `json:"string"`
		Time   time.Time `json:"tume"`
	}{
		{
			String: "a1",
			Time:   time.Unix(1683903910, 0),
		},
		{
			String: "b1",
			Time:   time.Unix(1683904941, 0),
		},
		{
			String: "c1",
			Time:   time.Unix(1683904946, 0),
		},
	}
	expect := `a1 2023-05-12 15:05:10 +0000 UTC
b1 2023-05-12 15:22:21 +0000 UTC
c1 2023-05-12 15:22:26 +0000 UTC`
	actual, err := TableConfig{}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_TimeAsDiffs(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string    `json:"string"`
		Time   time.Time `json:"tume"`
	}{
		{
			String: "a1",
			Time:   time.Unix(1683903910, 0),
		},
		{
			String: "b1",
			Time:   time.Unix(1683904941, 0),
		},
		{
			String: "c1",
			Time:   time.Unix(1683904946, 0),
		},
	}
	expect := `a1 2h 13m
b1 1h 55m
c1 1h 55m`
	actual, err := TableConfig{
		TimeAsDiffs: true,
	}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_TimeInNestedStruct(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	type nested struct {
		Time time.Time `json:"time"`
	}
	data := []struct {
		String string `json:"string"`
		Nested nested `json:"nested"`
	}{
		{
			String: "a1",
			Nested: nested{
				Time: time.Unix(1683903910, 0),
			},
		},
		{
			String: "b1",
			Nested: nested{
				Time: time.Unix(1683904941, 0),
			},
		},
		{
			String: "c1",
			Nested: nested{
				Time: time.Unix(1683904946, 0),
			},
		},
	}
	expect := `string time
-----------
a1     2023-05-12 15:05:10 +0000 UTC
b1     2023-05-12 15:22:21 +0000 UTC
c1     2023-05-12 15:22:26 +0000 UTC`
	actual, err := TableConfig{
		IncludeHeader:     true,
		JSONTagsAsHeaders: true,
	}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestStructs_EmptyCollectionsBlank(t *testing.T) {
	ctx := now.TimeTravelingContext(time.Unix(1683911893, 0))
	data := []struct {
		String string            `json:"string"`
		Slice  []string          `json:"slice"`
		Map    map[string]string `json:"map"`
	}{
		{
			String: "a1",
			Slice:  []string{"a", "a"},
			Map:    map[string]string{"ka": "va"},
		},
		{
			String: "b1",
			Slice:  []string{},
			Map:    map[string]string{"kb": "vb"},
		},
		{
			String: "c1",
			Slice:  []string{"c", "c"},
			Map:    map[string]string{},
		},
	}
	expect := `a1 [a a] map[ka:va]
b1       map[kb:vb]
c1 [c c]`
	actual, err := TableConfig{
		EmptyCollectionsBlank: true,
	}.Structs(ctx, data)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}
