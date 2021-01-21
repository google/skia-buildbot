package incident

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAreIncidentsFlaky(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Now().Unix()

	// Add 10 incidents with duration = 10 mins and 10 with duration < 10 mins.
	incidents := []Incident{}
	for i := 0; i < 10; i++ {
		incidents = append(incidents, Incident{LastSeen: now, Start: now - 600})
		incidents = append(incidents, Incident{LastSeen: now, Start: now - 10})
	}
	// Should not be flaky because 50% are flaky not matching the specified 60%.
	assert.False(t, AreIncidentsFlaky(incidents, 10, 600, 0.60))
	// Should be flaky because 50% are flaky matching the specifeid 50%.
	assert.True(t, AreIncidentsFlaky(incidents, 10, 600, 0.50))

	// Add 4 incidents with duration = 10 mins and 6 with duration < 10 mins.
	incidents = []Incident{}
	for i := 0; i < 4; i++ {
		incidents = append(incidents, Incident{LastSeen: now, Start: now - 600})
	}
	for i := 0; i < 6; i++ {
		incidents = append(incidents, Incident{LastSeen: now, Start: now - 10})
	}
	// Should not be flaky because 60% are flaky not matching the specified 61%.
	assert.False(t, AreIncidentsFlaky(incidents, 10, 600, 0.61))
	// Should be flaky because 60% are flaky matching the specified 60%.
	assert.True(t, AreIncidentsFlaky(incidents, 10, 600, 0.60))

	// Add 9 incidents with duration < 10 mins.
	incidents = []Incident{}
	for i := 0; i < 9; i++ {
		incidents = append(incidents, Incident{LastSeen: now, Start: now - 10})
	}
	// Should not be flaky because num incidents did not meet the specified threshold of 10.
	assert.False(t, AreIncidentsFlaky(incidents, 10, 600, 1.00))
	// Should be flaky because num incidents do meet the specified threshold of 9 and 100%.
	assert.True(t, AreIncidentsFlaky(incidents, 9, 600, 1.00))
}

func TestIsSilenced(t *testing.T) {
	unittest.SmallTest(t)

	i := Incident{
		Params: map[string]string{
			"foo": "2123",
			"bar": "aa",
		},
	}

	// Test with simple silences

	silences := []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"2123"}},
		},
	}
	assert.True(t, i.IsSilenced(silences, true))

	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"abc", "xyz", "2123"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"blah": []string{"abc", "xyz", "2123"}},
		},
	}
	assert.True(t, i.IsSilenced(silences, true))

	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"abc", "xyz", "32"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"blah": []string{"abc", "xyz", "2123"}},
		},
	}
	assert.False(t, i.IsSilenced(silences, true))

	// Test with ignore.
	silences = []silence.Silence{
		{
			Active:   false,
			ParamSet: paramtools.ParamSet{"foo": []string{"2123"}},
		},
	}
	assert.False(t, i.IsSilenced(silences, true))
	assert.True(t, i.IsSilenced(silences, false))

	// Tests with regexes.

	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"2.*"}},
		},
	}
	assert.True(t, i.IsSilenced(silences, true))

	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"3.*"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"bar": []string{"aa"}},
		},
	}
	assert.True(t, i.IsSilenced(silences, true))

	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"3.*"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"bar": []string{"bb"}},
		},
	}
	assert.False(t, i.IsSilenced(silences, true))

	// Test with paramset with both regex and non-regex by adding another value to existing key.
	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"3.*"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"bar": []string{"bb", "aa", "cc"}},
		},
	}
	assert.True(t, i.IsSilenced(silences, true))

	// Test with silence that does not apply.
	silences = []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"3.*"}},
		},
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"bar": []string{"bb", "aa", "cc"}, "blah": []string{"abc"}},
		},
	}
	assert.False(t, i.IsSilenced(silences, true))
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
