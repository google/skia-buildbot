package topicstore

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
)

type repositoryTopicStoreImpl struct {
	// spannerClient is used to insert data into Spanner.
	spannerClient *spanner.Client

	// metric for write operations for topics.
	writeMetrics metrics2.Timer

	// metric for topic read operations.
	readMetrics metrics2.Timer

	// metric for topic search operations.
	searchMetrics metrics2.Timer
}

// NewRepositoryTopicStore returns a new TopicStore instance that uses the RepositoryTopics table.
func NewRepositoryTopicStore(spannerClient *spanner.Client) TopicStore {
	return &repositoryTopicStoreImpl{
		spannerClient: spannerClient,
		writeMetrics:  metrics2.NewTimer("history_rag_write_repository_topic"),
		readMetrics:   metrics2.NewTimer("history_rag_read_repository_topic"),
		searchMetrics: metrics2.NewTimer("history_rag_search_repository_topics"),
	}
}

// WriteTopic writes the topic data into the database.
func (s *repositoryTopicStoreImpl) WriteTopic(ctx context.Context, topic *Topic) error {
	s.writeMetrics.Start()
	defer s.writeMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.RepositoryWriteTopic")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("repository", topic.Repository))
	span.AddAttributes(trace.Int64Attribute("topic_id", topic.ID))
	span.AddAttributes(trace.Int64Attribute("chunk_count", int64(len(topic.Chunks))))

	_, err := s.spannerClient.ReadWriteTransaction(ctx, func(ctx context.Context, rwt *spanner.ReadWriteTransaction) error {
		// Check if the topic already exists.
		stmt := spanner.NewStatement("SELECT topic_id FROM RepositoryTopics WHERE repository = @repository AND topic_id = @topicID")
		stmt.Params["repository"] = topic.Repository
		stmt.Params["topicID"] = topic.ID
		existingID := int64(-1)
		err := rwt.Query(ctx, stmt).Do(func(r *spanner.Row) error {
			return r.ColumnByName("topic_id", &existingID)
		})
		if err != nil {
			return skerr.Wrap(err)
		}
		topicID := topic.ID
		repository := topic.Repository

		var mutations []*spanner.Mutation
		if existingID == -1 {
			// Not found, insert
			m := spanner.InsertMap("RepositoryTopics", map[string]interface{}{
				"repository":         repository,
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
			mutations = append(mutations, spanner.Delete("RepositoryTopicChunks", spanner.KeyRange{
				Start: spanner.Key{repository, topicID},
				End:   spanner.Key{repository, topicID},
				Kind:  spanner.ClosedClosed,
			}))
			m := spanner.UpdateMap("RepositoryTopics", map[string]interface{}{
				"repository":         repository,
				"topic_id":           topicID,
				"title":              topic.Title,
				"topic_group":        topic.TopicGroup,
				"commit_count":       topic.CommitCount,
				"summary":            topic.Summary,
				"code_context":       topic.CodeContext,
				"code_context_lines": topic.CodeContextLines,
			})
			mutations = append(mutations, m)
		}

		// Insert new topic chunks.
		for _, chunk := range topic.Chunks {
			chunkID := chunk.ID
			m := spanner.InsertMap("RepositoryTopicChunks", map[string]interface{}{
				"repository":    repository,
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
	return skerr.Wrap(err)
}

// ReadTopic returns the topic data for the topic id provided.
func (s *repositoryTopicStoreImpl) ReadTopic(ctx context.Context, topicID int64) (*Topic, error) {
	s.readMetrics.Start()
	defer s.readMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.RepositoryReadTopic")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("topic_id", topicID))

	ret := &Topic{
		ID: topicID,
	}
	stmt := spanner.NewStatement(`
		SELECT
			t1.repository,
			t1.title,
			t1.summary,
			t1.code_context,
			t1.code_context_lines,
			t1.commit_count
		FROM RepositoryTopics AS t1
		WHERE t1.topic_id = @topicID
		LIMIT 1
	`)
	stmt.Params["topicID"] = topicID
	var topicPopulated bool
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		if !topicPopulated {
			if err := r.ColumnByName("repository", &ret.Repository); err != nil {
				return skerr.Wrap(err)
			}
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
func (s *repositoryTopicStoreImpl) SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int) ([]*FoundTopic, error) {
	s.searchMetrics.Start()
	defer s.searchMetrics.Stop()

	ctx, span := trace.StartSpan(ctx, "historyrag.topicstore.RepositorySearchTopics")
	defer span.End()

	stmt := spanner.NewStatement(`
		SELECT
			t.repository,
			t.topic_id,
			t.title,
			t.summary,
			c.chunk_id AS chunk_id,
			c.chunk_content,
			c.embedding,
			COSINE_DISTANCE(c.embedding, @queryEmbedding) as distance
		FROM
			RepositoryTopicChunks AS c
		JOIN
			RepositoryTopics AS t ON c.topic_id = t.topic_id AND c.repository = t.repository
		ORDER BY
			distance
		LIMIT @topicCount
	`)
	stmt.Params["queryEmbedding"] = queryEmbedding
	stmt.Params["topicCount"] = topicCount
	var ret []*FoundTopic
	topicMap := make(map[string]*FoundTopic) // Key is repository + topicID
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		var repository string
		if err := r.ColumnByName("repository", &repository); err != nil {
			return skerr.Wrap(err)
		}
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

		key := fmt.Sprintf("%s-%d", repository, topicID)
		if _, ok := topicMap[key]; !ok {
			ft := &FoundTopic{
				ID:         topicID,
				Repository: repository,
				Title:      title,
				Distance:   distance,
				Summary:    summary,
			}
			topicMap[key] = ft
			ret = append(ret, ft)
		}
		topicMap[key].Chunks = append(topicMap[key].Chunks, &TopicChunk{
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
