package topicstore

// TopicJSON represents the structure of the topic data as it appears in the JSON files.
type TopicJSON struct {
	TopicID     string `json:"topic_id"`
	Summary     string `json:"summary"`
	CodeContext string `json:"code_context"`
	Commits     []struct {
		Hash     string              `json:"hash"`
		Message  string              `json:"message"`
		Date     string              `json:"date"`
		Author   string              `json:"author"`
		Metadata map[string][]string `json:"metadata"`
	} `json:"commits"`
}

// NewTopicFromJSON converts a TopicJSON object into a Topic object, ready for insertion.
// It expects the chunks to be pre-generated with embeddings.
func NewTopicFromJSON(tj *TopicJSON, chunks []*TopicChunk) *Topic {
	return &Topic{
		ID:          tj.TopicID,
		DisplayName: tj.Summary,
		Chunks:      chunks,
	}
}
