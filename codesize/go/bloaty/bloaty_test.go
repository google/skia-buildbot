package bloaty

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBloatyOutput_EmptyBloatyOutput_Error(t *testing.T) {
	_, err := ParseTSVOutput("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty input")
}

func TestParseBloatyOutput_NotTSV_Error(t *testing.T) {

	bloatyOutput := `compileunits,symbols,vmsize,filesize
../../dm/DMSrcSink.cpp,DM::CodecSrc::draw(),7307,7382
`

	_, err := ParseTSVOutput(bloatyOutput)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on line 1: unrecognized header format; must be: \"compileunits\\tsymbols\\tvmsize\\tfilesize\"")
}

func TestParseBloatyOutput_WrongColumns_Error(t *testing.T) {

	bloatyOutput := `compileunits	filesize
../../dm/DMSrcSink.cpp	7382
`

	_, err := ParseTSVOutput(bloatyOutput)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on line 1: unrecognized header format; must be: \"compileunits\\tsymbols\\tvmsize\\tfilesize\"")
}

// sampleBloatyOutput is a valid Bloaty output that exercises the following logic:
//
//   - Enforcement of unique symbol names (see the repeated symbols).
//   - Special handling of third_party paths (see the third_party compile units).
var sampleBloatyOutput = `compileunits	symbols	vmsize	filesize
../../third_party/externals/harfbuzz/src/hb-ot-font.cc	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	13
../../third_party/externals/harfbuzz/src/hb-subset.cc	[section .debug_info]	0	4213071
../../dm/DMSrcSink.cpp	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	14
../../dm/DMSrcSink.cpp	DM::CodecSrc::draw()	7307	7382
../../src/sksl/SkSLCompiler.cpp	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	16
../../../../../../skia/third_party/externals/libwebp/src/enc/vp8l_enc.c	EncodeStreamHook	7656	7656
[section .rodata]	[section .rodata]	10425940	10425940
[section .rodata]	propsVectorsTrie_index	62456	62456
[section .text]	png_create_read_struct_2	75	75
[section .eh_frame]	png_create_read_struct_2	32	32
[section .eh_frame_hdr]	png_create_read_struct_2	8	8
`

func TestParseBloatyOutput_Success(t *testing.T) {

	items, err := ParseTSVOutput(sampleBloatyOutput)
	require.NoError(t, err)

	assert.Equal(t, []OutputItem{
		{
			CompileUnit:       "third_party/externals/harfbuzz/src/hb-ot-font.cc",
			Symbol:            "(anonymous namespace)::TLSCurrentObjects::Get()::objects",
			VirtualMemorySize: 0,
			FileSize:          13,
		},
		{
			CompileUnit:       "third_party/externals/harfbuzz/src/hb-subset.cc",
			Symbol:            "[section .debug_info]",
			VirtualMemorySize: 0,
			FileSize:          4213071,
		},
		{
			CompileUnit:       "dm/DMSrcSink.cpp",
			Symbol:            "(anonymous namespace)::TLSCurrentObjects::Get()::objects",
			VirtualMemorySize: 0,
			FileSize:          14,
		},
		{
			CompileUnit:       "dm/DMSrcSink.cpp",
			Symbol:            "DM::CodecSrc::draw()",
			VirtualMemorySize: 7307,
			FileSize:          7382,
		},
		{
			CompileUnit:       "src/sksl/SkSLCompiler.cpp",
			Symbol:            "(anonymous namespace)::TLSCurrentObjects::Get()::objects",
			VirtualMemorySize: 0,
			FileSize:          16,
		},
		{
			CompileUnit:       "third_party/externals/libwebp/src/enc/vp8l_enc.c",
			Symbol:            "EncodeStreamHook",
			VirtualMemorySize: 7656,
			FileSize:          7656,
		},
		{
			CompileUnit:       "[section .rodata]",
			Symbol:            "UNKNOWN [section .rodata]",
			VirtualMemorySize: 10425940,
			FileSize:          10425940,
		},
		{
			CompileUnit:       "[section .rodata]",
			Symbol:            "propsVectorsTrie_index [section .rodata]",
			VirtualMemorySize: 62456,
			FileSize:          62456,
		},
		{
			CompileUnit:       "[section .text]",
			Symbol:            "png_create_read_struct_2 [section .text]",
			VirtualMemorySize: 75,
			FileSize:          75,
		},
		{
			CompileUnit:       "[section .eh_frame]",
			Symbol:            "png_create_read_struct_2 [section .eh_frame]",
			VirtualMemorySize: 32,
			FileSize:          32,
		},
		{
			CompileUnit:       "[section .eh_frame_hdr]",
			Symbol:            "png_create_read_struct_2 [section .eh_frame_hdr]",
			VirtualMemorySize: 8,
			FileSize:          8,
		},
	},
		items)
}

func TestGenTreeMapDataTable_AllRowsCreatedInSpecifiedOrder(t *testing.T) {

	items, err := ParseTSVOutput(sampleBloatyOutput)
	require.NoError(t, err)

	rows := GenTreeMapDataTableRows(items, 200)

	assert.Equal(t, []TreeMapDataTableRow{
		{
			Name:   "ROOT",
			Parent: "",
			Size:   0,
		},
		{
			Name:   "[section .eh_frame]",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Parent: "[section .eh_frame]",
			Name:   "png_create_read_struct_2 [section .eh_frame]",
			Size:   32,
		},
		{
			Name:   "[section .eh_frame_hdr]",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Parent: "[section .eh_frame_hdr]",
			Name:   "png_create_read_struct_2 [section .eh_frame_hdr]",
			Size:   8,
		},
		{
			Name:   "[section .rodata]",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "UNKNOWN [section .rodata]",
			Parent: "[section .rodata]",
			Size:   10425940,
		},
		{
			Parent: "[section .rodata]",
			Name:   "propsVectorsTrie_index [section .rodata]",
			Size:   62456,
		},
		{
			Name:   "[section .text]",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Parent: "[section .text]",
			Name:   "png_create_read_struct_2 [section .text]",
			Size:   75,
		},
		{
			Name:   "dm",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "dm/DMSrcSink.cpp",
			Parent: "dm",
			Size:   0,
		},
		{
			Name:   "DM::CodecSrc::draw()",
			Parent: "dm/DMSrcSink.cpp",
			Size:   7382,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects",
			Parent: "dm/DMSrcSink.cpp",
			Size:   14,
		},
		{
			Name:   "src",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "src/sksl",
			Parent: "src",
			Size:   0,
		},
		{
			Name:   "src/sksl/SkSLCompiler.cpp",
			Parent: "src/sksl",
			Size:   0,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects_1",
			Parent: "src/sksl/SkSLCompiler.cpp",
			Size:   16,
		},
		{
			Name:   "third_party",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "third_party/externals",
			Parent: "third_party",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz",
			Parent: "third_party/externals",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz/src",
			Parent: "third_party/externals/harfbuzz",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz/src/hb-ot-font.cc",
			Parent: "third_party/externals/harfbuzz/src",
			Size:   0,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects_2",
			Parent: "third_party/externals/harfbuzz/src/hb-ot-font.cc",
			Size:   13,
		},
		{
			Name:   "third_party/externals/harfbuzz/src/hb-subset.cc",
			Parent: "third_party/externals/harfbuzz/src",
			Size:   0,
		},
		{
			Name:   "[section .debug_info]",
			Parent: "third_party/externals/harfbuzz/src/hb-subset.cc",
			Size:   4213071,
		},
		{
			Name:   "third_party/externals/libwebp",
			Parent: "third_party/externals",
			Size:   0,
		},
		{
			Name:   "third_party/externals/libwebp/src",
			Parent: "third_party/externals/libwebp",
			Size:   0,
		},
		{
			Name:   "third_party/externals/libwebp/src/enc",
			Parent: "third_party/externals/libwebp/src",
			Size:   0,
		},
		{
			Name:   "third_party/externals/libwebp/src/enc/vp8l_enc.c",
			Parent: "third_party/externals/libwebp/src/enc",
			Size:   0,
		},
		{
			Name:   "EncodeStreamHook",
			Parent: "third_party/externals/libwebp/src/enc/vp8l_enc.c",
			Size:   7656,
		},
	},
		rows)
}

func TestGenTreeMapDataTable_GroupsSymbolsBeyondCutoff(t *testing.T) {

	// Create 10 alpha items (of which the biggest 4 will be shown) and 2 beta items (all of which
	// will be shown).
	var items []OutputItem
	for i := 1; i <= 10; i++ {
		items = append(items, OutputItem{
			CompileUnit: "alpha",
			Symbol:      "a" + strconv.Itoa(i),
			FileSize:    i,
		})
	}
	for i := 1; i <= 2; i++ {
		items = append(items, OutputItem{
			CompileUnit: "beta",
			Symbol:      "b" + strconv.Itoa(i),
			FileSize:    i,
		})
	}

	rows := GenTreeMapDataTableRows(items, 4)

	assert.Equal(t, []TreeMapDataTableRow{
		{
			Name:   "ROOT",
			Parent: "",
			Size:   0,
		},
		{
			Name:   "alpha",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "a10",
			Parent: "alpha",
			Size:   10,
		},
		{
			Name:   "a9",
			Parent: "alpha",
			Size:   9,
		},
		{
			Name:   "a8",
			Parent: "alpha",
			Size:   8,
		},
		{
			Name:   "a7",
			Parent: "alpha",
			Size:   7,
		},
		{
			Name:   "beta",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "b2",
			Parent: "beta",
			Size:   2,
		},
		{
			Name:   "b1",
			Parent: "beta",
			Size:   1,
		},
		// Overflow comes at the end
		{
			Name:   "remainder (alpha)",
			Parent: "alpha",
			Size:   6 + 5 + 4 + 3 + 2 + 1,
		},
	}, rows)
}
