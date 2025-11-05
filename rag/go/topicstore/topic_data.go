package topicstore

// TopicJSON represents the structure of the topic data as it appears in the JSON files.
type TopicJSON struct {
	TopicID     int64  `json:"topic_id"`
	Summary     string `json:"summary"`
	CodeContext string `json:"code_context"`
	Commits     []struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
		Date    string `json:"date"`
		Author  string `json:"author"`
	} `json:"commits"`
}
