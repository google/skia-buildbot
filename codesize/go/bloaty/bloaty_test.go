package bloaty

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseBloatyOutput_EmptyBloatyOutput_Error(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseTSVOutput("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty input")
}

func TestParseBloatyOutput_NotTSV_Error(t *testing.T) {
	unittest.SmallTest(t)

	bloatyOutput := `compileunits,symbols,vmsize,filesize
../../dm/DMSrcSink.cpp,DM::CodecSrc::draw(),7307,7382
`

	_, err := ParseTSVOutput(bloatyOutput)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on line 1: unrecognized header format; must be: \"compileunits\\tsymbols\\tvmsize\\tfilesize\"")
}

func TestParseBloatyOutput_WrongColumns_Error(t *testing.T) {
	unittest.SmallTest(t)

	bloatyOutput := `compileunits	filesize
../../dm/DMSrcSink.cpp	7382
`

	_, err := ParseTSVOutput(bloatyOutput)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on line 1: unrecognized header format; must be: \"compileunits\\tsymbols\\tvmsize\\tfilesize\"")
}

// sampleBloatyOutput is a valid Bloaty output that exercises the following logic:
//
//  - Enforcement of unique symbol names (see the repeated symbols).
//  - Special handling of third_party paths (see the third_party compile units).
var sampleBloatyOutput = `compileunits	symbols	vmsize	filesize
../../third_party/externals/harfbuzz/src/hb-ot-font.cc	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	13
../../third_party/externals/harfbuzz/src/hb-subset.cc	[section .debug_info]	0	4213071
../../dm/DMSrcSink.cpp	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	14
../../dm/DMSrcSink.cpp	DM::CodecSrc::draw()	7307	7382
../../src/sksl/SkSLCompiler.cpp	(anonymous namespace)::TLSCurrentObjects::Get()::objects	0	16
`

func TestParseBloatyOutput_Success(t *testing.T) {
	unittest.SmallTest(t)

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
	},
		items)
}

func TestGenTreeMapDataTable_Success(t *testing.T) {
	unittest.SmallTest(t)

	items, err := ParseTSVOutput(sampleBloatyOutput)
	require.NoError(t, err)

	rows := GenTreeMapDataTableRows(items)

	assert.Equal(t, []TreeMapDataTableRow{
		{
			Name:   "ROOT",
			Parent: "",
			Size:   0,
		},
		{
			Name:   "third_party (Path)",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "third_party/externals (Path)",
			Parent: "third_party (Path)",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz (Path)",
			Parent: "third_party/externals (Path)",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz/src (Path)",
			Parent: "third_party/externals/harfbuzz (Path)",
			Size:   0,
		},
		{
			Name:   "third_party/externals/harfbuzz/src/hb-ot-font.cc (Path)",
			Parent: "third_party/externals/harfbuzz/src (Path)",
			Size:   0,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects",
			Parent: "third_party/externals/harfbuzz/src/hb-ot-font.cc (Path)",
			Size:   13,
		},
		{
			Name:   "dm (Path)",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "dm/DMSrcSink.cpp (Path)",
			Parent: "dm (Path)",
			Size:   0,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects_1",
			Parent: "dm/DMSrcSink.cpp (Path)",
			Size:   14,
		},
		{
			Name:   "DM::CodecSrc::draw()",
			Parent: "dm/DMSrcSink.cpp (Path)",
			Size:   7382,
		},
		{
			Name:   "src (Path)",
			Parent: "ROOT",
			Size:   0,
		},
		{
			Name:   "src/sksl (Path)",
			Parent: "src (Path)",
			Size:   0,
		},
		{
			Name:   "src/sksl/SkSLCompiler.cpp (Path)",
			Parent: "src/sksl (Path)",
			Size:   0,
		},
		{
			Name:   "(anonymous namespace)::TLSCurrentObjects::Get()::objects_2",
			Parent: "src/sksl/SkSLCompiler.cpp (Path)",
			Size:   16,
		},
	},
		rows)
}
