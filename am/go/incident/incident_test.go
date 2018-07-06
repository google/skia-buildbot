package incident

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestAlertArrival(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.INCIDENT_AM, ds.INCIDENT_ACTIVE_PARENT_AM)
	defer cleanup()

	st := NewStore(ds.DS, []string{"ignore"})

	m := map[string]string{
		"ignore":    "pod_123",
		alerts.TYPE: alerts.TYPE_ALERTS,
		"alertname": "BotUnemployed",
		"bot":       "skia-rpi-064",
		"category":  "infra",
	}
	a, err := st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, a.Active)
	assert.NotEqual(t, "", a.Key)

	// A second alert arrival with the same config doesn't create
	// a new Incident.
	a, err = st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, a.Active)
	assert.NotEqual(t, "", a.Key)

	all, err := st.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(all))

	// But an alert arrival with a different config does create
	// a new incident.
	m["alertname"] = "BotMissing"
	m[ID] = ""
	b, err := st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, b.Active)
	assert.NotEqual(t, "", b.Key)
	assert.NotEqual(t, a.ID, b.ID)

	all, err = st.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(all))
}
