package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/rag/go/blamestore"
)

type mockBlameStore struct {
	blames []*blamestore.FileBlame
}

func (m *mockBlameStore) WriteBlame(ctx context.Context, blame *blamestore.FileBlame) error {
	m.blames = append(m.blames, blame)
	return nil
}

func TestHistoryIngester_IngestBlameFileData(t *testing.T) {
	ctx := context.Background()
	mockStore := &mockBlameStore{}
	ingester := New(mockStore)

	filePath := "foo.go"
	fileContent := []byte(`{
		"version": "0.1",
		"file_hash": "f5db6789ee8942bc72a8738ba86fbc0c22c09694",
		"lines": [
			"85111c5041120c782317b207d398ce82fd161fe6",
			"a89155ae3b87878b8e71883148fd5f2a426bb349"
		]
	}`)

	err := ingester.IngestBlameFileData(ctx, filePath, fileContent)
	assert.NoError(t, err)

	assert.Len(t, mockStore.blames, 1)
	blame := mockStore.blames[0]
	assert.Equal(t, "foo.go", blame.FilePath)
	assert.Equal(t, "f5db6789ee8942bc72a8738ba86fbc0c22c09694", blame.FileHash)
	assert.Equal(t, "0.1", blame.Version)
	assert.Len(t, blame.LineBlames, 2)
	assert.Equal(t, int64(1), blame.LineBlames[0].LineNumber)
	assert.Equal(t, "85111c5041120c782317b207d398ce82fd161fe6", blame.LineBlames[0].CommitHash)
	assert.Equal(t, int64(2), blame.LineBlames[1].LineNumber)
	assert.Equal(t, "a89155ae3b87878b8e71883148fd5f2a426bb349", blame.LineBlames[1].CommitHash)
}
