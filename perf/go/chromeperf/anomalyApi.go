package chromeperf

const (
	AddAnomalyUrl = "https://skia-bridge-dot-chromeperf.appspot.com/anomalies/add"
)

// ChromePerfRequest provides a struct for the data that is sent over
// to chromeperf when a regression is detected.
type ChromePerfRequest struct {
	StartRevision       int32   `json:"start_revision"`
	EndRevision         int32   `json:"end_revision"`
	ProjectID           string  `json:"project_id"`
	TestPath            string  `json:"test_path"`
	IsImprovement       bool    `json:"is_improvement"`
	BotName             string  `json:"bot_name"`
	Internal            bool    `json:"internal_only"`
	MedianBeforeAnomaly float32 `json:"median_before_anomaly"`
	MedianAfterAnomaly  float32 `json:"median_after_anomaly"`
}

// ChromePerfResponse provides a struct to hold the response data
// returned by the add anomalies api.
type ChromePerfResponse struct {
	AnomalyId    string `json:"anomaly_id"`
	AlertGroupId string `json:"alert_group_id"`
}
