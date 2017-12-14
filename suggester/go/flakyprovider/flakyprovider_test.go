package flakyprovider

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/task_scheduler/go/db"
)

func TestJSONToFlaky(t *testing.T) {
	now := time.Now()
	comments := []*db.RepoComments{
		&db.RepoComments{
			TaskSpecComments: map[string][]*db.TaskSpecComment{
				"Bot-1": []*db.TaskSpecComment{
					&db.TaskSpecComment{
						Timestamp: now.Add(-5 * time.Hour),
						Flaky:     true,
					},
				},
			},
		},
	}
	var b bytes.Buffer
	err := json.NewEncoder(b).Encode(comments)
	assert.NoError(t, err)

	flakes, err := jsonToFlaky(b)
	assert.NoError(t, err)
	assert.Len(t, flakes, 1)
}
