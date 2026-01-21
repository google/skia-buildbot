package anomalies

// Request object for the request from the anomaly table UI.
type GetAnomaliesRequest struct {
	SubName             string `json:"sheriff"`
	IncludeTriaged      bool   `json:"triaged"`
	IncludeImprovements bool   `json:"improvements"`
	QueryCursor         string `json:"anomaly_cursor"`
	Host                string `json:"host"`
	PaginationOffset    int    `json:"pagination_offset,omitempty"`
}
