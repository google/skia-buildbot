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
/mnt/pd0/s/w/ir/skia/third_party/externals/zlib/cpu_features.c	Cr_z_cpu_check_features	1567	2994
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
			CompileUnit:       "third_party/externals/zlib/cpu_features.c",
			Symbol:            "Cr_z_cpu_check_features",
			VirtualMemorySize: 1567,
			FileSize:          2994,
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

func TestParseTSVOutput_HandlesAndroidNDKFiles(t *testing.T) {
	// If a symbol is a "section", it should be omitted. We should also clean up the file paths

	const testData = `compileunits	symbols	vmsize	filesize
/buildbot/src/android/ndk-release-r21/external/libcxx/src/locale.cpp	std::__ndk1::num_put<>::do_put()	6504	6504
/buildbot/src/android/ndk-release-r21/external/libcxx/../../external/libunwind_llvm/src/UnwindRegistersSave.S	[section .text]	48	48
/buildbot/src/android/ndk-release-r21/external/libcxx/../../external/libunwind_llvm/src/UnwindRegistersSave.S	[section .dynsym]	16	16
/buildbot/src/android/ndk-release-r21/external/libcxx/../../external/libunwind_llvm/src/UnwindRegistersSave.S	[section .dynstr]	15	15
/mnt/pd0/s/w/ir/skia/third_party/externals/dng_sdk/source/dng_abort_sniffer.cpp	_GLOBAL__sub_I_dng_abort_sniffer.cpp	43	43
/mnt/pd0/s/w/ir/skia/third_party/externals/dng_sdk/source/dng_abort_sniffer.cpp	dng_abort_sniffer::SniffForAbort()	35	35
/mnt/pd0/s/w/ir/skia/third_party/externals/dng_sdk/source/dng_abort_sniffer.cpp	[section .text]	1	1
/mnt/pd0/s/w/ir/skia/third_party/externals/harfbuzz/src/hb-ot-layout.cc	OT::HeadlessArrayOf<>::operator[]()	84	84
../../../../../../skia/src/sksl/ir/SkSLType.cpp	std::__ndk1::unique_ptr<>::~unique_ptr()	9	9
`

	rv, err := ParseTSVOutput(testData)
	require.NoError(t, err)
	assert.Equal(t, []OutputItem{
		{
			CompileUnit:       "ndk-release-r21/external/libcxx/src/locale.cpp",
			Symbol:            "std::__ndk1::num_put<>::do_put()",
			VirtualMemorySize: 6504,
			FileSize:          6504,
		},
		{
			CompileUnit:       "third_party/externals/dng_sdk/source/dng_abort_sniffer.cpp",
			Symbol:            "_GLOBAL__sub_I_dng_abort_sniffer.cpp",
			VirtualMemorySize: 43,
			FileSize:          43,
		},
		{
			CompileUnit:       "third_party/externals/dng_sdk/source/dng_abort_sniffer.cpp",
			Symbol:            "dng_abort_sniffer::SniffForAbort()",
			VirtualMemorySize: 35,
			FileSize:          35,
		},
		{
			CompileUnit:       "third_party/externals/harfbuzz/src/hb-ot-layout.cc",
			Symbol:            "OT::HeadlessArrayOf<>::operator[]()",
			VirtualMemorySize: 84,
			FileSize:          84,
		},
		{
			CompileUnit:       "skia/src/sksl/ir/SkSLType.cpp",
			Symbol:            "std::__ndk1::unique_ptr<>::~unique_ptr()",
			VirtualMemorySize: 9,
			FileSize:          9,
		},
	}, rv)
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
		{
			Name:   "third_party/externals/zlib",
			Parent: "third_party/externals",
			Size:   0,
		},
		{
			Name:   "third_party/externals/zlib/cpu_features.c",
			Parent: "third_party/externals/zlib",
			Size:   0,
		},
		{
			Name:   "Cr_z_cpu_check_features",
			Parent: "third_party/externals/zlib/cpu_features.c",
			Size:   2994,
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
