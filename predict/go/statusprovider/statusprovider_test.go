package statusprovider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestProcessComments(t *testing.T) {
	testutils.SmallTest(t)
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
				"Bot-2": []*db.TaskSpecComment{
					&db.TaskSpecComment{
						Timestamp:     now,
						Flaky:         false,
						IgnoreFailure: true,
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

	flakes, err := processComments(comments)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)
	assert.True(t, aMinuteEarlier.Equal(flakes["Bot-1"]))
	assert.True(t, aMinuteEarlier.Equal(flakes["Bot-2"]))
}

func TestEmpty(t *testing.T) {
	testutils.SmallTest(t)
	flakes, err := processComments([]*db.RepoComments{})
	assert.NoError(t, err)
	assert.Len(t, flakes, 0)
}
