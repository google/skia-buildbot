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
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "aaaa",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"bbbb": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "bbbb",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"cccc": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "gamma", 26),
		DebugFlags:        mockFlags,
		FuzzName:          "cccc",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"dddd": {
		DebugStackTrace:   makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace: makeStacktrace("delta", "epsilon", 125),
		DebugFlags:        mockFlags,
		FuzzName:          "dddd",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"eeee": {
		DebugStackTrace:   StackTrace{},
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "eeee",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"ffff": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "ffff",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"gggg": {
		DebugStackTrace:   makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "gggg",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_arm8",
	},
	"jjjj": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "jjjj",
		FuzzCategory:      "skpicture",
		FuzzArchitecture:  "mock_x64",
		IsGrey:            true,
	},
}

var mockAPIDetails = map[string]FuzzReport{
	"hhhh": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: makeStacktrace("alpha", "beta", 16),
		DebugFlags:        mockFlags,
		FuzzName:          "hhhh",
		FuzzCategory:      "api",
		FuzzArchitecture:  "mock_x64",
	},
	"iiii": {
		DebugStackTrace:   makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace: StackTrace{},
		DebugFlags:        mockFlags,
		FuzzName:          "iiii",
		FuzzCategory:      "api",
		FuzzArchitecture:  "mock_arm8",
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
