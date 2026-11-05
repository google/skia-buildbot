// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package topicstore

import (
	"context"
	"math"
	"sort"
	"sync"

	"go.skia.org/infra/go/skerr"
)

type topicKey struct {
	repository string
	topicID    int64
}

// InMemoryTopicStore implements the TopicStore interface using an in-memory map.
type InMemoryTopicStore struct {
	mu     sync.RWMutex
	topics map[topicKey]*Topic
}

// NewInMemoryTopicStore returns a new instance of InMemoryTopicStore.
func NewInMemoryTopicStore() *InMemoryTopicStore {
	return &InMemoryTopicStore{
		topics: make(map[topicKey]*Topic),
	}
}

// WriteTopic writes the topic data into the memory.
func (s *InMemoryTopicStore) WriteTopic(ctx context.Context, topic *Topic) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topics[topicKey{repository: topic.Repository, topicID: topic.ID}] = topic
	return nil
}

// ReadTopic reads the topic information for the given topic id.
func (s *InMemoryTopicStore) ReadTopic(ctx context.Context, topicID int64, repository string) (*Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topic, ok := s.topics[topicKey{repository: repository, topicID: topicID}]
	if !ok {
		return nil, skerr.Fmt("Topic %d not found in repository %s", topicID, repository)
	}
	return topic, nil
}

// SearchTopics searches for the most relevant topics for the given query embedding.
func (s *InMemoryTopicStore) SearchTopics(ctx context.Context, queryEmbedding []float32, topicCount int, repository string) ([]*FoundTopic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type chunkWithDistance struct {
		topic    *Topic
		chunk    *TopicChunk
		distance float64
	}

	var allChunks []chunkWithDistance
	for _, topic := range s.topics {
		if repository != "" && topic.Repository != repository {
			continue
		}
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
	topicMap := make(map[topicKey]*FoundTopic)
	for _, cd := range allChunks {
		key := topicKey{repository: cd.topic.Repository, topicID: cd.topic.ID}
		if len(ret) >= topicCount && topicMap[key] == nil {
			continue
		}

		ft, ok := topicMap[key]
		if !ok {
			if len(ret) >= topicCount {
				continue
			}
			ft = &FoundTopic{
				ID:         cd.topic.ID,
				Repository: cd.topic.Repository,
				Title:      cd.topic.Title,
				Distance:   cd.distance,
				Summary:    cd.topic.Summary,
			}
			topicMap[key] = ft
			ret = append(ret, ft)
		}
		ft.Chunks = append(ft.Chunks, cd.chunk)
	}

	return ret, nil
}

func (s *InMemoryTopicStore) GetRepositories(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	repoMap := make(map[string]bool)
	for _, topic := range s.topics {
		if topic.Repository != "" {
			repoMap[topic.Repository] = true
		}
	}
	var repos []string
	for repo := range repoMap {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	return repos, nil
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
