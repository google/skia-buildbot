package topicstore

import (
	"context"

	"cloud.google.com/go/spanner"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
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
	ChunkIndex int
	Embedding  []float32
}

// TopicStore defines an interface for interacting with the database for any topic data.
type TopicStore interface {
	// WriteTopic writes the topic data into the database.
	WriteTopic(ctx context.Context, topic *Topic) error

	// ReadTopic reads the topic information for the given topic id.
	ReadTopic(ctx context.Context, topicID int64) (*Topic, error)

	// SearchTopics searches for the most relevant topics for the given query embedding.
	SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int) ([]*FoundTopic, error)
}

// FoundTopic is a struct that contains the topic information that was found in a search.
type FoundTopic struct {
	ID       int64
	Title    string
	Distance float64
	Summary  string
	Chunks   []*TopicChunk
}

type topicStoreImpl struct {
	// spannerClient is used to insert data into Spanner.
	spannerClient *spanner.Client

	// metric for write operations for topics.
	writeMetrics metrics2.Timer

	// metric for topic read operations.
	readMetrics metrics2.Timer

	// metric for topic search operations.
	searchMetrics metrics2.Timer
}

// New returns a new TopicStore instance.
func New(spannerClient *spanner.Client) TopicStore {
	return &topicStoreImpl{
		spannerClient: spannerClient,
		writeMetrics:  metrics2.NewTimer("history_rag_write_topic"),
		readMetrics:   metrics2.NewTimer("history_rag_read_topic"),
		searchMetrics: metrics2.NewTimer("history_rag_search_topics"),
	}
}

// WriteTopic writes the topic data into the database.
func (s *topicStoreImpl) WriteTopic(ctx context.Context, topic *Topic) error {
	s.writeMetrics.Start()
	defer s.writeMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.WriteTopic")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("topic_id", topic.ID))
	span.AddAttributes(trace.Int64Attribute("chunk_count", int64(len(topic.Chunks))))

	_, err := s.spannerClient.ReadWriteTransaction(ctx, func(ctx context.Context, rwt *spanner.ReadWriteTransaction) error {
		// Check if the topic already exists.
		stmt := spanner.NewStatement("SELECT topic_id FROM Topics WHERE topic_id = @topicID")
		stmt.Params["topicID"] = topic.ID
		existingID := int64(-1)
		err := rwt.Query(ctx, stmt).Do(func(r *spanner.Row) error {
			return r.ColumnByName("topic_id", &existingID)
		})
		if err != nil {
			return skerr.Wrap(err)
		}
		var mutations []*spanner.Mutation
		topicID := topic.ID

		if existingID == -1 {
			// Not found, insert
			m := spanner.InsertMap("Topics", map[string]interface{}{
				"topic_id":           topicID,
				"title":              topic.Title,
				"topic_group":        topic.TopicGroup,
				"commit_count":       topic.CommitCount,
				"code_context_lines": topic.CodeContextLines,
				"code_context":       topic.CodeContext,
				"Summary":            topic.Summary,
			})
			mutations = append(mutations, m)
		} else {
			// Found, update.
			// Delete old topic chunks.
			mutations = append(mutations, spanner.Delete("TopicChunks", spanner.KeyRange{
				Start: spanner.Key{topicID},
				End:   spanner.Key{topicID},
				Kind:  spanner.ClosedClosed,
			}))
			m := spanner.UpdateMap("Topics", map[string]interface{}{
				"topic_id": topicID,
				"title":    topic.Title,
			})
			mutations = append(mutations, m)
		}

		// Insert new topic chunks.
		for _, chunk := range topic.Chunks {
			chunkID := chunk.ID
			m := spanner.InsertMap("TopicChunks", map[string]interface{}{
				"chunk_id":      chunkID,
				"topic_id":      topicID,
				"chunk_content": chunk.Chunk,
				"chunk_index":   chunk.ChunkIndex,
				"embedding":     chunk.Embedding,
			})
			mutations = append(mutations, m)
			if len(mutations) >= spannerMutationLimit {
				if err := rwt.BufferWrite(mutations); err != nil {
					return skerr.Wrap(err)
				}
				mutations = nil
			}
		}

		if len(mutations) > 0 {
			return rwt.BufferWrite(mutations)
		}
		return nil
	})
	return err
}

// ReadTopic returns the topic data for the topic id provided.
func (s *topicStoreImpl) ReadTopic(ctx context.Context, topicID int64) (*Topic, error) {
	s.readMetrics.Start()
	defer s.readMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.ReadTopic")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("topic_id", topicID))

	ret := &Topic{
		ID: topicID,
	}
	stmt := spanner.NewStatement(`
		SELECT
			t1.title,
			t1.summary,
			t1.code_context,
			t1.code_context_lines,
			t1.commit_count
		FROM Topics AS t1
		WHERE t1.topic_id = @topicID
	`)
	stmt.Params["topicID"] = topicID
	var topicPopulated bool
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		if !topicPopulated {
			if err := r.ColumnByName("title", &ret.Title); err != nil {
				return skerr.Wrap(err)
			}
			topicPopulated = true
		}

		if err := r.ColumnByName("summary", &ret.Summary); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.ColumnByName("code_context", &ret.CodeContext); err != nil {
			return skerr.Wrap(err)
		}
		var codeContextLines int64
		if err := r.ColumnByName("code_context_lines", &codeContextLines); err != nil {
			return skerr.Wrap(err)
		}
		ret.CodeContextLines = int(codeContextLines)
		var commitCount int64
		if err := r.ColumnByName("commit_count", &commitCount); err != nil {
			return skerr.Wrap(err)
		}
		ret.CommitCount = int(commitCount)
		return nil
	})

	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// SearchTopics searches for the most relevant topics for the given query embedding.
func (s *topicStoreImpl) SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int) ([]*FoundTopic, error) {
	s.searchMetrics.Start()
	defer s.searchMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.SearchTopics")
	defer span.End()

	stmt := spanner.NewStatement(`
		SELECT
			t.topic_id,
			t.title,
			t.summary,
			c.chunk_id AS chunk_id,
			c.chunk_content,
			c.embedding,
			COSINE_DISTANCE(c.embedding, @queryEmbedding) as distance
		FROM
			TopicChunks AS c
		JOIN
			Topics AS t ON c.topic_id = t.topic_id
		ORDER BY
			distance
		LIMIT @topicCount
	`)
	stmt.Params["queryEmbedding"] = queryEmbedding
	stmt.Params["topicCount"] = topicCount
	var ret []*FoundTopic
	topicMap := make(map[int64]*FoundTopic)
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		var topicID int64
		if err := r.ColumnByName("topic_id", &topicID); err != nil {
			return skerr.Wrap(err)
		}
		var title string
		if err := r.ColumnByName("title", &title); err != nil {
			return skerr.Wrap(err)
		}
		var summary string
		if err := r.ColumnByName("summary", &summary); err != nil {
			return skerr.Wrap(err)
		}
		var chunkID int64
		if err := r.ColumnByName("chunk_id", &chunkID); err != nil {
			return skerr.Wrap(err)
		}
		var chunk string
		if err := r.ColumnByName("chunk_content", &chunk); err != nil {
			return skerr.Wrap(err)
		}
		var embedding []float32
		if err := r.ColumnByName("embedding", &embedding); err != nil {
			return skerr.Wrap(err)
		}
		var distance float64
		if err := r.ColumnByName("distance", &distance); err != nil {
			return skerr.Wrap(err)
		}

		if _, ok := topicMap[topicID]; !ok {
			ft := &FoundTopic{
				ID:       topicID,
				Title:    title,
				Distance: distance,
				Summary:  summary,
			}
			topicMap[topicID] = ft
			ret = append(ret, ft)
		}
		topicMap[topicID].Chunks = append(topicMap[topicID].Chunks, &TopicChunk{
			ID:        chunkID,
			TopicID:   topicID,
			Chunk:     chunk,
			Embedding: embedding,
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ret, nil
}
