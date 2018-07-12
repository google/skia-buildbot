package incident

import (
	"testing"
	"time"

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

	st := NewStore(ds.DS, []string{"ignore"}, 3)

	m := map[string]string{
		"ignore":    "pod_123",
		alerts.TYPE: alerts.TYPE_ALERTS,
		ALERT_NAME:  "BotUnemployed",
		"bot":       "skia-rpi-064",
		CATEGORY:    "infra",
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
	m[ALERT_NAME] = "BotMissing"
	delete(m, ID)
	b, err := st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, b.Active)
	assert.NotEqual(t, "", b.Key)
	assert.NotEqual(t, a.ID, b.ID)

	// Add a note.
	bb, err := st.AddNote(b.Key, Note{
		Text:   "Stuff happened.",
		Author: "fred@example.com",
		TS:     time.Now().Unix(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "Stuff happened.", bb.Notes[0].Text)
	assert.Equal(t, b.Key, bb.Key)

	// Fail to add note, bad key.
	_, err = st.AddNote("badkey", Note{})
	assert.Error(t, err)

	// Delete note, bad index.
	_, err = st.DeleteNote(b.Key, 1)
	assert.Error(t, err)

	// Delete note.
	b, err = st.DeleteNote(b.Key, 0)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(b.Notes))

	// Assign
	b, err = st.Assign(b.Key, "barney@example.org")
	assert.NoError(t, err)
	assert.Equal(t, "barney@example.org", b.Params[ASSIGNED_TO])

	// Archive
	assert.True(t, b.Active)
	b, err = st.Archive(b.Key)
	assert.NoError(t, err)
	assert.False(t, b.Active)
}

func TestIdForAlert(t *testing.T) {
	m := map[string]string{
		"__name__":   "ALERTS",
		"alertname":  "BotMissing",
		"alertstate": "firing",
		"bot":        "skia-rpi-064",
		"category":   "infra",
		"instance":   "skia-datahopper2:20000",
		"job":        "datahopper",
		"pool":       "Skia",
		"severity":   "critical",
		"swarming":   "chromium-swarm.appspot.com",
	}

	id1, err := idForAlert(m)
	assert.NoError(t, err)
	id2, err := idForAlert(m)
	assert.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func expectedLength(st *Store, expected int) ([]Incident, error) {
	var all []Incident
	err := testutils.EventuallyConsistent(3*time.Second, func() error {
		var err error
		all, err = st.GetAll()
		if err != nil {
			return err
		}
		if 1 != len(all) {
			return nil
		}
		return nil
	})
	return all, err
}

func TestHealthz(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.INCIDENT_AM, ds.INCIDENT_ACTIVE_PARENT_AM)
	defer cleanup()

	st := NewStore(ds.DS, []string{}, 2)

	m := map[string]string{
		alerts.TYPE:     alerts.TYPE_ALERTS,
		alerts.LOCATION: "skia-public",
		ALERT_NAME:      "BotUnemployed",
		"bot":           "skia-rpi-064",
		CATEGORY:        "infra",
	}

	// Add an incident.
	a, err := st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, a.Active)
	assert.NotEqual(t, "", a.Key)

	// Have 3 healthz arrivals, on the last one the Incident should be archived.
	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	all, err := expectedLength(st, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), all[0].ResolvedCount)

	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	all, err = expectedLength(st, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), all[0].ResolvedCount)

	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	all, err = expectedLength(st, 0)
	assert.NoError(t, err)

	// Now start a new incident.
	a, err = st.AlertArrival(m)
	assert.NoError(t, err)
	assert.Equal(t, true, a.Active)
	assert.NotEqual(t, "", a.Key)

	// Healthz increments ResolvedCount.
	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	all, err = expectedLength(st, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), all[0].ResolvedCount)

	// AlertArrival zeroes ResolvedCount.
	a, err = st.AlertArrival(m)
	assert.NoError(t, err)
	all, err = expectedLength(st, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), all[0].ResolvedCount)

	// Test that Healthz only increments the right incidents.
	m2 := map[string]string{
		alerts.TYPE:     alerts.TYPE_ALERTS,
		alerts.LOCATION: "skia-not-public",
		ALERT_NAME:      "BotUnemployed",
		"bot":           "skia-rpi-064",
		CATEGORY:        "infra",
	}
	a, err = st.AlertArrival(m)
	assert.NoError(t, err)
	a, err = st.AlertArrival(m2)
	assert.NoError(t, err)
	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	err = st.Healthz("skia-public")
	assert.NoError(t, err)
	all, err = expectedLength(st, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), all[0].ResolvedCount+all[1].ResolvedCount)
}
