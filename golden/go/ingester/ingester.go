package ingester

// WIP (stephana). JSON struct to consume the output of a single test run.
type RenderResult struct {
	Key     map[string]string `json:"key"`
	Md5     string            `json:"md5"`
	Options map[string]string `json:"options"`
}

type TestRunResults struct {
	GitHash string            `json:"gitHash"`
	Key     map[string]string `json:"key"`
	Results []RenderResult    `json:"results"`
}
