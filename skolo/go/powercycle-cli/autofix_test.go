package main

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseAPIResult(t *testing.T) {
	unittest.SmallTest(t)

	bots, err := getMatchingCandidates([]byte(TEST_DATA), "jumphost-rpi-01")
	assert.NoError(t, err)
	assert.Len(t, bots, 2, "There is 1 bot and 1 device that our jumphost can reboot")

	assert.Equal(t, "skia-rpi-058-device", bots[0])
	assert.Equal(t, "skia-rpi-001", bots[1])

}

const TEST_DATA = `{"list":[
{"bot_id":"skia-rpi-058","host_id":"jumphost-rpi-01","dimensions":[{"key":"android_devices","value":["1"]},{"key":"device_os","value":["O","OPR6.170623.010"]},{"key":"device_type","value":["dragon"]},{"key":"id","value":["skia-rpi-058"]},{"key":"kvm","value":["0"]},{"key":"os","value":["Android"]},{"key":"pool","value":["Skia"]},{"key":"quarantined","value":["Device Missing"]}],"status":"Device Missing","since":"2017-09-13T18:09:37.882Z","silenced":false},
{"bot_id":"skia-rpi-258","host_id":"jumphost-rpi-02","dimensions":[{"key":"android_devices","value":["1"]},{"key":"device_os","value":["O","OPR6.170623.010"]},{"key":"device_type","value":["dragon"]},{"key":"id","value":["skia-rpi-058"]},{"key":"kvm","value":["0"]},{"key":"os","value":["Android"]},{"key":"pool","value":["Skia"]},{"key":"quarantined","value":["Device Missing"]}],"status":"Device Missing","since":"2017-09-13T18:09:37.882Z","silenced":false},
{"bot_id":"skia-rpi-001","host_id":"jumphost-rpi-01","dimensions":[{"key":"android_devices","value":["1"]},{"key":"device_os","value":["O","OPR6.170623.010"]},{"key":"device_type","value":["dragon"]},{"key":"id","value":["skia-rpi-058"]},{"key":"kvm","value":["0"]},{"key":"os","value":["Android"]},{"key":"pool","value":["Skia"]}],"status":"Host Missing","since":"2017-09-13T18:09:37.882Z","silenced":false},
{"bot_id":"skia-rpi-002","host_id":"jumphost-rpi-01","dimensions":[{"key":"android_devices","value":["1"]},{"key":"device_os","value":["O","OPR6.170623.010"]},{"key":"device_type","value":["dragon"]},{"key":"id","value":["skia-rpi-058"]},{"key":"kvm","value":["0"]},{"key":"os","value":["Android"]},{"key":"pool","value":["Skia"]}],"status":"Host Missing","since":"2017-09-13T18:09:37.882Z","silenced":true}
]}`
