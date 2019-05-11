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
	unittest.LargeTest(t)

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

func TestIdForAlert(t *testing.T) {
	unittest.LargeTest(t)
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
	st := NewStore(nil, []string{})

	id1, err := st.idForAlert(m)
	assert.NoError(t, err)
	id2, err := st.idForAlert(m)
	assert.NoError(t, err)
	assert.Equal(t, id1, id2)

	m[alerts.STATE] = alerts.STATE_ACTIVE
	id2, err = st.idForAlert(m)
	assert.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func TestGetRegexesToOwners(t *testing.T) {
	unittest.SmallTest(t)

	ownersRegexesStr := "owner1:abbr_regex1,abbr_regex2;owner2:abbr_regex3"
	m1, err := getRegexesToOwners(ownersRegexesStr)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(m1))
	assert.Equal(t, "owner1", m1["abbr_regex1"])
	assert.Equal(t, "owner1", m1["abbr_regex2"])
	assert.Equal(t, "owner2", m1["abbr_regex3"])

	// Test badly formed regex with missing owner1.
	m2, err := getRegexesToOwners("abbr_regex1,abbr2_regex;owner2:abbr_regex3")
	assert.Error(t, err)
	assert.Nil(t, m2)
}

func TestGetOwnerIfMatch(t *testing.T) {
	unittest.SmallTest(t)

	// Test matches.
	ownersRegexesStr := "superman@krypton.com:Bizarro.*,^Kryptonite.*Asteroid.*$;batman@gotham.com:Joker.*"

	ownerTest1, err := getOwnerIfMatch(ownersRegexesStr, "something Bizarro something")
	assert.NoError(t, err)
	assert.Equal(t, "superman@krypton.com", ownerTest1)

	ownerTest2, err := getOwnerIfMatch(ownersRegexesStr, "Kryptonite really big Asteroid thing")
	assert.NoError(t, err)
	assert.Equal(t, "superman@krypton.com", ownerTest2)

	ownerTest3, err := getOwnerIfMatch(ownersRegexesStr, "Joker is here!!!")
	assert.NoError(t, err)
	assert.Equal(t, "batman@gotham.com", ownerTest3)

	ownerTest4, err := getOwnerIfMatch(ownersRegexesStr, "Joker")
	assert.NoError(t, err)
	assert.Equal(t, "batman@gotham.com", ownerTest4)

	// Test misses.
	ownerMiss1, err := getOwnerIfMatch(ownersRegexesStr, "bizarro")
	assert.NoError(t, err)
	assert.Equal(t, "", ownerMiss1)

	ownerMiss2, err := getOwnerIfMatch(ownersRegexesStr, "wrong start Kryptonite really big Asteroid thing")
	assert.NoError(t, err)
	assert.Equal(t, "", ownerMiss2)

	ownerMiss3, err := getOwnerIfMatch(ownersRegexesStr, "joker")
	assert.NoError(t, err)
	assert.Equal(t, "", ownerMiss3)

	// Test badly formed regex.
	badRegex := "superman@krypton:.***"
	ownerBadTest, err := getOwnerIfMatch(badRegex, "Anything")
	assert.Error(t, err)
	assert.Equal(t, "", ownerBadTest)
}
