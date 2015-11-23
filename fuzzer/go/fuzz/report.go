package fuzz

type FuzzReport []FuzzReportFile

type FuzzReportFile struct {
	FileName    string               `json:"fileName"`
	BinaryCount int                  `json:"binaryCount"`
	ApiCount    int                  `json:"apiCount"`
	Functions   []FuzzReportFunction `json:"byFunction"`
}

type FuzzReportFunction struct {
	FunctionName string                 `json:"functionName"`
	BinaryCount  int                    `json:"binaryCount"`
	ApiCount     int                    `json:"apiCount"`
	LineNumbers  []FuzzReportLineNumber `json:"byLineNumber"`
}

type FuzzReportLineNumber struct {
	LineNumber    int                `json:"lineNumber"`
	BinaryCount   int                `json:"binaryCount"`
	ApiCount      int                `json:"apiCount"`
	BinaryDetails []FuzzReportBinary `json:"binaryReports"`
}

// We make this intermediate struct so we can control the json names and disentagle the backend structure from what is displayed/visualized
type FuzzReportBinary struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`
	BadBinaryName      string     `json:"binaryName"`
	BinaryType         string     `json:"binaryType"`
}

func MockFuzzReport() FuzzReport {
	return FuzzReport{
		{
			"foo.h", 30, 0, []FuzzReportFunction{
				{
					"frizzle()", 18, 0, []FuzzReportLineNumber{
						{
							64, 17, 0, []FuzzReportBinary{},
						},
						{
							69, 1, 0, []FuzzReportBinary{},
						},
					},
				}, {
					"zizzle()", 12, 0, []FuzzReportLineNumber{
						{
							123, 12, 0, []FuzzReportBinary{},
						},
					},
				},
			},
		}, {
			"bar.h", 15, 3, []FuzzReportFunction{
				{
					"frizzle()", 15, 3, []FuzzReportLineNumber{
						{
							566, 15, 2, []FuzzReportBinary{},
						},
						{
							568, 0, 1, []FuzzReportBinary{},
						},
					},
				},
			},
		},
	}
}

func MockFuzzReportFileWithBinary() FuzzReportFile {
	mockBinaryFuzz := FuzzReportBinary{
		DebugStackTrace: StackTrace{
			Frames: []StackTraceFrame{
				BasicStackFrame("src/core/", "SkReadBuffer.cpp", 344),
				BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
			},
		},
		ReleaseStackTrace: StackTrace{
			Frames: []StackTraceFrame{
				BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
				BasicStackFrame("src/core/", "SkReadBuffer.h", 136),
				BasicStackFrame("src/core/", "SkPaint.cpp", 1971),
				BasicStackFrame("src/core/", "SkReadBuffer.h", 126),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
				BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
				BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				BasicStackFrame("src/core/", "SkPicture.cpp", 142),
				BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
				BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 71),
			},
		},
		HumanReadableFlags: []string{"DebugDumped", "DebugAssertion", "ReleaseTimedOut"},
		BadBinaryName:      "badbeef",
		BinaryType:         "skp",
	}

	return FuzzReportFile{
		FileName:    "foo.h",
		BinaryCount: 9,
		ApiCount:    0,
		Functions: []FuzzReportFunction{
			{
				"frizzle()", 9, 0, []FuzzReportLineNumber{
					{
						64, 8, 0, []FuzzReportBinary{mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz},
					},
					{
						69, 1, 0, []FuzzReportBinary{mockBinaryFuzz},
					},
				},
			},
		},
	}
}
