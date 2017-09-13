package interfaces

import (
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skolo/go/censustaker/common"
)

var expected = []common.Bot{
	{MACAddress: "40:40:4C:36:80:62", Port: 21},
	{MACAddress: "40:40:4C:36:83:62", Port: 19},
	{MACAddress: "40:41:03:00:13:D8", Port: 1},
	{MACAddress: "40:41:30:01:14:D0", Port: 1},
}

func TestParsing(t *testing.T) {
	testutils.SmallTest(t)

	bots, err := parseSSHResult(strings.Split(TEST_DATA, "\n"))
	assert.NoError(t, err)

	assert.Equal(t, expected, bots)
}

func TestDeduping(t *testing.T) {
	testutils.SmallTest(t)

	deduped := dedupeBots(expected)

	assert.Len(t, deduped, 2, "Only 2 bots have unique ports")
	assert.Contains(t, deduped, common.Bot{MACAddress: "40:40:4C:36:80:62", Port: 21})
	assert.Contains(t, deduped, common.Bot{MACAddress: "40:40:4C:36:83:62", Port: 19})
}

const TEST_DATA = `VLAN ID  MAC Address         Interface              IfIndex  Status
-------  ------------------  ---------------------  -------  ------------
1        40:40:4c:36:80:62   0/21                   21       Learned
1        40:40:4C:36:83:62   0/19                   19       Learned
1        40:41:03:00:13:D8   0/1                    1        Learned
1        40:41:30:01:14:D0   0/1                    1        Learned
`
