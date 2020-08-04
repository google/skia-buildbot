// Package data_kitchen_sink demonstrates all of the data that Gold stores. It strives to
// encompass as much of the scenarios we see in real world data as possible, while being small
// and organized enough to be comprehensible.
//
// The scenario here is that there are three tests (circle, square, triangle) that produce an image
// of the given shape. These are divided into two corpora (round and corners). At the beginning of
// the data timeline (which is 10 commits long) these tests are run on a Windows 10.2 machine and
// two iOS devices ("iPad6,3" and "iPhone12,1").
// On each of these devices, the tests are run in RGB mode and GREY mode, producing outputs that
// are in color or greyscale.
//
// This means we start with 18 traces (3 tests * 3 devices * 2 color_mode). The addition of one
// device and the os upgrade means there are 30 traces seen over the last 10 commits. See
// MakeTraces for some specific comments on any of the given traces.
//
// At specific commits the following "interesting" things happen:
//  - At commit index 3, the Windows 10.2 device is upgraded to 10.3. This causes a slight change in
//    the circle tests, producing untriaged output.
//  - At commit index 5, a new device (Android walleye) is added. draws correctly, except it is very
//    flaky for the square test in RGB mode.
//  - At commit index 7, a change fixes the triangle tests on iOS but breaks the circle tests.
//  - At commit index 8, the optional params for autotriage are added to walleye's square rgb test.
//  - At commit index 9, the Windows 10.3 tests with the GREY mode have not completed yet.
//
// There are two CLs of note: one that attempts to fix the iOS devices (and partially succeeds), and
// one that adds some new tests. The iOS one covers all possibilities of a digest being triaged/not
// and the same as one on master branch and not. The new test CL not only adds a new corpus and
// a new test on an existing corpus, but has data coming in from an internal CRS and CIS.
//
// Future growth: When Gold is ready to have a more generic "grouping" for traces, test name +
// color_mode is a natural split here.
package data_kitchen_sink

import (
	"time"

	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

func MakeTraces() []tiling.TracePair {
	return []tiling.TracePair{
		// ============= Windows 10.2 traces =============
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace is a little non-deterministic - it sometimes outputs one of two digests
			ID: ",color mode=GREY,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestA02Pos, DigestA02Pos, DigestA03Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=triangle,os=Windows10.2,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestB01Pos, DigestB01Pos, DigestB01Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=QuadroP400,name=triangle,os=Windows10.2,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestB02Pos, DigestB02Pos, DigestB02Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=circle,os=Windows10.2,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestC01Pos, DigestC01Pos, DigestC01Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=QuadroP400,name=circle,os=Windows10.2,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestC02Pos, DigestC02Pos, DigestC02Pos, DigestNoData, DigestNoData,
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot2OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		// ============= Windows 10.3 traces =============
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.3,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestA01Pos, DigestA01Pos,
					DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace is a little non-deterministic - it sometimes outputs one of two digests
			ID: ",color mode=GREY,device=QuadroP400,name=square,os=Windows10.3,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestA02Pos, DigestA03Pos,
					DigestA02Pos, DigestA02Pos, DigestA03Pos, DigestA02Pos, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=triangle,os=Windows10.3,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestB01Pos, DigestB01Pos,
					DigestB01Pos, DigestB01Pos, DigestB01Pos, DigestB01Pos, DigestB01Pos,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=QuadroP400,name=triangle,os=Windows10.3,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestB02Pos, DigestB02Pos,
					DigestB02Pos, DigestB02Pos, DigestB02Pos, DigestB02Pos, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=circle,os=Windows10.3,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestC03Unt, DigestC03Unt,
					DigestC03Unt, DigestC03Unt, DigestC03Unt, DigestC03Unt, DigestC03Unt,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=QuadroP400,name=circle,os=Windows10.3,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestC04Unt, DigestC04Unt,
					DigestC04Unt, DigestC04Unt, DigestC04Unt, DigestC04Unt, DigestNoData,
				},
				map[string]string{
					OSKey:                 Windows10dot3OS,
					DeviceKey:             QuadroDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		// ============= ipad traces =============
		{
			ID: ",color mode=RGB,device=iPad6_3,name=square,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos,
					DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos, DigestA01Pos,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace is a little non-deterministic - it sometimes outputs one of three digests,
			// two of which have been triaged.
			ID: ",color mode=GREY,device=iPad6_3,name=square,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestA02Pos, DigestA03Pos, DigestA02Pos, DigestA03Pos, DigestA02Pos,
					DigestA02Pos, DigestA02Pos, DigestA02Pos, DigestA04Unt, DigestA03Pos,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing incorrectly until commit index 7.
			ID: ",color mode=RGB,device=iPad6_3,name=triangle,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestB03Neg, DigestB03Neg, DigestBlank, DigestB03Neg, DigestB03Neg,
					DigestBlank, DigestB03Neg, DigestB01Pos, DigestB01Pos, DigestB01Pos,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing incorrectly until commit index 7.
			ID: ",color mode=GREY,device=iPad6_3,name=triangle,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestB04Neg, DigestBlank, DigestB04Neg, DigestBlank, DigestB04Neg,
					DigestB04Neg, DigestB04Neg, DigestB02Pos, DigestB02Pos, DigestB02Pos,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing correctly until commit index 7.
			ID: ",color mode=RGB,device=iPad6_3,name=circle,os=iOS,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestC01Pos, DigestC01Pos, DigestC01Pos, DigestC01Pos, DigestC01Pos,
					DigestC01Pos, DigestC01Pos, DigestC05Unt, DigestC05Unt, DigestC05Unt,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing correctly until commit index 7.
			ID: ",color mode=GREY,device=iPad6_3,name=circle,os=iOS,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestC02Pos, DigestC02Pos, DigestC02Pos, DigestC02Pos, DigestC02Pos,
					DigestC02Pos, DigestC02Pos, DigestC05Unt, DigestC05Unt, DigestC05Unt,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPadDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		// ============= iPhone traces =============
		// Of note, we pretend the iPhone tests are slow and therefore have sparse data.
		// We do this by saying the RGB data is missing on every other commit and the GREY data is
		// missing on two commits out of three.
		{
			ID: ",color mode=RGB,device=iPhone12_1,name=square,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestA01Pos, DigestNoData, DigestA01Pos, DigestNoData, DigestA01Pos,
					DigestNoData, DigestA01Pos, DigestNoData, DigestA01Pos, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=iPhone12_1,name=square,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestA02Pos, DigestNoData, DigestNoData, DigestA02Pos,
					DigestNoData, DigestNoData, DigestA02Pos, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing incorrectly until commit index 7 (either blank or incorrect)
			ID: ",color mode=RGB,device=iPhone12_1,name=triangle,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestB03Neg, DigestNoData, DigestBlank, DigestNoData, DigestBlank,
					DigestNoData, DigestB03Neg, DigestNoData, DigestB01Pos, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing incorrectly until commit index 7.
			ID: ",color mode=GREY,device=iPhone12_1,name=triangle,os=iOS,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestBlank, DigestNoData, DigestNoData, DigestB04Neg,
					DigestNoData, DigestNoData, DigestB02Pos, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing correctly until commit index 7.
			ID: ",color mode=RGB,device=iPhone12_1,name=circle,os=iOS,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestC01Pos, DigestNoData, DigestC01Pos, DigestNoData, DigestC01Pos,
					DigestNoData, DigestC01Pos, DigestNoData, DigestC05Unt, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{ // This trace was drawing correctly until commit index 7.
			ID: ",color mode=GREY,device=iPhone12_1,name=circle,os=iOS,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestC02Pos, DigestNoData, DigestNoData, DigestC02Pos,
					DigestNoData, DigestNoData, DigestC05Unt, DigestNoData, DigestNoData,
				},
				map[string]string{
					OSKey:                 iOS,
					DeviceKey:             IPhoneDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		//  ============= walleye traces =============
		// This device doesn't exist before commit index 5. The Grey config is currently streaming
		// data in at commit index 7, so some traces are missing data there.
		{ // this trace was really flaky, so starting at index 8 it was configured to use fuzzy
			// matching.
			ID: ",color mode=RGB,device=walleye,name=square,os=Android,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestA05Unt, DigestA01Pos, DigestA06Unt, DigestA07Pos, DigestA08Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption:              PNGExtension,
					"image_matching_algorithm":   "fuzzy",
					"fuzzy_max_different_pixels": "2",
				}),
		},
		{
			ID: ",color mode=GREY,device=walleye,name=square,os=Android,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestA02Pos, DigestA02Pos, DigestA02Pos, DigestA02Pos, DigestA02Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(SquareTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=walleye,name=triangle,os=Android,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestB01Pos, DigestB01Pos, DigestB01Pos, DigestB01Pos, DigestB01Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=walleye,name=triangle,os=Android,source_type=corners,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestB02Pos, DigestB02Pos, DigestNoData, DigestB02Pos, DigestB02Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     CornersCorpus,
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=walleye,name=circle,os=Android,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestC01Pos, DigestC01Pos, DigestC01Pos, DigestC01Pos, DigestC01Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
		{
			ID: ",color mode=GREY,device=walleye,name=circle,os=Android,source_type=round,",
			Trace: tiling.NewTrace(
				[]types.Digest{
					DigestNoData, DigestNoData, DigestNoData, DigestNoData, DigestNoData,
					DigestC02Pos, DigestC02Pos, DigestNoData, DigestC02Pos, DigestC02Pos,
				},
				map[string]string{
					OSKey:                 AndroidOS,
					DeviceKey:             WalleyeDevice,
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionOption: PNGExtension,
				}),
		},
	}
}

type TryJobData struct {
	PatchSet tjstore.CombinedPSID
	CIS      string
	Keys     map[string]string
	Options  map[string]string
	Digest   types.Digest
}

func MakeDataFromTryJobs() []TryJobData {
	// Note, a real CQ run would probably have more than the data shown, but this subset of data
	// is the "interesting" part, subsetted for brevity.
	return []TryJobData{
		{ // This is a positive result that is already triaged/seen on master branch.
			PatchSet: idFixesIpad,
			CIS:      BuildBucket,
			Keys: map[string]string{
				OSKey:                 iOS,
				DeviceKey:             IPadDevice,
				types.CorpusField:     CornersCorpus,
				types.PrimaryKeyField: string(SquareTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestA01Pos,
		},
		{ // This is an untriaged result that is already seen on master branch.
			PatchSet: idFixesIpad,
			CIS:      BuildBucket,
			Keys: map[string]string{
				OSKey:                 iOS,
				DeviceKey:             IPadDevice,
				types.CorpusField:     CornersCorpus,
				types.PrimaryKeyField: string(SquareTest),
				ColorModeKey:          GreyColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestA04Unt,
		},
		{ // This is a positive result that is not seen on master branch.
			PatchSet: idFixesIpad,
			CIS:      BuildBucket,
			Keys: map[string]string{
				OSKey:                 iOS,
				DeviceKey:             IPadDevice,
				types.CorpusField:     RoundCorpus,
				types.PrimaryKeyField: string(CircleTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestC06Pos_CL,
		},
		{ // This is an untriaged digest that is not seen on master branch.
			PatchSet: idFixesIpad,
			CIS:      BuildBucket,
			Keys: map[string]string{
				OSKey:                 iOS,
				DeviceKey:             IPhoneDevice,
				types.CorpusField:     RoundCorpus,
				types.PrimaryKeyField: string(CircleTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestC07Unt_CL,
		},
		{ // Oops, this PS adds a new corpus, but it's blank.
			PatchSet: idAddsTextCorpus,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     TextCorpus,
				types.PrimaryKeyField: string(SevenTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestBlank,
		},
		{
			PatchSet: idAddsTextCorpus,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     TextCorpus,
				types.PrimaryKeyField: string(SevenTest),
				ColorModeKey:          GreyColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestBlank,
		},
		{
			PatchSet: idAddsTextCorpusAndRoundRect,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     TextCorpus,
				types.PrimaryKeyField: string(SevenTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestD01Pos_CL,
		},
		{
			PatchSet: idAddsTextCorpusAndRoundRect,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     TextCorpus,
				types.PrimaryKeyField: string(SevenTest),
				ColorModeKey:          GreyColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestD01Pos_CL,
		},
		{
			PatchSet: idAddsTextCorpusAndRoundRect,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     RoundCorpus,
				types.PrimaryKeyField: string(RoundRectTest),
				ColorModeKey:          RGBColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestE01Pos_CL,
		},
		{
			PatchSet: idAddsTextCorpusAndRoundRect,
			CIS:      BuildBucketInternal,
			Keys: map[string]string{
				OSKey:                 Windows10dot3OS,
				DeviceKey:             QuadroDevice,
				types.CorpusField:     RoundCorpus,
				types.PrimaryKeyField: string(RoundRectTest),
				ColorModeKey:          GreyColorMode,
			},
			Options: map[string]string{
				ExtensionOption: PNGExtension,
			},
			Digest: DigestE02Pos_CL,
		},
	}
}

func MakeMasterBranchExpectations() *expectations.Expectations {
	var e expectations.Expectations
	e.Set(SquareTest, DigestA01Pos, expectations.Positive)
	e.Set(SquareTest, DigestA02Pos, expectations.Positive)
	e.Set(SquareTest, DigestA03Pos, expectations.Positive)
	e.Set(SquareTest, DigestA07Pos, expectations.Positive)
	e.Set(SquareTest, DigestA08Pos, expectations.Positive)

	e.Set(TriangleTest, DigestB01Pos, expectations.Positive)
	e.Set(TriangleTest, DigestB02Pos, expectations.Positive)
	e.Set(TriangleTest, DigestB03Neg, expectations.Negative)
	e.Set(TriangleTest, DigestB04Neg, expectations.Negative)

	e.Set(CircleTest, DigestBlank, expectations.Negative)
	e.Set(CircleTest, DigestC01Pos, expectations.Positive)
	e.Set(CircleTest, DigestC02Pos, expectations.Positive)
	return &e
}

func MakeCLExpectations() map[string]*expectations.Expectations {
	var iosExpectations expectations.Expectations
	iosExpectations.Set(CircleTest, DigestC06Pos_CL, expectations.Positive)

	var newCorpusExpectations expectations.Expectations
	newCorpusExpectations.Set(SevenTest, DigestD01Pos_CL, expectations.Positive)
	newCorpusExpectations.Set(RoundRectTest, DigestE01Pos_CL, expectations.Positive)
	newCorpusExpectations.Set(RoundRectTest, DigestE02Pos_CL, expectations.Positive)

	return map[string]*expectations.Expectations{
		ChangeListIDThatAttemptsToFixIOS: &iosExpectations,
		ChangeListIDThatAddsNewTests:     &newCorpusExpectations,
	}
}

func MakeChangeLists() map[string][]code_review.ChangeList {
	return map[string][]code_review.ChangeList{
		GerritCRS: {
			{
				SystemID: "CL_was_abandoned",
				Owner:    UserTwo,
				Status:   code_review.Abandoned,
				Subject:  "Experimental Do Not Submit",
				Updated:  EighthCommitTime.Add(time.Minute),
			},
			{
				SystemID: "CL_updates_docs",
				Owner:    UserOne,
				Status:   code_review.Landed,
				Subject:  "Update Documentation",
				Updated:  NinethCommitTime.Add(time.Minute),
			},
			{
				SystemID: ChangeListIDThatAttemptsToFixIOS,
				Owner:    UserOne,
				Status:   code_review.Open,
				Subject:  "Fix iOS",
				Updated:  TenthCommitTime.Add(time.Minute),
			},
		},
		GerritInternalCRS: {
			{
				SystemID: ChangeListIDThatAddsNewTests,
				Owner:    UserTwo,
				Status:   code_review.Open,
				Subject:  "Add new tests",
				Updated:  TenthCommitTime.Add(time.Hour),
			},
		},
	}
}

func MakePatchSets() map[tjstore.CombinedPSID]code_review.PatchSet {
	abandonedID := tjstore.CombinedPSID{
		CRS: GerritCRS,
		CL:  "CL_was_abandoned",
		PS:  "experimental",
	}
	landedID := tjstore.CombinedPSID{
		CRS: GerritCRS,
		CL:  "CL_updates_docs",
		PS:  "docs",
	}
	return map[tjstore.CombinedPSID]code_review.PatchSet{
		idFixesIpad: {
			SystemID:     PatchSetIDFixesIPadButNotIPhone,
			ChangeListID: ChangeListIDThatAttemptsToFixIOS,
			Order:        1,
			GitHash:      "ffff111111111111111111111111111111111111",
		},
		idAddsTextCorpus: {
			SystemID:     PatchsetIDAddsNewCorpus,
			ChangeListID: ChangeListIDThatAddsNewTests,
			Order:        1,
			GitHash:      "ffff222222222222222222222222222222222222",
		},
		idAddsTextCorpusAndRoundRect: {
			SystemID:     PatchsetIDAddsNewCorpusAndTest,
			ChangeListID: ChangeListIDThatAddsNewTests,
			Order:        2,
			GitHash:      "ffff333333333333333333333333333333333333",
		},
		abandonedID: {
			SystemID:     "experimental",
			ChangeListID: "CL_was_abandoned",
			Order:        1,
			GitHash:      "ffff444444444444444444444444444444444444",
		},
		landedID: {
			SystemID:     "docs",
			ChangeListID: "CL_updates_docs",
			Order:        1,
			GitHash:      "ffff555555555555555555555555555555555555",
		},
	}
}

type TriageLogEntry struct {
	ID      string
	User    string
	TS      time.Time
	Details []ExpectationDelta
}

type ExpectationDelta struct {
	Grouping    map[string]string //  e.g. {"source_type": "round", "name": "circle"}
	Digest      types.Digest
	LabelBefore expectations.Label
	LabelAfter  expectations.Label
}

// MakeMasterBranchTriageHistory returns enough data that one could accurately reproduce what got
// triaged when. It aligns with the scenario and with MakeMasterBranchExpectations().
func MakeMasterBranchTriageHistory() []TriageLogEntry {
	cornersSquareGrouping := map[string]string{
		types.CorpusField:     CornersCorpus,
		types.PrimaryKeyField: string(SquareTest),
	}
	cornersTriangleGrouping := map[string]string{
		types.CorpusField:     CornersCorpus,
		types.PrimaryKeyField: string(TriangleTest),
	}
	roundCircleGrouping := map[string]string{
		types.CorpusField:     RoundCorpus,
		types.PrimaryKeyField: string(CircleTest),
	}
	return []TriageLogEntry{
		{
			User: UserOne,
			TS:   InitialTriageTime,
			Details: []ExpectationDelta{
				{
					Grouping:    cornersSquareGrouping,
					Digest:      DigestA01Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    cornersSquareGrouping,
					Digest:      DigestA02Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    cornersTriangleGrouping,
					Digest:      DigestB01Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    cornersTriangleGrouping,
					Digest:      DigestB02Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Negative, // accidentally triaged positive before
				},
				{
					Grouping:    cornersTriangleGrouping,
					Digest:      DigestB03Neg,
					LabelAfter:  expectations.Negative,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    cornersTriangleGrouping,
					Digest:      DigestB04Neg,
					LabelAfter:  expectations.Negative,
					LabelBefore: expectations.Positive, // accidentally triaged positive before
				},
				{
					Grouping:    roundCircleGrouping,
					Digest:      DigestBlank,
					LabelAfter:  expectations.Negative,
					LabelBefore: expectations.Positive, // accidentally triaged positive before
				},
				{
					Grouping:    roundCircleGrouping,
					Digest:      DigestC01Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    roundCircleGrouping,
					Digest:      DigestC02Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{
					Grouping:    roundCircleGrouping,
					Digest:      DigestC03Unt,
					LabelAfter:  expectations.Positive, // This is incorrectly triaged, see below.
					LabelBefore: expectations.Untriaged,
				},
			},
		},
		{
			User: UserTwo,
			TS:   ThirdCommitTime,
			Details: []ExpectationDelta{
				{
					Grouping:    cornersSquareGrouping,
					Digest:      DigestA03Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
				{ // This was accidentally triaged positive before. Developers sometimes untriage something
					// to indicate they aren't sure about it and want to get help from someone who knows the
					// tests better than they do.
					Grouping:    roundCircleGrouping,
					Digest:      DigestC03Unt,
					LabelAfter:  expectations.Untriaged,
					LabelBefore: expectations.Positive,
				},
			},
		},
		{
			User: AutoTriageUser,
			TS:   NinethCommitTime,
			Details: []ExpectationDelta{
				{
					Grouping:    cornersSquareGrouping,
					Digest:      DigestA07Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
			},
		},
		{
			User: AutoTriageUser,
			TS:   TenthCommitTime,
			Details: []ExpectationDelta{
				{
					Grouping:    cornersSquareGrouping,
					Digest:      DigestA08Pos,
					LabelAfter:  expectations.Positive,
					LabelBefore: expectations.Untriaged,
				},
			},
		},
	}
}

type DiffBetweenDigests struct {
	LeftDigest  types.Digest
	RightDigest types.Digest
	Metrics     diff.DiffMetrics
}

func MakePixelDiffsForCorpusNameGrouping() []DiffBetweenDigests {
	const circleGrouping = `{"corpus":"round","name":"circle"}`
	const triangleGrouping = `{"corpus":"corners","name":"triangle"}`
	// These were generated in TestMakeDiffsForCorpusNameGrouping_HasCorrectData and hand formatted.
	return []DiffBetweenDigests{
		{
			LeftDigest: DigestC01Pos, RightDigest: DigestC02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC01Pos, RightDigest: DigestC03Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    32,
				PixelDiffPercent: 50,
				MaxRGBADiffs:     [4]int{1, 7, 4, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC01Pos, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC01Pos, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{40, 149, 100, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC01Pos, RightDigest: DigestC06Pos_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    4,
				PixelDiffPercent: 6.25,
				MaxRGBADiffs:     [4]int{15, 12, 83, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC01Pos, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 131, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC02Pos, RightDigest: DigestC03Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC02Pos, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{3, 3, 3, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC02Pos, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 96, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC02Pos, RightDigest: DigestC06Pos_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC02Pos, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{77, 77, 77, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC03Unt, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC03Unt, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{39, 144, 97, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC03Unt, RightDigest: DigestC06Pos_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    32,
				PixelDiffPercent: 50,
				MaxRGBADiffs:     [4]int{14, 11, 80, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC03Unt, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 126, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC04Unt, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 96, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC04Unt, RightDigest: DigestC06Pos_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC04Unt, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{77, 77, 77, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC05Unt, RightDigest: DigestC06Pos_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{40, 149, 100, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC05Unt, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 27, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestC06Pos_CL, RightDigest: DigestC07Unt_CL,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 131, 168, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestBlank, RightDigest: DigestB01Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{250, 244, 197, 255},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestBlank, RightDigest: DigestB02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{239, 239, 239, 255},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestBlank, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{250, 244, 197, 255},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestBlank, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 255},
				DimDiffer:        true,
			},
		}, {
			LeftDigest: DigestB01Pos, RightDigest: DigestB02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    28,
				PixelDiffPercent: 43.75,
				MaxRGBADiffs:     [4]int{11, 5, 42, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestB01Pos, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    7,
				PixelDiffPercent: 10.9375,
				MaxRGBADiffs:     [4]int{250, 244, 197, 51},
				DimDiffer:        false},
		}, {
			LeftDigest: DigestB01Pos, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 42},
				DimDiffer:        true,
			},
		}, {
			LeftDigest: DigestB02Pos, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    34,
				PixelDiffPercent: 53.125,
				MaxRGBADiffs:     [4]int{250, 244, 197, 51},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestB02Pos, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    41,
				PixelDiffPercent: 64.0625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 42},
				DimDiffer:        true,
			},
		}, {
			LeftDigest: DigestB03Neg, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{255, 255, 255, 51},
				DimDiffer:        true,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA03Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA06Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    1,
				PixelDiffPercent: 1.5625,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA01Pos, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{4, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA03Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    1,
				PixelDiffPercent: 1.5625,
				MaxRGBADiffs:     [4]int{3, 3, 3, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    3,
				PixelDiffPercent: 4.6875,
				MaxRGBADiffs:     [4]int{3, 3, 3, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA06Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA02Pos, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA03Pos, RightDigest: DigestA04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{3, 3, 3, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA03Pos, RightDigest: DigestA05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA03Pos, RightDigest: DigestA06Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA03Pos, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA03Pos, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA04Unt, RightDigest: DigestA05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA04Unt, RightDigest: DigestA06Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA04Unt, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA04Unt, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    36,
				PixelDiffPercent: 56.25,
				MaxRGBADiffs:     [4]int{106, 21, 21, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA05Unt, RightDigest: DigestA06Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA05Unt, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    3,
				PixelDiffPercent: 4.6875,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA05Unt, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    3,
				PixelDiffPercent: 4.6875,
				MaxRGBADiffs:     [4]int{7, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA06Unt, RightDigest: DigestA07Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    1,
				PixelDiffPercent: 1.5625,
				MaxRGBADiffs:     [4]int{3, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA06Unt, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{11, 0, 0, 0},
				DimDiffer:        false,
			},
		}, {
			LeftDigest: DigestA07Pos, RightDigest: DigestA08Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    1,
				PixelDiffPercent: 1.5625,
				MaxRGBADiffs:     [4]int{11, 0, 0, 0},
				DimDiffer:        false,
			},
		},
	}
}

func MakeCommits() []tiling.Commit {
	return []tiling.Commit{
		{
			Hash:       FirstCommitHash,
			CommitTime: FirstCommitTime,
			Author:     UserOne,
			Subject:    "first commit",
		},
		{
			Hash:       SecondCommitHash,
			CommitTime: SecondCommitTime,
			Author:     UserTwo,
			Subject:    "second commit",
		},
		{
			Hash:       ThirdCommitHash,
			CommitTime: ThirdCommitTime,
			Author:     UserOne,
			Subject:    "third commit",
		},
		{
			Hash:       FourthCommitHash,
			CommitTime: FourthCommitTime,
			Author:     UserTwo,
			Subject:    "fourth commit",
		},
		{
			Hash:       FifthCommitHash,
			CommitTime: FifthCommitTime,
			Author:     UserOne,
			Subject:    "fifth commit",
		},
		{
			Hash:       SixthCommitHash,
			CommitTime: SixthCommitTime,
			Author:     UserTwo,
			Subject:    "sixth commit",
		},
		{
			Hash:       SeventhCommitHash,
			CommitTime: SeventhCommitTime,
			Author:     UserOne,
			Subject:    "seventh commit",
		},
		{
			Hash:       EighthCommitHash,
			CommitTime: EighthCommitTime,
			Author:     UserTwo,
			Subject:    "eighth commit",
		},
		{
			Hash:       NinethCommitHash,
			CommitTime: NinethCommitTime,
			Author:     UserOne,
			Subject:    "nineth commit",
		},
		{
			Hash:       TenthCommitHash,
			CommitTime: TenthCommitTime,
			Author:     UserTwo,
			Subject:    "tenth commit",
		},
	}
}

const (
	NumCommits = 10

	RoundCorpus   = "round"
	CornersCorpus = "corners"
	// The following corpus is added in a CL
	TextCorpus = "text"

	CircleTest   = types.TestName("circle")
	SquareTest   = types.TestName("square")
	TriangleTest = types.TestName("triangle")
	// The following tests are added in a CL
	SevenTest     = types.TestName("seven")
	RoundRectTest = types.TestName("round rect")

	ColorModeKey    = "color mode" // There is intentionally a space here to make sure we handle it.
	DeviceKey       = "device"
	ExtensionOption = "ext"
	OSKey           = "os"

	AndroidOS       = "Android"
	iOS             = "iOS"
	Windows10dot2OS = "Windows10.2"
	Windows10dot3OS = "Windows10.3"

	QuadroDevice  = "QuadroP400"
	IPadDevice    = "iPad6,3"    // These happen to have a comma in it, which needs to be sanitized
	IPhoneDevice  = "iPhone12,1" // in the trace id.
	WalleyeDevice = "walleye"

	GreyColorMode = "GREY"
	RGBColorMode  = "RGB"

	PNGExtension = "png"

	// Digests starting with an "a" belong to the square tests, a "b" prefix is for triangle, and so
	// on. The numbers (and thus the hash itself) are arbitrary. The suffix reveals how these are
	// triaged as of the last commit.
	// Square Images
	DigestA01Pos = types.Digest("a01a01a01a01a01a01a01a01a01a01a0")
	DigestA02Pos = types.Digest("a02a02a02a02a02a02a02a02a02a02a0") // GREY version of A01
	DigestA03Pos = types.Digest("a03a03a03a03a03a03a03a03a03a03a0") // small diff from A02
	DigestA04Unt = types.Digest("a04a04a04a04a04a04a04a04a04a04a0") // small diff from A02
	DigestA05Unt = types.Digest("a05a05a05a05a05a05a05a05a05a05a0") // small diff from A01
	DigestA06Unt = types.Digest("a06a06a06a06a06a06a06a06a06a06a0") // small diff from A01
	DigestA07Pos = types.Digest("a07a07a07a07a07a07a07a07a07a07a0") // small diff from A01
	DigestA08Pos = types.Digest("a08a08a08a08a08a08a08a08a08a08a0") // small diff from A01

	// Triangle Images
	DigestB01Pos = types.Digest("b01b01b01b01b01b01b01b01b01b01b0")
	DigestB02Pos = types.Digest("b02b02b02b02b02b02b02b02b02b02b0") // GREY version of B01
	DigestB03Neg = types.Digest("b03b03b03b03b03b03b03b03b03b03b0") // big diff from B01
	DigestB04Neg = types.Digest("b04b04b04b04b04b04b04b04b04b04b0") // truncated version of B02

	// Circle Images
	DigestC01Pos    = types.Digest("c01c01c01c01c01c01c01c01c01c01c0")
	DigestC02Pos    = types.Digest("c02c02c02c02c02c02c02c02c02c02c0") // GREY version of C01
	DigestC03Unt    = types.Digest("c03c03c03c03c03c03c03c03c03c03c0") // small diff from C01
	DigestC04Unt    = types.Digest("c04c04c04c04c04c04c04c04c04c04c0") // small diff from C02
	DigestC05Unt    = types.Digest("c05c05c05c05c05c05c05c05c05c05c0") // big incorrect diff from C01
	DigestC06Pos_CL = types.Digest("c06c06c06c06c06c06c06c06c06c06c0") // small diff from C01
	DigestC07Unt_CL = types.Digest("c07c07c07c07c07c07c07c07c07c07c0") // big incorrect diff from C02

	// Seven Images (it is intentional that there is only one triaged digest here).
	DigestD01Pos_CL = types.Digest("d01d01d01d01d01d01d01d01d01d01d0")

	// RoundRect Images
	DigestE01Pos_CL = types.Digest("e01e01e01e01e01e01e01e01e01e01e0")
	DigestE02Pos_CL = types.Digest("e02e02e02e02e02e02e02e02e02e02e0") // GREY version of E01

	// This digest is for a blank image, which has been triaged as negative on the circle test,
	// but not the others. It shows up in one other trace and one CL, where it should be seen as
	// Untriaged.
	DigestBlank = types.Digest("00000000000000000000000000000000")

	DigestNoData = tiling.MissingDigest

	UserOne        = "userOne@example.com"
	UserTwo        = "userTwo@example.com"
	AutoTriageUser = "fuzzy" // we use the algorithm name as the user name for auto triaging.

	FirstCommitHash   = "cccc111111111111111111111111111111111111"
	SecondCommitHash  = "cccc222222222222222222222222222222222222"
	ThirdCommitHash   = "cccc333333333333333333333333333333333333"
	FourthCommitHash  = "cccc444444444444444444444444444444444444"
	FifthCommitHash   = "cccc555555555555555555555555555555555555"
	SixthCommitHash   = "cccc666666666666666666666666666666666666"
	SeventhCommitHash = "cccc777777777777777777777777777777777777"
	EighthCommitHash  = "cccc888888888888888888888888888888888888"
	NinethCommitHash  = "cccc999999999999999999999999999999999999"
	TenthCommitHash   = "ccccAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	ChangeListIDThatAttemptsToFixIOS = "CL_fix_ios"
	PatchSetIDFixesIPadButNotIPhone  = "PS_fixes_ipad_but_not_iphone"

	ChangeListIDThatAddsNewTests   = "CL_new_tests"
	PatchsetIDAddsNewCorpus        = "PS_adds_new_corpus"
	PatchsetIDAddsNewCorpusAndTest = "PS_adds_new_corpus_and_test"

	GerritCRS         = "gerrit"
	GerritInternalCRS = "gerrit_internal"

	BuildBucket         = "buildbucket"
	BuildBucketInternal = "buildbucketInternal"
)

var (
	InitialTriageTime = time.Date(2020, time.February, 28, 0, 0, 0, 0, time.UTC)
	FirstCommitTime   = time.Date(2020, time.March, 1, 0, 0, 0, 0, time.UTC)
	SecondCommitTime  = time.Date(2020, time.March, 2, 0, 0, 0, 0, time.UTC)
	ThirdCommitTime   = time.Date(2020, time.March, 3, 0, 0, 0, 0, time.UTC)
	FourthCommitTime  = time.Date(2020, time.March, 4, 0, 0, 0, 0, time.UTC)
	FifthCommitTime   = time.Date(2020, time.March, 5, 0, 0, 0, 0, time.UTC)
	SixthCommitTime   = time.Date(2020, time.March, 6, 0, 0, 0, 0, time.UTC)
	SeventhCommitTime = time.Date(2020, time.March, 7, 0, 0, 0, 0, time.UTC)
	EighthCommitTime  = time.Date(2020, time.March, 8, 0, 0, 0, 0, time.UTC)
	NinethCommitTime  = time.Date(2020, time.March, 9, 0, 0, 0, 0, time.UTC)
	TenthCommitTime   = time.Date(2020, time.March, 10, 0, 0, 0, 0, time.UTC)
)

var (
	idFixesIpad = tjstore.CombinedPSID{
		CRS: GerritCRS,
		CL:  ChangeListIDThatAttemptsToFixIOS,
		PS:  PatchSetIDFixesIPadButNotIPhone,
	}
	idAddsTextCorpus = tjstore.CombinedPSID{
		CRS: GerritInternalCRS,
		CL:  ChangeListIDThatAddsNewTests,
		PS:  PatchsetIDAddsNewCorpus,
	}
	idAddsTextCorpusAndRoundRect = tjstore.CombinedPSID{
		CRS: GerritInternalCRS,
		CL:  ChangeListIDThatAddsNewTests,
		PS:  PatchsetIDAddsNewCorpusAndTest,
	}
)
