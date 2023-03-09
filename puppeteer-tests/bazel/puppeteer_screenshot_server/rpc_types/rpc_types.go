package rpc_types

type Screenshot struct {
	TestName string `json:"test_name"`
	URL      string `json:"url"`
}

type GetScreenshotsRPCResponse struct {
	ScreenshotsByApplication map[string][]Screenshot `json:"screenshots_by_application"`
}
