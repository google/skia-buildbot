package fuzz

import (
	"reflect"
	"testing"
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
		[]StackTraceFrame{
			{
				"mock/package/",
				file,
				line,
				function,
			},
		},
	}
}

var mockFlags = []string{"foo", "bar"}

var mockBinaryDetails = map[string]FuzzReportBinary{
	"aaaa": FuzzReportBinary{
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "beta", 16),
		mockFlags,
		"aaaa",
		"skp",
	},
	"bbbb": FuzzReportBinary{
		makeStacktrace("alpha", "beta", 16),
		StackTrace{},
		mockFlags,
		"bbbb",
		"skp",
	},
	"cccc": FuzzReportBinary{
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "gamma", 26),
		mockFlags,
		"cccc",
		"skp",
	},
	"dddd": FuzzReportBinary{
		makeStacktrace("alpha", "gamma", 43),
		makeStacktrace("delta", "epsilon", 125),
		mockFlags,
		"dddd",
		"png",
	},
	"eeee": FuzzReportBinary{
		StackTrace{},
		StackTrace{},
		mockFlags,
		"eeee",
		"png",
	},
	"ffff": FuzzReportBinary{
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "beta", 16),
		mockFlags,
		"ffff",
		"skp",
	},
	"gggg": FuzzReportBinary{
		makeStacktrace("delta", "epsilon", 122),
		StackTrace{},
		mockFlags,
		"gggg",
		"png",
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
		UNKNOWN_FILE, 1, 0, []FuzzReportFunction{
			FuzzReportFunction{
				UNKNOWN_FUNCTION, 1, 0, []FuzzReportLineNumber{
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
		UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
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
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "beta", 16),
		mockFlags,
		"mmmm",
	},
	"nnnn": FuzzReportAPI{
		makeStacktrace("alpha", "beta", 16),
		StackTrace{},
		mockFlags,
		"nnnn",
	},
	"oooo": FuzzReportAPI{
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "gamma", 26),
		mockFlags,
		"oooo",
	},
	"pppp": FuzzReportAPI{
		makeStacktrace("alpha", "gamma", 43),
		makeStacktrace("zeta", "theta", 125),
		mockFlags,
		"pppp",
	},
	"qqqq": FuzzReportAPI{
		StackTrace{},
		StackTrace{},
		mockFlags,
		"qqqq",
	},
	"rrrr": FuzzReportAPI{
		makeStacktrace("alpha", "beta", 16),
		makeStacktrace("alpha", "beta", 16),
		mockFlags,
		"rrrr",
	},
	"ssss": FuzzReportAPI{
		makeStacktrace("zeta", "theta", 122),
		StackTrace{},
		mockFlags,
		"ssss",
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
		UNKNOWN_FILE, 0, 1, []FuzzReportFunction{
			FuzzReportFunction{
				UNKNOWN_FUNCTION, 0, 1, []FuzzReportLineNumber{
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
		UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
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
		UNKNOWN_FILE, 1, 1, []FuzzReportFunction{
			FuzzReportFunction{
				UNKNOWN_FUNCTION, 1, 1, []FuzzReportLineNumber{
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
