package data

import (
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
)

func TestSortedFuzzReports(t *testing.T) {
	a := make(SortedFuzzReports, 0, 5)
	addingOrder := []string{"gggg", "aaaa", "cccc", "eeee", "dddd", "bbbb",
		"ffff"}

	for _, key := range addingOrder {
		a = a.append(mockPictureDetails[key])
	}

	b := make(SortedFuzzReports, 0, 5)
	sortedOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, mockPictureDetails[key])
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected: %#v\n, but was: %#v", a, b)
	}
}

func TestAddFuzz(t *testing.T) {
	builder := loadReports()

	report := builder.getTreeSortedByTotal("skpicture")
	if !reflect.DeepEqual(expectedPictureTree, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedPictureTree, report)
	}

	report = builder.getTreeSortedByTotal("api")
	if !reflect.DeepEqual(expectedAPITree, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedAPITree, report)
	}
}

func loadReports() *treeReportBuilder {
	addingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg"}

	builder := newBuilder()
	for _, key := range addingOrder {
		builder.addFuzzReport("skpicture", mockPictureDetails[key])
	}
	addingOrder = []string{"iiii", "hhhh"}
	for _, key := range addingOrder {
		builder.addFuzzReport("api", mockAPIDetails[key])
	}
	return builder
}

func makeStacktrace(file, function string, line int) StackTrace {
	return StackTrace{
		Frames: []StackTraceFrame{
			{
				PackageName:  "mock/package/",
				FileName:     file,
				LineNumber:   line,
				FunctionName: function,
			},
		},
	}
}

var mockFlags = []string{"foo", "bar"}

var mockPictureDetails = map[string]FuzzReport{
	"aaaa": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "aaaa",
		FuzzCategory:      "skpicture",
	},
	"bbbb": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "bbbb",
		FuzzCategory:      "skpicture",
	},
	"cccc": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "gamma", 26),
		DebugFlags:        mockFlags,
		FuzzName:          "cccc",
		FuzzCategory:      "skpicture",
	},
	"dddd": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace: makeStacktrace("delta", "epsilon", 125),
		DebugFlags:        mockFlags,
		FuzzName:          "dddd",
		FuzzCategory:      "skpicture",
	},
	"eeee": FuzzReport{
		DebugStackTrace:   StackTrace{},
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "eeee",
		FuzzCategory:      "skpicture",
	},
	"ffff": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "ffff",
		FuzzCategory:      "skpicture",
	},
	"gggg": FuzzReport{
		DebugStackTrace:   makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "gggg",
		FuzzCategory:      "skpicture",
	},
}

var mockAPIDetails = map[string]FuzzReport{
	"hhhh": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "hhhh",
		FuzzCategory:      "api",
	},
	"iiii": FuzzReport{
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "iiii",
		FuzzCategory:      "api",
	},
}

var expectedPictureTree = FuzzReportTree{
	FileFuzzReport{
		FileName: "mock/package/alpha", Count: 4, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "beta", Count: 3, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 16, Count: 3, Details: []FuzzReport{mockPictureDetails["aaaa"], mockPictureDetails["bbbb"], mockPictureDetails["ffff"]},
					},
				},
			}, FunctionFuzzReport{
				FunctionName: "gamma", Count: 1, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 26, Count: 1, Details: []FuzzReport{mockPictureDetails["cccc"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		FileName: "mock/package/delta", Count: 2, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "epsilon", Count: 2, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 122, Count: 1, Details: []FuzzReport{mockPictureDetails["gggg"]},
					},
					LineFuzzReport{
						LineNumber: 125, Count: 1, Details: []FuzzReport{mockPictureDetails["dddd"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		FileName: common.UNKNOWN_FILE, Count: 1, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: common.UNKNOWN_FUNCTION, Count: 1, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: -1, Count: 1, Details: []FuzzReport{mockPictureDetails["eeee"]},
					},
				},
			},
		},
	},
}

var expectedPictureSummary = FuzzReportTree{
	FileFuzzReport{
		FileName: "mock/package/alpha", Count: 4, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "beta", Count: 3, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 16, Count: 3, Details: nil,
					},
				},
			}, FunctionFuzzReport{
				FunctionName: "gamma", Count: 1, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 26, Count: 1, Details: nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		FileName: "mock/package/delta", Count: 2, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "epsilon", Count: 2, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 122, Count: 1, Details: nil,
					},
					LineFuzzReport{
						LineNumber: 125, Count: 1, Details: nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		FileName: common.UNKNOWN_FILE, Count: 1, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: common.UNKNOWN_FUNCTION, Count: 1, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: -1, Count: 1, Details: nil,
					},
				},
			},
		},
	},
}

var expectedAPITree = FuzzReportTree{
	FileFuzzReport{
		FileName: "mock/package/alpha", Count: 2, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "beta", Count: 2, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 16, Count: 2, Details: []FuzzReport{mockAPIDetails["hhhh"], mockAPIDetails["iiii"]},
					},
				},
			},
		},
	},
}

var expectedAPISummary = FuzzReportTree{
	FileFuzzReport{
		FileName: "mock/package/alpha", Count: 2, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "beta", Count: 2, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 16, Count: 2, Details: nil,
					},
				},
			},
		},
	},
}
