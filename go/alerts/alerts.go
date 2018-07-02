package alerts

const (
	// TOPIC is the PubSub topic for alert messages.
	TOPIC = "promtheus-alerts"
)

// Result contains all the key-value pairs for each alert, for example:
//
// "metric":{
// 		"__name__":"ALERTS",
// 		"alertname":"BotUnemployed",
// 		"alertstate":"firing",
// 		"bot":"skia-rpi-064",
// 		"category":"infra",
// 		"instance":"skia-datahopper2:20000",
// 		"job":"datahopper",
// 		"pool":"Skia",
// 		"severity":"critical",
// 		"swarming":"chromium-swarm.appspot.com"
// },
type Result struct {
	Metric map[string]string `json:"metric"`
}

type Data struct {
	ResultType string   `json:"result_type"`
	Results    []Result `json:"results"`
}

// QueryResponse is the top level struct returned from querying
// the Prometheus v1 REST API.
type QueryResponse struct {
	Status string `json:"status"`
	Data   Data   `json:"data"`
}
