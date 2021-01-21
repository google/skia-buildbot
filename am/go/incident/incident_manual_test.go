package incident

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAlertArrival(t *testing.T) {
	unittest.ManualTest(t) // Disabled due to flaky cloud emulator on the CI

	cleanup := testutil.InitDatastore(t, ds.INCIDENT_AM, ds.INCIDENT_ACTIVE_PARENT_AM)
	defer cleanup()

	st := NewStore(ds.DS, []string{"ignore"})

	m := map[string]string{
		"ignore":     "pod_123",
		alerts.TYPE:  alerts.TYPE_ALERTS,
		alerts.STATE: alerts.STATE_ACTIVE,
		ALERT_NAME:   "BotUnemployed",
		"bot":        "skia-rpi-064",
		CATEGORY:     "infra",
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
	bb, err := st.AddNote(b.Key, note.Note{
		Text:   "Stuff happened.",
		Author: "fred@example.com",
		TS:     time.Now().Unix(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "Stuff happened.", bb.Notes[0].Text)
	assert.Equal(t, b.Key, bb.Key)

	// Fail to add note, bad key.
	_, err = st.AddNote("badkey", note.Note{})
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

	m[alerts.STATE] = alerts.STATE_RESOLVED
	b, err = st.AlertArrival(m)
	assert.NoError(t, err)
	assert.False(t, b.Active)

	// Archive
	b, err = st.Archive(b.Key)
	assert.NoError(t, err)

	recent, err := st.GetRecentlyResolvedForID(b.ID, "")
	assert.NoError(t, err)
	assert.Len(t, recent, 1)

	recent, err = st.GetRecentlyResolvedForID(b.ID, b.Key)
	assert.NoError(t, err)
	assert.Len(t, recent, 0)
}
