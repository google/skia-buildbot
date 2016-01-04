package fuzz

import (
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
)

func TestSortedBinaryFuzzReports(t *testing.T) {
	a := make(SortedBinaryFuzzReports, 0, 5)
	addingOrder := []string{"gggg", "aaaa", "cccc", "eeee", "dddd", "bbbb",
		"ffff"}

	for _, key := range addingOrder {
		a = a.append(mockBinaryDetails[key])
	}

	b := make(SortedBinaryFuzzReports, 0, 5)
	sortedOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, mockBinaryDetails[key])
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected: %#v\n, but was: %#v", a, b)
	}
}

func TestSortedAPIFuzzReports(t *testing.T) {
	a := make(SortedAPIFuzzReports, 0, 5)
	addingOrder := []string{"mmmm", "nnnn", "qqqq", "pppp",
		"oooo", "rrrr", "ssss"}

	for _, key := range addingOrder {
		a = a.append(mockAPIDetails[key])
	}

	b := make(SortedAPIFuzzReports, 0, 5)
	sortedOrder := []string{"mmmm", "nnnn", "oooo", "pppp", "qqqq",
		"rrrr", "ssss"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, mockAPIDetails[key])
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected: %#v\n, but was: %#v", a, b)
	}
}

func TestAddBinary(t *testing.T) {
	addingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg"}

	builder := treeReportBuilder{}
	for _, key := range addingOrder {
		builder.addReportBinary(mockBinaryDetails[key])
	}

	report := builder.getBinaryTreeSortedByTotal()

	if !reflect.DeepEqual(expectedBinaryReport1, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedBinaryReport1, report)
	}
}

func TestAddAPI(t *testing.T) {
	addingOrder := []string{"mmmm", "nnnn", "qqqq", "pppp",
		"oooo", "rrrr", "ssss"}

	builder := treeReportBuilder{}
	for _, key := range addingOrder {
		builder.addReportAPI(mockAPIDetails[key])
	}

	report := builder.getAPITreeSortedByTotal()

	if !reflect.DeepEqual(expectedAPIReport1, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedAPIReport1, report)
	}
}

func TestAddBoth(t *testing.T) {
	// Tests the ability to remove the full api reports from the binary reports and vice versa
	builder := loadAPIAndBinary()

	apiReport := builder.getAPITreeSortedByTotal()

	if !reflect.DeepEqual(expectedAPIReport2, apiReport) {
		t.Errorf("API Report Expected: %#v\n, but was: %#v", expectedAPIReport2, apiReport)
	}

	binaryReport := builder.getBinaryTreeSortedByTotal()

	if !reflect.DeepEqual(expectedBinaryReport2, binaryReport) {
		t.Errorf("Binary Report Expected: %#v\n, but was: %#v", expectedBinaryReport2, binaryReport)
	}
}

func loadAPIAndBinary() *treeReportBuilder {
	binaryAddingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg"}
	apiAddingOrder := []string{"mmmm", "nnnn", "qqqq", "pppp",
		"oooo", "rrrr", "ssss"}
	builder := treeReportBuilder{}
	for i := range binaryAddingOrder {
		builder.addReportBinary(mockBinaryDetails[binaryAddingOrder[i]])
		builder.addReportAPI(mockAPIDetails[apiAddingOrder[i]])
	}
	return &builder
}

func TestSummary(t *testing.T) {
	builder := loadAPIAndBinary()

	summary := builder.getSummarySortedByTotal()

	if !reflect.DeepEqual(expectedSummary, summary) {
		t.Errorf("Summary Report Expected: %#v\n, but was: %#v", expectedSummary, summary)
	}
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

var mockBinaryDetails = map[string]BinaryFuzzReport{
	"aaaa": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "aaaa",
		BinaryType:         "skp",
	},
	"bbbb": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "bbbb",
		BinaryType:         "skp",
	},
	"cccc": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "cccc",
		BinaryType:         "skp",
	},
	"dddd": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("delta", "epsilon", 125),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "dddd",
		BinaryType:         "png",
	},
	"eeee": BinaryFuzzReport{
		DebugStackTrace:    StackTrace{},
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "eeee",
		BinaryType:         "png",
	},
	"ffff": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "ffff",
		BinaryType:         "skp",
	},
	"gggg": BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "gggg",
		BinaryType:         "png",
	},
}

var expectedBinaryReport1 = FuzzReportTree{
	FileFuzzReport{
		"mock/package/alpha", 4, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				"beta", 3, 0, []LineFuzzReport{
					LineFuzzReport{
						16, 3, 0, []BinaryFuzzReport{mockBinaryDetails["aaaa"], mockBinaryDetails["bbbb"], mockBinaryDetails["ffff"]}, nil,
					},
				},
			}, FunctionFuzzReport{
				"gamma", 1, 0, []LineFuzzReport{
					LineFuzzReport{
						26, 1, 0, []BinaryFuzzReport{mockBinaryDetails["cccc"]}, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/delta", 2, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				"epsilon", 2, 0, []LineFuzzReport{
					LineFuzzReport{
						122, 1, 0, []BinaryFuzzReport{mockBinaryDetails["gggg"]}, nil,
					},
					LineFuzzReport{
						125, 1, 0, []BinaryFuzzReport{mockBinaryDetails["dddd"]}, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		common.UNKNOWN_FILE, 1, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				common.UNKNOWN_FUNCTION, 1, 0, []LineFuzzReport{
					LineFuzzReport{
						-1, 1, 0, []BinaryFuzzReport{mockBinaryDetails["eeee"]}, nil,
					},
				},
			},
		},
	},
}

var expectedBinaryReport2 = FuzzReportTree{
	FileFuzzReport{
		"mock/package/alpha", 4, 4, []FunctionFuzzReport{
			FunctionFuzzReport{
				"beta", 3, 3, []LineFuzzReport{
					LineFuzzReport{
						16, 3, 3, []BinaryFuzzReport{mockBinaryDetails["aaaa"], mockBinaryDetails["bbbb"], mockBinaryDetails["ffff"]}, nil,
					},
				},
			}, FunctionFuzzReport{
				"gamma", 1, 1, []LineFuzzReport{
					LineFuzzReport{
						26, 1, 1, []BinaryFuzzReport{mockBinaryDetails["cccc"]}, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		common.UNKNOWN_FILE, 1, 1, []FunctionFuzzReport{
			FunctionFuzzReport{
				common.UNKNOWN_FUNCTION, 1, 1, []LineFuzzReport{
					LineFuzzReport{
						-1, 1, 1, []BinaryFuzzReport{mockBinaryDetails["eeee"]}, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/delta", 2, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				"epsilon", 2, 0, []LineFuzzReport{
					LineFuzzReport{
						122, 1, 0, []BinaryFuzzReport{mockBinaryDetails["gggg"]}, nil,
					},
					LineFuzzReport{
						125, 1, 0, []BinaryFuzzReport{mockBinaryDetails["dddd"]}, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/zeta", 0, 2, []FunctionFuzzReport{
			FunctionFuzzReport{
				"theta", 0, 2, []LineFuzzReport{
					LineFuzzReport{
						122, 0, 1, nil, nil,
					},
					LineFuzzReport{
						125, 0, 1, nil, nil,
					},
				},
			},
		},
	},
}

var mockAPIDetails = map[string]APIFuzzReport{
	"mmmm": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		TestName:           "mmmm",
	},
	"nnnn": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "nnnn",
	},
	"oooo": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		TestName:           "oooo",
	},
	"pppp": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("zeta", "theta", 125),
		HumanReadableFlags: mockFlags,
		TestName:           "pppp",
	},
	"qqqq": APIFuzzReport{
		DebugStackTrace:    StackTrace{},
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "qqqq",
	},
	"rrrr": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		TestName:           "rrrr",
	},
	"ssss": APIFuzzReport{
		DebugStackTrace:    makeStacktrace("zeta", "theta", 122),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "ssss",
	},
}

var expectedAPIReport1 = FuzzReportTree{
	FileFuzzReport{
		"mock/package/alpha", 0, 4, []FunctionFuzzReport{
			FunctionFuzzReport{
				"beta", 0, 3, []LineFuzzReport{
					LineFuzzReport{
						16, 0, 3, nil, []APIFuzzReport{mockAPIDetails["mmmm"], mockAPIDetails["nnnn"], mockAPIDetails["rrrr"]},
					},
				},
			}, FunctionFuzzReport{
				"gamma", 0, 1, []LineFuzzReport{
					LineFuzzReport{
						26, 0, 1, nil, []APIFuzzReport{mockAPIDetails["oooo"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/zeta", 0, 2, []FunctionFuzzReport{
			FunctionFuzzReport{
				"theta", 0, 2, []LineFuzzReport{
					LineFuzzReport{
						122, 0, 1, nil, []APIFuzzReport{mockAPIDetails["ssss"]},
					},
					LineFuzzReport{
						125, 0, 1, nil, []APIFuzzReport{mockAPIDetails["pppp"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		common.UNKNOWN_FILE, 0, 1, []FunctionFuzzReport{
			FunctionFuzzReport{
				common.UNKNOWN_FUNCTION, 0, 1, []LineFuzzReport{
					LineFuzzReport{
						-1, 0, 1, nil, []APIFuzzReport{mockAPIDetails["qqqq"]},
					},
				},
			},
		},
	},
}

var expectedAPIReport2 = FuzzReportTree{
	FileFuzzReport{
		"mock/package/alpha", 4, 4, []FunctionFuzzReport{
			FunctionFuzzReport{
				"beta", 3, 3, []LineFuzzReport{
					LineFuzzReport{
						16, 3, 3, nil, []APIFuzzReport{mockAPIDetails["mmmm"], mockAPIDetails["nnnn"], mockAPIDetails["rrrr"]},
					},
				},
			}, FunctionFuzzReport{
				"gamma", 1, 1, []LineFuzzReport{
					LineFuzzReport{
						26, 1, 1, nil, []APIFuzzReport{mockAPIDetails["oooo"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		common.UNKNOWN_FILE, 1, 1, []FunctionFuzzReport{
			FunctionFuzzReport{
				common.UNKNOWN_FUNCTION, 1, 1, []LineFuzzReport{
					LineFuzzReport{
						-1, 1, 1, nil, []APIFuzzReport{mockAPIDetails["qqqq"]},
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/delta", 2, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				"epsilon", 2, 0, []LineFuzzReport{
					LineFuzzReport{
						122, 1, 0, nil, nil,
					},
					LineFuzzReport{
						125, 1, 0, nil, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/zeta", 0, 2, []FunctionFuzzReport{
			FunctionFuzzReport{
				"theta", 0, 2, []LineFuzzReport{
					LineFuzzReport{
						122, 0, 1, nil, []APIFuzzReport{mockAPIDetails["ssss"]},
					},
					LineFuzzReport{
						125, 0, 1, nil, []APIFuzzReport{mockAPIDetails["pppp"]},
					},
				},
			},
		},
	},
}

var expectedSummary = FuzzReportTree{
	FileFuzzReport{
		"mock/package/alpha", 4, 4, []FunctionFuzzReport{
			FunctionFuzzReport{
				"beta", 3, 3, []LineFuzzReport{
					LineFuzzReport{
						16, 3, 3, nil, nil,
					},
				},
			}, FunctionFuzzReport{
				"gamma", 1, 1, []LineFuzzReport{
					LineFuzzReport{
						26, 1, 1, nil, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		common.UNKNOWN_FILE, 1, 1, []FunctionFuzzReport{
			FunctionFuzzReport{
				common.UNKNOWN_FUNCTION, 1, 1, []LineFuzzReport{
					LineFuzzReport{
						-1, 1, 1, nil, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/delta", 2, 0, []FunctionFuzzReport{
			FunctionFuzzReport{
				"epsilon", 2, 0, []LineFuzzReport{
					LineFuzzReport{
						122, 1, 0, nil, nil,
					},
					LineFuzzReport{
						125, 1, 0, nil, nil,
					},
				},
			},
		},
	},
	FileFuzzReport{
		"mock/package/zeta", 0, 2, []FunctionFuzzReport{
			FunctionFuzzReport{
				"theta", 0, 2, []LineFuzzReport{
					LineFuzzReport{
						122, 0, 1, nil, nil,
					},
					LineFuzzReport{
						125, 0, 1, nil, nil,
					},
				},
			},
		},
	},
}
