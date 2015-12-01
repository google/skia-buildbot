package fuzz

import (
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
)

func TestAddBinary(t *testing.T) {

	addingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg"}

	builder := fuzzReportBuilder{}
	for _, key := range addingOrder {
		builder.addReportBinary(mockBinaryDetails[key])
	}

	report := builder.getBinaryReportSortedByTotal()

	if !reflect.DeepEqual(expectedBinaryReport1, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedBinaryReport1, report)
	}
}

func TestAddAPI(t *testing.T) {
	addingOrder := []string{"mmmm", "nnnn", "qqqq", "pppp",
		"oooo", "rrrr", "ssss"}

	builder := fuzzReportBuilder{}
	for _, key := range addingOrder {
		builder.addReportAPI(mockAPIDetails[key])
	}

	report := builder.getAPIReportSortedByTotal()

	if !reflect.DeepEqual(expectedAPIReport1, report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedAPIReport1, report)
	}
}

func TestAddBoth(t *testing.T) {
	// Tests the ability to remove the full api reports from the binary reports and vice versa
	builder := loadAPIAndBinary()

	apiReport := builder.getAPIReportSortedByTotal()

	if !reflect.DeepEqual(expectedAPIReport2, apiReport) {
		t.Errorf("API Report Expected: %#v\n, but was: %#v", expectedAPIReport2, apiReport)
	}

	binaryReport := builder.getBinaryReportSortedByTotal()

	if !reflect.DeepEqual(expectedBinaryReport2, binaryReport) {
		t.Errorf("Binary Report Expected: %#v\n, but was: %#v", expectedBinaryReport2, binaryReport)
	}
}

func loadAPIAndBinary() *fuzzReportBuilder {
	binaryAddingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg"}
	apiAddingOrder := []string{"mmmm", "nnnn", "qqqq", "pppp",
		"oooo", "rrrr", "ssss"}
	builder := fuzzReportBuilder{}
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

var mockBinaryDetails = map[string]FuzzReportBinary{
	"aaaa": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "aaaa",
		BinaryType:         "skp",
	},
	"bbbb": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "bbbb",
		BinaryType:         "skp",
	},
	"cccc": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "cccc",
		BinaryType:         "skp",
	},
	"dddd": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("delta", "epsilon", 125),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "dddd",
		BinaryType:         "png",
	},
	"eeee": FuzzReportBinary{
		DebugStackTrace:    StackTrace{},
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "eeee",
		BinaryType:         "png",
	},
	"ffff": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "ffff",
		BinaryType:         "skp",
	},
	"gggg": FuzzReportBinary{
		DebugStackTrace:    makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "gggg",
		BinaryType:         "png",
	},
}

var expectedBinaryReport1 = FuzzReport{
	FuzzReportFile{
		"mock/package/alpha", 4, 0, []FuzzReportFunction{
			FuzzReportFunction{
				"beta", 3, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						16, 3, 0, []FuzzReportBinary{mockBinaryDetails["aaaa"], mockBinaryDetails["bbbb"], mockBinaryDetails["ffff"]}, nil,
					},
				},
			}, FuzzReportFunction{
				"gamma", 1, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						26, 1, 0, []FuzzReportBinary{mockBinaryDetails["cccc"]}, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/delta", 2, 0, []FuzzReportFunction{
			FuzzReportFunction{
				"epsilon", 2, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 1, 0, []FuzzReportBinary{mockBinaryDetails["gggg"]}, nil,
					},
					FuzzReportLineNumber{
						125, 1, 0, []FuzzReportBinary{mockBinaryDetails["dddd"]}, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		common.UNKNOWN_FILE, 1, 0, []FuzzReportFunction{
			FuzzReportFunction{
				common.UNKNOWN_FUNCTION, 1, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						-1, 1, 0, []FuzzReportBinary{mockBinaryDetails["eeee"]}, nil,
					},
				},
			},
		},
	},
}

var expectedBinaryReport2 = FuzzReport{
	FuzzReportFile{
		"mock/package/alpha", 4, 4, []FuzzReportFunction{
			FuzzReportFunction{
				"beta", 3, 3, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						16, 3, 3, []FuzzReportBinary{mockBinaryDetails["aaaa"], mockBinaryDetails["bbbb"], mockBinaryDetails["ffff"]}, nil,
					},
				},
			}, FuzzReportFunction{
				"gamma", 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						26, 1, 1, []FuzzReportBinary{mockBinaryDetails["cccc"]}, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		common.UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				common.UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						-1, 1, 1, []FuzzReportBinary{mockBinaryDetails["eeee"]}, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/delta", 2, 0, []FuzzReportFunction{
			FuzzReportFunction{
				"epsilon", 2, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 1, 0, []FuzzReportBinary{mockBinaryDetails["gggg"]}, nil,
					},
					FuzzReportLineNumber{
						125, 1, 0, []FuzzReportBinary{mockBinaryDetails["dddd"]}, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/zeta", 0, 2, []FuzzReportFunction{
			FuzzReportFunction{
				"theta", 0, 2, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 0, 1, nil, nil,
					},
					FuzzReportLineNumber{
						125, 0, 1, nil, nil,
					},
				},
			},
		},
	},
}

var mockAPIDetails = map[string]FuzzReportAPI{
	"mmmm": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		TestName:           "mmmm",
	},
	"nnnn": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "nnnn",
	},
	"oooo": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		TestName:           "oooo",
	},
	"pppp": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("zeta", "theta", 125),
		HumanReadableFlags: mockFlags,
		TestName:           "pppp",
	},
	"qqqq": FuzzReportAPI{
		DebugStackTrace:    StackTrace{},
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "qqqq",
	},
	"rrrr": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		TestName:           "rrrr",
	},
	"ssss": FuzzReportAPI{
		DebugStackTrace:    makeStacktrace("zeta", "theta", 122),
		ReleaseStackTrace:  StackTrace{},
		HumanReadableFlags: mockFlags,
		TestName:           "ssss",
	},
}

var expectedAPIReport1 = FuzzReport{
	FuzzReportFile{
		"mock/package/alpha", 0, 4, []FuzzReportFunction{
			FuzzReportFunction{
				"beta", 0, 3, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						16, 0, 3, nil, []FuzzReportAPI{mockAPIDetails["mmmm"], mockAPIDetails["nnnn"], mockAPIDetails["rrrr"]},
					},
				},
			}, FuzzReportFunction{
				"gamma", 0, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						26, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["oooo"]},
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/zeta", 0, 2, []FuzzReportFunction{
			FuzzReportFunction{
				"theta", 0, 2, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["ssss"]},
					},
					FuzzReportLineNumber{
						125, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["pppp"]},
					},
				},
			},
		},
	},
	FuzzReportFile{
		common.UNKNOWN_FILE, 0, 1, []FuzzReportFunction{
			FuzzReportFunction{
				common.UNKNOWN_FUNCTION, 0, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						-1, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["qqqq"]},
					},
				},
			},
		},
	},
}

var expectedAPIReport2 = FuzzReport{
	FuzzReportFile{
		"mock/package/alpha", 4, 4, []FuzzReportFunction{
			FuzzReportFunction{
				"beta", 3, 3, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						16, 3, 3, nil, []FuzzReportAPI{mockAPIDetails["mmmm"], mockAPIDetails["nnnn"], mockAPIDetails["rrrr"]},
					},
				},
			}, FuzzReportFunction{
				"gamma", 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						26, 1, 1, nil, []FuzzReportAPI{mockAPIDetails["oooo"]},
					},
				},
			},
		},
	},
	FuzzReportFile{
		common.UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				common.UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						-1, 1, 1, nil, []FuzzReportAPI{mockAPIDetails["qqqq"]},
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/delta", 2, 0, []FuzzReportFunction{
			FuzzReportFunction{
				"epsilon", 2, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 1, 0, nil, nil,
					},
					FuzzReportLineNumber{
						125, 1, 0, nil, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/zeta", 0, 2, []FuzzReportFunction{
			FuzzReportFunction{
				"theta", 0, 2, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["ssss"]},
					},
					FuzzReportLineNumber{
						125, 0, 1, nil, []FuzzReportAPI{mockAPIDetails["pppp"]},
					},
				},
			},
		},
	},
}

var expectedSummary = FuzzReport{
	FuzzReportFile{
		"mock/package/alpha", 4, 4, []FuzzReportFunction{
			FuzzReportFunction{
				"beta", 3, 3, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						16, 3, 3, nil, nil,
					},
				},
			}, FuzzReportFunction{
				"gamma", 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						26, 1, 1, nil, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		common.UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				common.UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						-1, 1, 1, nil, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/delta", 2, 0, []FuzzReportFunction{
			FuzzReportFunction{
				"epsilon", 2, 0, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 1, 0, nil, nil,
					},
					FuzzReportLineNumber{
						125, 1, 0, nil, nil,
					},
				},
			},
		},
	},
	FuzzReportFile{
		"mock/package/zeta", 0, 2, []FuzzReportFunction{
			FuzzReportFunction{
				"theta", 0, 2, []FuzzReportLineNumber{
					FuzzReportLineNumber{
						122, 0, 1, nil, nil,
					},
					FuzzReportLineNumber{
						125, 0, 1, nil, nil,
					},
				},
			},
		},
	},
}
