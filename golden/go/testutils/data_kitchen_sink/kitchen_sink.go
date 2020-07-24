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
// Future growth: When Gold is ready to have a more generic "grouping" for traces, test name +
// color_mode is a natural split here.
package data_kitchen_sink

import (
	"time"

	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/tiling"
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

func MakeExpectations() expectations.Classifier {
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

// MakeTriageHistory returns enough data that one could accurately reproduce what got triaged when.
// It aligns with the scenario and with MakeExpectations().
func MakeTriageHistory() []expectations.TriageLogEntry {
	return []expectations.TriageLogEntry{
		{
			User: UserOne,
			TS:   InitialTriageTime,
			Details: []expectations.Delta{
				{
					Grouping: SquareTest,
					Digest:   DigestA01Pos,
					Label:    expectations.Positive,
				},
				{
					Grouping: SquareTest,
					Digest:   DigestA02Pos,
					Label:    expectations.Positive,
				},
				{
					Grouping: TriangleTest,
					Digest:   DigestB01Pos,
					Label:    expectations.Positive,
				},
				{
					Grouping: TriangleTest,
					Digest:   DigestB02Pos,
					Label:    expectations.Positive,
				},
				{
					Grouping: TriangleTest,
					Digest:   DigestB03Neg,
					Label:    expectations.Negative,
				},
				{
					Grouping: TriangleTest,
					Digest:   DigestB04Neg,
					Label:    expectations.Negative,
				},
				{
					Grouping: CircleTest,
					Digest:   DigestBlank,
					Label:    expectations.Negative,
				},
				{
					Grouping: CircleTest,
					Digest:   DigestC01Pos,
					Label:    expectations.Positive,
				},
				{
					Grouping: CircleTest,
					Digest:   DigestC02Pos,
					Label:    expectations.Positive,
				},
			},
		},
		{
			User: UserTwo,
			TS:   ThirdCommitTime,
			Details: []expectations.Delta{
				{
					Grouping: SquareTest,
					Digest:   DigestA03Pos,
					Label:    expectations.Positive,
				},
			},
		},
		{
			User: AutoTriageUser,
			TS:   NinethCommitTime,
			Details: []expectations.Delta{
				{
					Grouping: SquareTest,
					Digest:   DigestA07Pos,
					Label:    expectations.Positive,
				},
			},
		},
		{
			User: AutoTriageUser,
			TS:   TenthCommitTime,
			Details: []expectations.Delta{
				{
					Grouping: SquareTest,
					Digest:   DigestA08Pos,
					Label:    expectations.Positive,
				},
			},
		},
	}
}

type DiffBetweenDigests struct {
	Grouping    string // TODO(kjlubick) might not need
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
			Grouping: circleGrouping, LeftDigest: DigestC01Pos, RightDigest: DigestC02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC01Pos, RightDigest: DigestC03Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    32,
				PixelDiffPercent: 50,
				MaxRGBADiffs:     [4]int{1, 7, 4, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC01Pos, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC01Pos, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{40, 149, 100, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC02Pos, RightDigest: DigestC03Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC02Pos, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    2,
				PixelDiffPercent: 3.125,
				MaxRGBADiffs:     [4]int{3, 3, 3, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC02Pos, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 96, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC03Unt, RightDigest: DigestC04Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 66, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC03Unt, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    44,
				PixelDiffPercent: 68.75,
				MaxRGBADiffs:     [4]int{39, 144, 97, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: circleGrouping, LeftDigest: DigestC04Unt, RightDigest: DigestC05Unt,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{141, 96, 168, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestBlank, RightDigest: DigestB01Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{250, 244, 197, 255},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestBlank, RightDigest: DigestB02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{239, 239, 239, 255},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestBlank, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{250, 244, 197, 255},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestBlank, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 255},
				DimDiffer:        true,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB01Pos, RightDigest: DigestB02Pos,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    28,
				PixelDiffPercent: 43.75,
				MaxRGBADiffs:     [4]int{11, 5, 42, 0},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB01Pos, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    7,
				PixelDiffPercent: 10.9375,
				MaxRGBADiffs:     [4]int{250, 244, 197, 51},
				DimDiffer:        false},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB01Pos, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    58,
				PixelDiffPercent: 90.625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 42},
				DimDiffer:        true,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB02Pos, RightDigest: DigestB03Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    34,
				PixelDiffPercent: 53.125,
				MaxRGBADiffs:     [4]int{250, 244, 197, 51},
				DimDiffer:        false,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB02Pos, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    41,
				PixelDiffPercent: 64.0625,
				MaxRGBADiffs:     [4]int{255, 255, 255, 42},
				DimDiffer:        true,
			},
		}, {
			Grouping: triangleGrouping, LeftDigest: DigestB03Neg, RightDigest: DigestB04Neg,
			Metrics: diff.DiffMetrics{
				NumDiffPixels:    64,
				PixelDiffPercent: 100,
				MaxRGBADiffs:     [4]int{255, 255, 255, 51},
				DimDiffer:        true,
			},
		},
	}
}

const (
	NumCommits = 10

	RoundCorpus   = "round"
	CornersCorpus = "corners"

	CircleTest   = types.TestName("circle")
	SquareTest   = types.TestName("square")
	TriangleTest = types.TestName("triangle")

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

	// Digests starting with an "a" belong to the square tests, a "b" prefix is for triangle, and a
	// "c" prefix is for the circle tests. The numbers (and thus the hash itself) are arbitrary.
	// The suffix reveals how these are triaged as of the last commit.
	DigestA01Pos = types.Digest("a01a01a01a01a01a01a01a01a01a01a0")
	DigestA02Pos = types.Digest("a02a02a02a02a02a02a02a02a02a02a0") // GREY version of A01
	DigestA03Pos = types.Digest("a03a03a03a03a03a03a03a03a03a03a0") // small diff from A02
	DigestA04Unt = types.Digest("a04a04a04a04a04a04a04a04a04a04a0") // small diff from A02
	DigestA05Unt = types.Digest("a05a05a05a05a05a05a05a05a05a05a0") // small diff from A01
	DigestA06Unt = types.Digest("a06a06a06a06a06a06a06a06a06a06a0") // small diff from A01
	DigestA07Pos = types.Digest("a07a07a07a07a07a07a07a07a07a07a0") // small diff from A01
	DigestA08Pos = types.Digest("a08a08a08a08a08a08a08a08a08a08a0") // small diff from A01

	DigestB01Pos = types.Digest("b01b01b01b01b01b01b01b01b01b01b0")
	DigestB02Pos = types.Digest("b02b02b02b02b02b02b02b02b02b02b0") // GREY version of B01
	DigestB03Neg = types.Digest("b03b03b03b03b03b03b03b03b03b03b0") // big diff from B01
	DigestB04Neg = types.Digest("b04b04b04b04b04b04b04b04b04b04b0") // truncated version of B02

	DigestC01Pos = types.Digest("c01c01c01c01c01c01c01c01c01c01c0")
	DigestC02Pos = types.Digest("c02c02c02c02c02c02c02c02c02c02c0") // GREY version of C01
	DigestC03Unt = types.Digest("c03c03c03c03c03c03c03c03c03c03c0") // small diff from C01
	DigestC04Unt = types.Digest("c04c04c04c04c04c04c04c04c04c04c0") // small diff from C02
	DigestC05Unt = types.Digest("c05c05c05c05c05c05c05c05c05c05c0") // big diff from C01

	// This digest is for a blank image, which has been triaged as negative on the circle test,
	// but not the others. It has shown up on the Triangle test and should be identified as untriaged
	// there.
	DigestBlank = types.Digest("00000000000000000000000000000000")

	DigestNoData = tiling.MissingDigest

	UserOne        = "userOne@example.com"
	UserTwo        = "userTwo@example.com"
	AutoTriageUser = "fuzzy" // we use the algorithm name as the user name for auto triaging.
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
