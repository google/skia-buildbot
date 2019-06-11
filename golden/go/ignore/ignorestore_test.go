package ignore

import (
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestToQuery(t *testing.T) {
	unittest.SmallTest(t)
	queries, err := ToQuery([]*IgnoreRule{})
	assert.NoError(t, err)
	assert.Len(t, queries, 0)

	r1 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	queries, err = ToQuery([]*IgnoreRule{r1})
	assert.NoError(t, err)
	assert.Equal(t, queries[0], url.Values{"config": []string{"gpu"}})

	// A bad rule won't get converted
	r1 = NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "bad=%", "reason")
	queries, err = ToQuery([]*IgnoreRule{r1})
	assert.NotNil(t, err)
	assert.Empty(t, queries)
}
