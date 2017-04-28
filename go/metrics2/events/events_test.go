package events

import (
	"io/ioutil"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestAggregateMetric(t *testing.T) {
	testutils.MediumTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	db, err := NewEventDB(path.Join(tmp, "events.bdb"))
	assert.NoError(t, err)
	m, err := NewEventMetrics(db)
	assert.NoError(t, err)

	s := "my-events"
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	assert.NoError(t, m.db.Insert(&Event{
		Stream:    s,
		Timestamp: k1,
		Data:      encodeEvent(v1),
	}))

	period := 20 * time.Minute
	called := false
	assert.NoError(t, m.AggregateMetric(s, nil, period, func(vs []*Event) (float64, error) {
		called = true
		assert.Equal(t, 1, len(vs))
		assert.Equal(t, v1, decodeEvent(vs[0].Data))
		return 0.0, nil
	}))
	assert.False(t, called)
	assert.NoError(t, m.updateMetrics(now))
	assert.True(t, called)
}
