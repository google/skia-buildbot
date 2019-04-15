package data

import "fmt"

func MockReport(category, id string) FuzzReport {
	if category == "skpicture" {
		return mockPictureDetails[id]
	}
	if category == "api" {
		return mockAPIDetails[id]
	}
	panic(fmt.Sprintf("No mock reports for category %s", category))
}

var mockFlags = []string{"foo", "bar"}

var mockPictureDetails = map[string]FuzzReport{
	"aaaa": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": makeStacktrace("alpha", "beta", 16),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "aaaa",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"bbbb": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": {},
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "bbbb",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"cccc": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": makeStacktrace("alpha", "gamma", 26),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "cccc",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"dddd": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "gamma", 43),
			"CLANG_RELEASE": makeStacktrace("delta", "epsilon", 125),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "dddd",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"eeee": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   {},
			"CLANG_RELEASE": {},
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "eeee",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"ffff": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": makeStacktrace("alpha", "beta", 16),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "ffff",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"gggg": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("delta", "epsilon", 122),
			"CLANG_RELEASE": {},
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "gggg",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_arm8",
	},
	"jjjj": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": makeStacktrace("alpha", "beta", 16),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "jjjj",
		FuzzCategory:     "skpicture",
		FuzzArchitecture: "mock_x64",
		IsGrey:           true,
	},
}

var mockAPIDetails = map[string]FuzzReport{
	"hhhh": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": makeStacktrace("alpha", "beta", 16),
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "hhhh",
		FuzzCategory:     "api",
		FuzzArchitecture: "mock_x64",
	},
	"iiii": {
		Stacktraces: map[string]StackTrace{
			"CLANG_DEBUG":   makeStacktrace("alpha", "beta", 16),
			"CLANG_RELEASE": {},
		},
		Flags:            map[string][]string{"CLANG_DEBUG": mockFlags},
		FuzzName:         "iiii",
		FuzzCategory:     "api",
		FuzzArchitecture: "mock_arm8",
	},
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
