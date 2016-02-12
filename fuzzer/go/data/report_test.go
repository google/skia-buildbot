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
		a = a.append(MockReport("skpicture", key))
	}

	b := make(SortedFuzzReports, 0, 5)
	sortedOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, MockReport("skpicture", key))
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
		builder.addFuzzReport("skpicture", MockReport("skpicture", key))
	}
	addingOrder = []string{"iiii", "hhhh"}
	for _, key := range addingOrder {
		builder.addFuzzReport("api", MockReport("api", key))
	}
	return builder
}

var expectedPictureTree = FuzzReportTree{
	FileFuzzReport{
		FileName: "mock/package/alpha", Count: 4, Functions: []FunctionFuzzReport{
			FunctionFuzzReport{
				FunctionName: "beta", Count: 3, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 16, Count: 3, Details: []FuzzReport{MockReport("skpicture", "aaaa"), MockReport("skpicture", "bbbb"), MockReport("skpicture", "ffff")},
					},
				},
			}, FunctionFuzzReport{
				FunctionName: "gamma", Count: 1, LineNumbers: []LineFuzzReport{
					LineFuzzReport{
						LineNumber: 26, Count: 1, Details: []FuzzReport{MockReport("skpicture", "cccc")},
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
						LineNumber: 122, Count: 1, Details: []FuzzReport{MockReport("skpicture", "gggg")},
					},
					LineFuzzReport{
						LineNumber: 125, Count: 1, Details: []FuzzReport{MockReport("skpicture", "dddd")},
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
						LineNumber: -1, Count: 1, Details: []FuzzReport{MockReport("skpicture", "eeee")},
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
						LineNumber: 16, Count: 2, Details: []FuzzReport{MockReport("api", "hhhh"), MockReport("api", "iiii")},
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
