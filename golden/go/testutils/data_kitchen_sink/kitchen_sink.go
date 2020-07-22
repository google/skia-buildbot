// Package data_kitchen_sink demonstrates all of the data that Gold stores. It strives to
// encompass as much of the scenarios we see in real world data as possible, while being small
// and organized enough to be comprehensible.
//
// The scenario here is that there are three tests (circle, square, triangle) that produce an image
// of the given shape. These are divided into two corpora (round and corners). At the beginning of
// the data timeline (which is 10 commits long) these tests are run on a Windows 10.2 machine, two
// iOS devices ("iPad6,3" and "iPhone12,1") and two Android devices ("marlin" and "walleye").
// On each of these devices, the tests are run in RGB mode and GREY mode, producing outputs that
// are in color or greyscale.
//
// This means we start with 30 traces (3 tests * 5 devices * 2 color_mode). See MakeTraces for
// some specific comments on any of the given traces.
//
// At specific commits things happen as follows:
//  - At commit index 3, the Windows 10.2 device is upgraded to 10.3. This causes a slight change in
//    the circle tests, producing untriaged output.
//
//
// Future growth: When Gold is ready to have a more generic "grouping" for traces, test name +
// color_mode is a natural split here.
package data_kitchen_sink

import (
	"time"

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
					ExtensionKey: PNGExtension,
				}),
		},
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
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionKey: PNGExtension,
				}),
		},
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
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionKey: PNGExtension,
				}),
		},
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
					types.PrimaryKeyField: string(TriangleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionKey: PNGExtension,
				}),
		},
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
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          RGBColorMode,
				}, map[string]string{
					ExtensionKey: PNGExtension,
				}),
		},
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
					types.CorpusField:     RoundCorpus,
					types.PrimaryKeyField: string(CircleTest),
					ColorModeKey:          GreyColorMode,
				}, map[string]string{
					ExtensionKey: PNGExtension,
				}),
		},
		// ============= Windows 10.3 traces =============
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
		{
			ID: ",color mode=RGB,device=QuadroP400,name=square,os=Windows10.2,source_type=corners,",
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
					ExtensionKey: PNGExtension,
				}),
		},
	}
}

func MakeExpectations() expectations.Classifier {
	var e expectations.Expectations
	e.Set(SquareTest, DigestA01Pos, expectations.Positive)
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

	ExtensionKey = "ext"
	OSKey        = "os"
	DeviceKey    = "device"
	ColorModeKey = "color mode" // There is intentionally a space here to make sure we handle it.

	Windows10dot2OS = "Windows10.2"
	Windows10dot3OS = "Windows10.3"

	QuadroDevice = "QuadroP400"

	RGBColorMode  = "RGB"
	GreyColorMode = "GREY"

	PNGExtension = "png"

	DigestA01Pos = types.Digest("a01a01a01a01a01a01a01a01a01a01a0")

	DigestNoData = tiling.MissingDigest

	UserOne = "userOne@example.com"
)

var (
	InitialTriageTime = time.Date(2020, time.March, 1, 0, 0, 0, 0, time.UTC)
)
