package config

import (
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type contiansDuration struct {
	D DurationAsString
}

func TestDurationAsString_NonEmptyDuration_RoundTripsCorrectly(t *testing.T) {
	a := contiansDuration{
		D: DurationAsString(2 * time.Hour),
	}
	b, err := json.Marshal(a)
	require.NoError(t, err)

	var deserializedA contiansDuration
	err = json.Unmarshal(b, &deserializedA)
	require.NoError(t, err)
	require.Equal(t, a.D, deserializedA.D)
}

func TestDurationAsString_DeserializesShortDurationsCorrectly(t *testing.T) {
	var a contiansDuration
	err := json.Unmarshal([]byte("{\"D\":\"2h\"}"), &a)
	require.NoError(t, err)
	require.Equal(t, 2*time.Hour, time.Duration(a.D))

}

func TestDurationAsString_DeserializesEmptyDurationsCorrectly(t *testing.T) {
	var a contiansDuration
	err := json.Unmarshal([]byte("{\"D\":\"\"}"), &a)
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), time.Duration(a.D))
}
