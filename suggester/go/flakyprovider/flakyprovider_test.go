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
	aMinuteEarlier := now.Add(-1 * time.Minute)
	comments := []*db.RepoComments{
		&db.RepoComments{
			TaskSpecComments: map[string][]*db.TaskSpecComment{
				"Bot-1": []*db.TaskSpecComment{
					&db.TaskSpecComment{
						Timestamp: now,
						Flaky:     true,
					},
					&db.TaskSpecComment{
						Timestamp: now.Add(time.Minute),
						Flaky:     false,
					},
					&db.TaskSpecComment{
						Timestamp: aMinuteEarlier,
						Flaky:     true,
					},
				},
			},
		},
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(comments)
	assert.NoError(t, err)

	flakes, err := jsonToFlaky(&b)
	assert.NoError(t, err)
	assert.Len(t, flakes, 1)
	assert.True(t, aMinuteEarlier.Equal(flakes["Bot-1"]))
}

func TestJSONEmptyToFlaky(t *testing.T) {
	flakes, err := jsonToFlaky(bytes.NewBufferString("[]"))
	assert.NoError(t, err)
	assert.Len(t, flakes, 0)
}

func TestJSONInvalidToFlaky(t *testing.T) {
	_, err := jsonToFlaky(bytes.NewBufferString(""))
	assert.Error(t, err)
}
