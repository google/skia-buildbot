package topicstore

import (
	"context"
	"math"
	"sort"
	"sync"

	"go.skia.org/infra/go/skerr"
)

// InMemoryTopicStore implements the TopicStore interface using an in-memory map.
type InMemoryTopicStore struct {
	mu     sync.RWMutex
	topics map[int64]*Topic
}

// NewInMemoryTopicStore returns a new instance of InMemoryTopicStore.
func NewInMemoryTopicStore() *InMemoryTopicStore {
	return &InMemoryTopicStore{
		topics: make(map[int64]*Topic),
	}
}

// WriteTopic writes the topic data into the memory.
func (s *InMemoryTopicStore) WriteTopic(ctx context.Context, topic *Topic) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topics[topic.ID] = topic
	return nil
}

// ReadTopic reads the topic information for the given topic id.
func (s *InMemoryTopicStore) ReadTopic(ctx context.Context, topicID int64) (*Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topic, ok := s.topics[topicID]
	if !ok {
		return nil, skerr.Fmt("Topic %d not found", topicID)
	}
	return topic, nil
}

// SearchTopics searches for the most relevant topics for the given query embedding.
func (s *InMemoryTopicStore) SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int) ([]*FoundTopic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type chunkWithDistance struct {
		topic    *Topic
		chunk    *TopicChunk
		distance float64
	}

	var allChunks []chunkWithDistance
	for _, topic := range s.topics {
		for _, chunk := range topic.Chunks {
			dist := cosineDistance(queryEmbedding, chunk.Embedding)
			allChunks = append(allChunks, chunkWithDistance{
				topic:    topic,
				chunk:    chunk,
				distance: dist,
			})
		}
	}

	// Sort by distance (ascending, as lower distance means more similar).
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].distance < allChunks[j].distance
	})

	var ret []*FoundTopic
	topicMap := make(map[int64]*FoundTopic)
	for _, cd := range allChunks {
		if len(ret) >= topicCount && topicMap[cd.topic.ID] == nil {
			continue
		}

		ft, ok := topicMap[cd.topic.ID]
		if !ok {
			if len(ret) >= topicCount {
				continue
			}
			ft = &FoundTopic{
				ID:       cd.topic.ID,
				Title:    cd.topic.Title,
				Distance: cd.distance,
				Summary:  cd.topic.Summary,
			}
			topicMap[cd.topic.ID] = ft
			ret = append(ret, ft)
		}
		ft.Chunks = append(ft.Chunks, cd.chunk)
	}

	return ret, nil
}

// cosineDistance calculates the cosine distance between two vectors.
// Cosine Distance = 1 - Cosine Similarity
func cosineDistance(v1, v2 []float32) float64 {
	if len(v1) != len(v2) || len(v1) == 0 {
		return 1.0
	}
	var dotProduct, mag1, mag2 float64
	for i := range v1 {
		dotProduct += float64(v1[i]) * float64(v2[i])
		mag1 += float64(v1[i]) * float64(v1[i])
		mag2 += float64(v2[i]) * float64(v2[i])
	}
	if mag1 == 0 || mag2 == 0 {
		return 1.0
	}
	similarity := dotProduct / (math.Sqrt(mag1) * math.Sqrt(mag2))
	return 1.0 - similarity
}
