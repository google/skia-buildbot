package pickle

import (
	"os"

	"github.com/nlpodyssey/gopickle/pickle"
	"github.com/nlpodyssey/gopickle/types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// indexChunk defines a struct for a chunk object in the index pkl file.
type indexChunk struct {
	ChunkId        int64  `json:"chunk_id"`
	ChunkContent   string `json:"chunk_content"`
	EmbeddingIndex int    `json:"embedding_index"`
}

// indexEntry defines a struct for an entry in the index pkl file.
type indexEntry struct {
	TopicID          int64        `json:"topic_id"`
	Title            string       `json:"title"`
	Group            string       `json:"group"`
	Keywords         []string     `json:"keywords"`
	CommitCount      int          `json:"commit_count"`
	CodeContextLines int          `json:"code_context_lines"`
	Chunks           []indexChunk `json:"chunks"`
}

// PickeReader provides a struct to read the pkl file.
type PickleReader struct {
	filepath string
}

// NewPickleReader returns a new instance of the PickleReader.
func NewPickleReader(filepath string) *PickleReader {
	return &PickleReader{
		filepath: filepath,
	}
}

// Read reads the pkl file and returns the index contents.
//
// The returned map is keyed on the topicID and value is the indexEntry for the topic.
func (r *PickleReader) Read() (map[int64]indexEntry, error) {
	// 1. Open the pickle file
	f, err := os.Open(r.filepath)
	if err != nil {
		sklog.Errorf("Could not open %s: %v", r.filepath, err)
		return nil, skerr.Wrap(err)
	}
	defer f.Close()
	// 2. Decode the pickle stream
	// The result is an interface{} representing the decoded Python object.
	obj, err := pickle.Load(r.filepath)
	if err != nil {
		sklog.Errorf("Could not decode %s: %v", r.filepath, err)
		return nil, skerr.Wrap(err)
	}

	/*
		The pickled object is structured as follows.
		{
		  "name": name
		  "topics": [{
		      "topic_id": <>,
			  "title": <>,
			  "group": <>,
			  "keywords": [<>],
			  "commit_count": <>,
			  "code_context_lines": <>,
			  "chunks": [
			 	  {
			  		  "chunk_id": <>,
					  "chunk_content": <>,
					  "embedding_index": <>
				  },
				  {
			  		  "chunk_id": <>,
					  "chunk_content": <>,
					  "embedding_index": <>
				  },
			  ]
		  }]
		}
	*/
	root_dict := obj.(*types.Dict)
	indexEntries := map[int64]indexEntry{}

	var topicData interface{}
	var ok bool
	if topicData, ok = root_dict.Get("topics"); !ok {
		return nil, skerr.Fmt("Topics information not found in %s", r.filepath)
	}
	topicList := topicData.(*types.List)
	for i := 0; i < topicList.Len(); i++ {
		topicItem := topicList.Get(i).(*types.Dict)

		indexEntry := indexEntry{
			Chunks: []indexChunk{},
		}
		if topicId, ok := topicItem.Get("topic_id"); ok {
			indexEntry.TopicID = topicId.(int64)
		}
		if title, ok := topicItem.Get("title"); ok {
			indexEntry.Title = title.(string)
		}
		if group, ok := topicItem.Get("group"); ok {
			indexEntry.Group = group.(string)
		}
		if keywords, ok := topicItem.Get("keywords"); ok {
			keyWordsList := keywords.(*types.List)
			for j := 0; j < keyWordsList.Len(); j++ {
				indexEntry.Keywords = append(indexEntry.Keywords, keyWordsList.Get(j).(string))
			}
		}
		if commitCount, ok := topicItem.Get("commit_count"); ok {
			indexEntry.CommitCount = commitCount.(int)
		}
		if codeContextLines, ok := topicItem.Get("code_context_lines"); ok {
			indexEntry.CodeContextLines = codeContextLines.(int)
		}
		if chunks, ok := topicItem.Get("chunks"); ok {
			chunkList := chunks.(*types.List)
			for j := 0; j < chunkList.Len(); j++ {
				chunkItem := chunkList.Get(j).(*types.Dict)
				chunk := indexChunk{}
				if chunkId, ok := chunkItem.Get("chunk_id"); ok {
					chunk.ChunkId = chunkId.(int64)
				}
				if chunkContent, ok := chunkItem.Get("chunk_content"); ok {
					chunk.ChunkContent = chunkContent.(string)
				}
				if embeddingIndex, ok := chunkItem.Get("embedding_index"); ok {
					chunk.EmbeddingIndex = embeddingIndex.(int)
				}
				indexEntry.Chunks = append(indexEntry.Chunks, chunk)
			}
		}
		indexEntries[indexEntry.TopicID] = indexEntry
	}

	return indexEntries, nil
}
