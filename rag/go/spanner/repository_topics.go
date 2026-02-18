package spanner

// RepositoryTopics stores the high-level, queryable information for each topic per repository.
type RepositoryTopics struct {
	Repository       string   `sql:"repository STRING(256)"`
	TopicId          int64    `sql:"topic_id INT64"`
	Title            string   `sql:"title STRING(1024) NOT NULL"`
	TopicGroup       string   `sql:"topic_group STRING(256)"`
	Summary          string   `sql:"summary STRING(MAX) NOT NULL"`
	CodeContext      string   `sql:"code_context STRING(MAX) NOT NULL"`
	CodeContextLines int64    `sql:"code_context_lines INT64 NOT NULL"`
	CommitCount      int64    `sql:"commit_count INT64"`
	pk               struct{} `sql:"PRIMARY KEY (repository, topic_id)"`
}

// RepositoryTopicChunks stores the individual text chunks of a topic's summary and their corresponding vector embeddings per repository.
type RepositoryTopicChunks struct {
	Repository   string    `sql:"repository STRING(256)"`
	TopicId      int64     `sql:"topic_id INT64"`
	ChunkId      int64     `sql:"chunk_id INT64"`
	ChunkIndex   int64     `sql:"chunk_index INT64 NOT NULL"`
	ChunkContent string    `sql:"chunk_content STRING(MAX) NOT NULL"`
	Embedding    []float32 `sql:"embedding ARRAY<FLOAT32>(vector_length=>768) NOT NULL"`
	pk           struct{}  `sql:"PRIMARY KEY (repository, topic_id, chunk_id)"`
	interleave   struct{}  `sql:"INTERLEAVE IN PARENT RepositoryTopics ON DELETE CASCADE"`
	embeddingIdx struct{}  `sql:"VECTOR INDEX RepositoryTopicChunksEmbeddingIndex (embedding) OPTIONS (distance_type='COSINE')"`
}
