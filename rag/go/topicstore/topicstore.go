package topicstore

import (
	"context"
)

const (
	// spannerMutationLimit is the maximum number of mutations per commit.
	// From https://cloud.google.com/spanner/quotas#limits_for_creating_reading_updating_and_deleting_data
	// The official limit is 20,000. We use a slightly smaller number to be safe.
	spannerMutationLimit = 19000
)

// Topic represents a single topic.
type Topic struct {
	ID               int64
	Repository       string
	Title            string
	TopicGroup       string
	CommitCount      int
	CodeContext      string
	CodeContextLines int
	Summary          string
	Chunks           []*TopicChunk
}

// TopicChunk represents a chunk of a topic.
type TopicChunk struct {
	ID         int64
	TopicID    int64
	Chunk      string
	ChunkIndex int64
	Embedding  []float32
}

// TopicStore defines an interface for interacting with the database for any topic data.
type TopicStore interface {
	// WriteTopic writes the topic data into the database.
	WriteTopic(ctx context.Context, topic *Topic) error

	// ReadTopic reads the topic information for the given topic id.
	ReadTopic(ctx context.Context, topicID int64, repository string) (*Topic, error)

	// SearchTopics searches for the most relevant topics for the given query embedding.
	SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int, repository string) ([]*FoundTopic, error)

	// GetRepositories returns a list of all repositories in the database.
	GetRepositories(ctx context.Context) ([]string, error)
}

// FoundTopic is a struct that contains the topic information that was found in a search.
type FoundTopic struct {
	ID         int64
	Repository string
	Title      string
	Distance   float64
	Summary    string
	Chunks     []*TopicChunk
}
