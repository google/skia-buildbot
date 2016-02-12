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
