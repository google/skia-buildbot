package reminder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/paramtools"
)

func TestGetDailyNextTickDuration(t *testing.T) {

	fakeNow := time.Date(2011, 11, 30, 16, 0, 0, 0, time.UTC)
	oneHourLater := time.Date(2011, 11, 30, 17, 0, 0, 0, time.UTC).Hour()
	assert.Equal(t, time.Hour, getDailyNextTickDuration(fakeNow, oneHourLater))

	fakeNow = time.Date(2011, 11, 30, 16, 0, 0, 0, time.UTC)
	oneHourEarlier := time.Date(2011, 11, 30, 15, 0, 0, 0, time.UTC).Hour()
	assert.Equal(t, 23*time.Hour, getDailyNextTickDuration(fakeNow, oneHourEarlier))

	fakeNow = time.Date(2011, 11, 30, 15, 55, 0, 0, time.UTC)
	fiveMinsLater := time.Date(2011, 11, 30, 16, 0, 0, 0, time.UTC).Hour()
	assert.Equal(t, 5*time.Minute, getDailyNextTickDuration(fakeNow, fiveMinsLater))

	fakeNow = time.Date(2011, 11, 30, 16, 05, 0, 0, time.UTC)
	fiveMinsEarlier := time.Date(2011, 11, 30, 16, 0, 0, 0, time.UTC).Hour()
	assert.Equal(t, 23*time.Hour+55*time.Minute, getDailyNextTickDuration(fakeNow, fiveMinsEarlier))
}

func TestGetOwnersToAlerts(t *testing.T) {

	supermanAssignedAlert := incident.Incident{
		Params: map[string]string{
			"foo":         "2123",
			"assigned_to": "superman@krypton.com",
		},
	}
	supermanOwnedAlert := incident.Incident{
		Params: map[string]string{
			"foo":   "2123",
			"owner": "superman@krypton.com",
		},
	}
	batmanAssignedAlert := incident.Incident{
		Params: map[string]string{
			"foo":   "333",
			"owner": "batman@gotham.com",
		},
	}
	unOwnedAlert := incident.Incident{
		Params: map[string]string{
			"foo": "333",
		},
	}
	incidents := []incident.Incident{supermanAssignedAlert, supermanOwnedAlert, batmanAssignedAlert, unOwnedAlert}
	ownersToAlerts := getOwnersToAlerts(incidents, nil)
	assert.Equal(t, 2, len(ownersToAlerts))
	assert.Equal(t, 2, len(ownersToAlerts["superman@krypton.com"]))
	assert.Equal(t, 1, len(ownersToAlerts["batman@gotham.com"]))

	// Add a silence that applies to all but the batmanAssignedAlert and unOwnedAlert.
	silences := []silence.Silence{
		{
			Active:   true,
			ParamSet: paramtools.ParamSet{"foo": []string{"2123"}},
		},
	}
	ownersToAlerts = getOwnersToAlerts(incidents, silences)
	assert.Equal(t, 1, len(ownersToAlerts))
	assert.Equal(t, 1, len(ownersToAlerts["batman@gotham.com"]))
}
