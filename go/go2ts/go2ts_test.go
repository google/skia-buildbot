package go2ts

import (
	"bytes"
	"image/color"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_ComplexStruct_Success(t *testing.T) {

	type OtherStruct struct {
		T time.Time `json:"t,omitempty"`
	}

	type InnermostStruct struct {
		InnermostField string
	}

	type StructWithEmbeddedStruct struct {
		InnermostStruct
		Field string
	}

	type StructWithEmbeddedStructPtr struct {
		*InnermostStruct
		Field string
	}

	type AnotherInnermostStruct struct {
		AnotherInnermostField string
	}

	type MultilevelEmbeddedStruct struct {
		*StructWithEmbeddedStruct
		AnotherInnermostStruct
		OutermostField string
	}

	// This struct shows that the outermost fields in embedded structs always take precedence,
	// regardless of whether the overlapping field appears before or after the struct.
	type StructWithOverlappingEmbeddedStructs struct {
		InnermostStruct
		InnermostField        int // Overlaps with InnermostStruct, but has a different type.
		AnotherInnermostField int // Overlaps with AnotherInnermostStruct, but has a different type.
		AnotherInnermostStruct
	}

	type RecursiveStruct struct {
		Field                   string
		Recursion               *RecursiveStruct
		RecursionIgnoreNil      *RecursiveStruct `go2ts:"ignorenil"`
		RecursionSlice          []RecursiveStruct
		RecursionSliceIgnoreNil []RecursiveStruct `go2ts:"ignorenil"`
	}

	type Mode string

	type Offset int

	type Direction string

	const (
		Up    Direction = "up"
		Down  Direction = "down"
		Left  Direction = "left"
		Right Direction = "right"
	)

	var AllDirections = []Direction{Up, Down, Left, Right}

	type AppleVariety string

	const (
		Gala       AppleVariety = "gala"
		Honeycrisp AppleVariety = "honeycrisp"
	)

	var AllAppleVarieties = []AppleVariety{Honeycrisp, Gala}

	type OrangeVariety string

	const (
		Bergamot   OrangeVariety = "bergamot"
		Clementine OrangeVariety = "clementine"
	)

	var AllOrangeVarieties = []OrangeVariety{Bergamot, Clementine}

	type YearlyYield map[int]int

	type Farm struct {
		Name  string
		Stats YearlyYield
	}

	type AppleOrchard struct {
		Farm    Farm
		Variety AppleVariety
	}

	type Data map[string]interface{}

	type ParamSet map[string][]string

	type ParamSetIgnoreNil map[string][]string

	type ComplexStruct struct {
		String                               string
		StringWithAnnotation                 string `json:"s"`
		Bool                                 bool
		Int                                  int
		Float64                              float64
		Time                                 time.Time
		Other                                OtherStruct
		OtherPtr                             *OtherStruct
		WithEmbeddedStruct                   StructWithEmbeddedStruct
		WithEmbeddedStructPtr                StructWithEmbeddedStructPtr
		MultilevelEmbedded                   MultilevelEmbeddedStruct
		StructWithOverlappingEmbeddedStructs StructWithOverlappingEmbeddedStructs
		RecursiveStruct                      RecursiveStruct
		OptionalString                       string       `json:",omitempty"`
		OptionalInt                          int          `json:",omitempty"`
		OptionalFloat64                      float64      `json:",omitempty"`
		OptionalTime                         time.Time    `json:",omitempty"`
		OptionalOtherStruct                  OtherStruct  `json:",omitempty"`
		OptionalOtherStructPtr               *OtherStruct `json:",omitempty"`
		OptionalOtherStructPtrIgnoreNil      *OtherStruct `json:",omitempty" go2ts:"ignorenil"`
		Data                                 Data
		DataPtr                              *Data
		ParamSet                             ParamSet
		ParamSetIgnoreNil                    ParamSetIgnoreNil `go2ts:"ignorenil"`
		MapString                            map[string]string
		MapStringIgnoreNil                   map[string]string `go2ts:"ignorenil"`
		MapStringSlice                       map[string][]string
		MapStringSliceIgnoreNil              map[string][]string `go2ts:"ignorenil"`
		MapStringSliceSlice                  map[string][][]string
		MapStringSliceSliceIgnoreNil         map[string][][]string `go2ts:"ignorenil"`
		MapStringPtrSlice                    map[string][]*string
		MapStringPtrSliceIgnoreNil           map[string][]*string `go2ts:"ignorenil"`
		MapIntKeys                           map[int]string
		MapStringAliasKeys                   map[Mode]string
		MapIntAliasKeys                      map[Offset]string
		MapOtherStruct                       map[string]OtherStruct
		MapOtherStructPtr                    map[string]*OtherStruct
		Slice                                []string
		SliceIgnoreNil                       []string `go2ts:"ignorenil"`
		SliceOfSlice                         [][]string
		SliceOfSliceIgnoreNil                [][]string `go2ts:"ignorenil"`
		SliceOfData                          []Data
		MapOfData                            map[string]Data
		MapOfSliceOfData                     map[string][]Data
		MapOfMapOfSliceOfData                map[string]map[string][]Data
		Mode                                 Mode
		InlineStruct                         struct{ A int }
		Array                                [3]string
		skipped                              bool
		Offset                               Offset
		Color                                color.Alpha
		Direction                            Direction
		AppleVariety                         AppleVariety
		OrangeVariety                        OrangeVariety
		AppleOrchard                         AppleOrchard
		NotSerialized                        string `json:"-"`
	}

	const complexStructExpected = `// DO NOT EDIT. This file is automatically generated.

export namespace apple {
	export interface Farm {
		Name: string;
		Stats: apple.YearlyYield;
	}
}

export namespace apple {
	export interface Orchard {
		Farm: apple.Farm;
		Variety: apple.Variety;
	}
}

export interface OtherStruct {
	t?: string;
}

export interface StructWithEmbeddedStruct {
	Field: string;
	InnermostField: string;
}

export interface StructWithEmbeddedStructPtr {
	Field: string;
	InnermostField?: string;
}

export interface MultilevelEmbeddedStruct {
	OutermostField: string;
	Field?: string;
	InnermostField?: string;
	AnotherInnermostField: string;
}

export interface StructWithOverlappingEmbeddedStructs {
	InnermostField: number;
	AnotherInnermostField: number;
}

export interface RecursiveStruct {
	Field: string;
	Recursion: RecursiveStruct | null;
	RecursionIgnoreNil: RecursiveStruct;
	RecursionSlice: RecursiveStruct[] | null;
	RecursionSliceIgnoreNil: RecursiveStruct[];
}

export interface Anonymous1 {
	A: number;
}

export interface Alpha {
	A: number;
}

export interface ComplexStruct {
	String: string;
	s: string;
	Bool: boolean;
	Int: number;
	Float64: number;
	Time: string;
	Other: OtherStruct;
	OtherPtr: OtherStruct | null;
	WithEmbeddedStruct: StructWithEmbeddedStruct;
	WithEmbeddedStructPtr: StructWithEmbeddedStructPtr;
	MultilevelEmbedded: MultilevelEmbeddedStruct;
	StructWithOverlappingEmbeddedStructs: StructWithOverlappingEmbeddedStructs;
	RecursiveStruct: RecursiveStruct;
	OptionalString?: string;
	OptionalInt?: number;
	OptionalFloat64?: number;
	OptionalTime?: string;
	OptionalOtherStruct?: OtherStruct;
	OptionalOtherStructPtr?: OtherStruct | null;
	OptionalOtherStructPtrIgnoreNil?: OtherStruct;
	Data: Data;
	DataPtr: Data | null;
	ParamSet: ParamSet;
	ParamSetIgnoreNil: ParamSetIgnoreNil;
	MapString: { [key: string]: string } | null;
	MapStringIgnoreNil: { [key: string]: string };
	MapStringSlice: { [key: string]: string[] | null } | null;
	MapStringSliceIgnoreNil: { [key: string]: string[] };
	MapStringSliceSlice: { [key: string]: (string[] | null)[] | null } | null;
	MapStringSliceSliceIgnoreNil: { [key: string]: string[][] };
	MapStringPtrSlice: { [key: string]: (string | null)[] | null } | null;
	MapStringPtrSliceIgnoreNil: { [key: string]: string[] };
	MapIntKeys: { [key: number]: string } | null;
	MapStringAliasKeys: { [key: string]: string } | null;
	MapIntAliasKeys: { [key: number]: string } | null;
	MapOtherStruct: { [key: string]: OtherStruct } | null;
	MapOtherStructPtr: { [key: string]: OtherStruct | null } | null;
	Slice: string[] | null;
	SliceIgnoreNil: string[];
	SliceOfSlice: (string[] | null)[] | null;
	SliceOfSliceIgnoreNil: string[][];
	SliceOfData: Data[] | null;
	MapOfData: { [key: string]: Data } | null;
	MapOfSliceOfData: { [key: string]: Data[] | null } | null;
	MapOfMapOfSliceOfData: { [key: string]: { [key: string]: Data[] | null } | null } | null;
	Mode: Mode;
	InlineStruct: Anonymous1;
	Array: string[];
	Offset: Offset;
	Color: Alpha;
	Direction: Direction;
	AppleVariety: apple.Variety;
	OrangeVariety: orange.Variety;
	AppleOrchard: apple.Orchard;
}

export namespace apple { export type YearlyYield = { [key: number]: number } | null; }

export namespace apple { export type Variety = 'honeycrisp' | 'gala'; }

export type Data = { [key: string]: any } | null;

export type ParamSet = { [key: string]: string[] | null } | null;

export type ParamSetIgnoreNil = { [key: string]: string[] };

export type Mode = string;

export type Offset = number;

export type Direction = 'up' | 'down' | 'left' | 'right';

export namespace orange { export type Variety = 'bergamot' | 'clementine'; }
`

	go2ts := New()
	go2ts.AddWithNameToNamespace(AppleOrchard{}, "Orchard", "apple")
	go2ts.Add(ComplexStruct{})
	go2ts.AddUnion(AllDirections)
	go2ts.AddUnion(AllDirections)
	go2ts.AddUnionWithNameToNamespace(AllAppleVarieties, "Variety", "apple")
	go2ts.AddUnionWithNameToNamespace(AllOrangeVarieties, "Variety", "orange")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)

	assert.Equal(t, complexStructExpected, b.String())
}

func TestRender_NoTypesAdded_ReturnsEmptyString(t *testing.T) {
	go2ts := New()
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	assert.Equal(t, "// DO NOT EDIT. This file is automatically generated.\n", b.String())
}

func TestRender_SameTypeAddedInMultipleWays_RendersTypeOnce(t *testing.T) {
	type SomeStruct struct {
		B string
	}

	go2ts := New()
	go2ts.Add(reflect.TypeOf(SomeStruct{}))
	go2ts.AddWithName(SomeStruct{}, "ADifferentName")
	go2ts.Add(reflect.TypeOf([]SomeStruct{}).Elem())
	go2ts.Add(SomeStruct{})
	go2ts.Add(&SomeStruct{})
	go2ts.Add(reflect.New(reflect.TypeOf(SomeStruct{})))
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export interface SomeStruct {
	B: string;
}
`
	assert.Equal(t, expected, b.String())
}

func TestRender_FirstAddDeterminesInterfaceName(t *testing.T) {
	type SomeStruct struct {
		B string
	}

	go2ts := New()
	go2ts.AddWithName(SomeStruct{}, "ADifferentName")
	go2ts.Add(reflect.TypeOf(SomeStruct{}))
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export interface ADifferentName {
	B: string;
}
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddMultiple_Success(t *testing.T) {
	type SomeStruct struct {
		B string
	}

	type AnotherStruct struct {
		A string
	}

	go2ts := New()
	go2ts.AddMultiple(SomeStruct{}, AnotherStruct{})
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export interface SomeStruct {
	B: string;
}

export interface AnotherStruct {
	A: string;
}
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddMultipleEmpty_Success(t *testing.T) {
	go2ts := New()
	go2ts.AddMultiple()
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddMultipleUnion_Success(t *testing.T) {
	type DayOfWeek string

	const (
		Monday  DayOfWeek = "Mon"
		Tuesday DayOfWeek = "Tue"
	)
	daysOfWeek := []DayOfWeek{Monday, Tuesday}

	type MonthsOfYear string

	const (
		January  MonthsOfYear = "Jan"
		February MonthsOfYear = "Feb"
		March    MonthsOfYear = "Mar"
	)
	monthsOfYear := []MonthsOfYear{January, February, March}

	go2ts := New()
	go2ts.AddMultipleUnion(daysOfWeek, monthsOfYear)
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type DayOfWeek = 'Mon' | 'Tue';

export type MonthsOfYear = 'Jan' | 'Feb' | 'Mar';
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddMultipleUnionEmpty_Success(t *testing.T) {
	go2ts := New()
	go2ts.AddMultipleUnion()
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.
`
	assert.Equal(t, expected, b.String())
}

func TestRender_Add_NonStructType_Success(t *testing.T) {
	type Data map[string][]string

	go2ts := New()
	go2ts.Add(Data{})
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type Data = { [key: string]: string[] | null } | null;
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddIgnoreNil_NonStructType_Success(t *testing.T) {
	type Data map[string][]string

	go2ts := New()
	go2ts.AddIgnoreNil(Data{})
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type Data = { [key: string]: string[] };
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddWithName_NonStructType_Success(t *testing.T) {
	type Data map[string][]string

	go2ts := New()
	go2ts.AddWithName(Data{}, "SomeNewName")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type SomeNewName = { [key: string]: string[] | null } | null;
`
	assert.Equal(t, expected, b.String())
}

func TestRender_AddWithNameIgnoreNil_NonStructType_Success(t *testing.T) {
	type Data map[string][]string

	go2ts := New()
	go2ts.AddWithNameIgnoreNil(Data{}, "SomeNewName")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type SomeNewName = { [key: string]: string[] };
`
	assert.Equal(t, expected, b.String())
}

func TestAdd_UnsupportedType_Panic(t *testing.T) {
	type HasUnsupportedFieldTypes struct {
		C complex128
	}

	go2ts := New()
	assert.PanicsWithValue(t, `Go Kind "complex128" cannot be serialized to JSON.`, func() {
		go2ts.Add(HasUnsupportedFieldTypes{})
	})
}

func TestAddUnionWithName_SliceOfString_Success(t *testing.T) {
	type DayOfWeek string

	const (
		Monday  DayOfWeek = "Mon"
		Tuesday DayOfWeek = "Tue"
	)

	go2ts := New()
	go2ts.AddUnionWithName([]DayOfWeek{Monday, Tuesday}, "")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type DayOfWeek = 'Mon' | 'Tue';
`
	assert.Equal(t, expected, b.String())
}

func TestAddUnion_SliceOfString_Success(t *testing.T) {
	type DayOfWeek string

	const (
		Monday  DayOfWeek = "Mon"
		Tuesday DayOfWeek = "Tue"
	)

	go2ts := New()
	go2ts.AddUnion([]DayOfWeek{Monday, Tuesday})
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type DayOfWeek = 'Mon' | 'Tue';
`
	assert.Equal(t, expected, b.String())
}

func TestAddUnionWithName_SliceOfStringWithName_Success(t *testing.T) {
	type DayOfWeek string

	const (
		Monday  DayOfWeek = "Mon"
		Tuesday DayOfWeek = "Tue"
	)

	go2ts := New()
	go2ts.AddUnionWithName([]DayOfWeek{Monday, Tuesday}, "ShouldBeTheTypeName")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type ShouldBeTheTypeName = 'Mon' | 'Tue';
`
	assert.Equal(t, expected, b.String())
}

func TestAddUnionWithName_ArrayOfInt_Success(t *testing.T) {
	type SomeOption int

	const (
		OptionA SomeOption = 1
		OptionB SomeOption = 3
	)

	go2ts := New()
	go2ts.AddUnionWithName([2]SomeOption{OptionA, OptionB}, "")
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export type SomeOption = 1 | 3;
`
	assert.Equal(t, expected, b.String())
}

func TestAddUnionWithName_NotSliceOrArray_Panic(t *testing.T) {
	type SomeOption int

	go2ts := New()
	assert.PanicsWithValue(t, `AddUnionWithName must be supplied an array or slice, got int: 1`, func() {
		go2ts.AddUnionWithName(SomeOption(1), "")
	})
}

func TestAddUnion_DefinitionFoundFromStructAndUnion_UnionTypeDefinitionIsEmitted(t *testing.T) {
	type SomeOption int

	const (
		OptionA SomeOption = 1
		OptionB SomeOption = 3
	)

	type SomeStruct struct {
		Choices SomeOption
	}

	go2ts := New()
	go2ts.Add(SomeStruct{})
	go2ts.AddUnion([2]SomeOption{OptionA, OptionB})
	var b bytes.Buffer
	err := go2ts.Render(&b)
	require.NoError(t, err)
	expected := `// DO NOT EDIT. This file is automatically generated.

export interface SomeStruct {
	Choices: SomeOption;
}

export type SomeOption = 1 | 3;
`
	assert.Equal(t, expected, b.String())
}
