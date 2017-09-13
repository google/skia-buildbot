package bots

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skolo/go/censustaker/common"
)

func TestParsing(t *testing.T) {
	testutils.SmallTest(t)

	bots := parseAnsibleResult(TEST_DATA)

	var expected = []common.Bot{
		{Hostname: "skia-rpi-master", MACAddress: "F4:4D:30:B6:22:01", IPV4Address: "192.168.1.99"},
		{Hostname: "skia-rpi-master-spare", MACAddress: "F4:4D:30:B6:25:02", IPV4Address: "192.168.1.98"},
		{Hostname: "skia-rpi-002", MACAddress: "B8:27:EB:66:6C:03", IPV4Address: "192.168.1.102"},
		{Hostname: "skia-rpi-001", MACAddress: "B8:27:EB:16:BA:04", IPV4Address: "192.168.1.101"},
	}

	assert.Equal(t, expected, bots)
}

const TEST_DATA = `skia-rpi-master 192.168.1.99 f4:4d:30:b6:22:01
skia-rpi-master-spare 192.168.1.98 f4:4d:30:b6:25:02
skia-rpi-002 192.168.1.102 b8:27:eb:66:6c:03
skia-rpi-001 192.168.1.101 B8:27:eb:16:ba:04`
