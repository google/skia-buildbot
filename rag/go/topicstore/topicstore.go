package topicstore

import (
	"context"

	"cloud.google.com/go/spanner"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	SearchTopics(ctx context.Context, queryEmbedding []float32) ([]*FoundTopic, error)
}

// FoundTopic is a struct that contains the topic information that was found in a search.
type FoundTopic struct {
	ID       int64
	Title    string
	Distance float64
	Chunks   []*TopicChunk
}

type topicStoreImpl struct {
	// spannerClient is used to insert data into Spanner.
	spannerClient *spanner.Client
}

// New returns a new TopicStore instance.
func New(spannerClient *spanner.Client) TopicStore {
	return &topicStoreImpl{
		spannerClient: spannerClient,
	}
}

// WriteTopic writes the topic data into the database.
func (s *topicStoreImpl) WriteTopic(ctx context.Context, topic *Topic) error {
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

		sklog.Infof("Writing topic %d", topicID)
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
			sklog.Infof("Deleting old chunks for topic")
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
	ret := &Topic{
		ID: topicID,
	}
	stmt := spanner.NewStatement(`
		SELECT
			t1.display_name,
			t2.id AS chunk_id,
			t2.chunk,
			t2.embedding
		FROM Topics AS t1
		LEFT JOIN TopicChunks AS t2 ON t1.id = t2.topic_id
		WHERE t1.id = @topicID
	`)
	stmt.Params["topicID"] = topicID
	var topicPopulated bool
	err := s.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		if !topicPopulated {
			if err := r.ColumnByName("display_name", &ret.Title); err != nil {
				return skerr.Wrap(err)
			}
			topicPopulated = true
		}

		var chunkID spanner.NullInt64
		if err := r.ColumnByName("chunk_id", &chunkID); err != nil {
			return skerr.Wrap(err)
		}
		var chunk spanner.NullString
		if err := r.ColumnByName("chunk", &chunk); err != nil {
			return skerr.Wrap(err)
		}
		var embedding []float32
		if err := r.ColumnByName("embedding", &embedding); err != nil {
			return skerr.Wrap(err)
		}

		if chunkID.Valid && chunk.Valid {
			tc := &TopicChunk{
				ID:        chunkID.Int64,
				TopicID:   topicID,
				Chunk:     chunk.StringVal,
				Embedding: embedding,
			}
			ret.Chunks = append(ret.Chunks, tc)
		}
		return nil
	})

	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if !topicPopulated {
		return nil, nil // Not found
	}

	return ret, nil
}

// SearchTopics searches for the most relevant topics for the given query embedding.
func (s *topicStoreImpl) SearchTopics(ctx context.Context, queryEmbedding []float32) ([]*FoundTopic, error) {
	stmt := spanner.NewStatement(`
		SELECT
			t.topic_id,
			t.title,
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
		LIMIT 10
	`)
	stmt.Params["queryEmbedding"] = queryEmbedding
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
