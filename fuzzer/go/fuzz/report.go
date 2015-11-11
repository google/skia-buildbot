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
