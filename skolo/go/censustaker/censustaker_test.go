package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/skolo/go/powercycle"
)

func TestMakeConfig_Success(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Arp_ReturnsTable",
		"Test_FakeExe_EdgeSwitch_ReturnsTable",
	)

	out, err := makeConfig(ctx, fakeAddress, fakeUser, fakePassword, regexp.MustCompile("rpi"))
	require.NoError(t, err)

	// The following machines have a mac address that appears in both the arpOutput as well as
	// the edge switch output. Machine 192.168.1.100 also appears in both lists, but should not
	// show up in the list because of the "rpi" matching.
	assert.Equal(t, powercycle.EdgeSwitchConfig{
		Address:  fakeAddress,
		User:     fakeUser,
		Password: "", // intentionally blanked out
		DevPortMap: map[powercycle.DeviceID]int{
			"skia-rpi-001": 1,
			"skia-rpi-007": 7,
			"skia-rpi-011": 11,
			"skia-rpi-014": 14,
			"skia-rpi-026": 26,
			"skia-rpi-030": 30,
			"skia-rpi-034": 34,
			"skia-rpi-042": 42,
		},
	}, out)
}

func Test_FakeExe_Arp_ReturnsTable(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"arp"}, args)

	fmt.Println(arpOutput)
}

// This is real arp output from rack 1 circa April 2020. It has been edited for television and
// formatted to fit this screen.
const arpOutput = `Address                  HWtype  HWaddress           Flags Mask            Iface
skia-rpi-072             ether   b8:27:eb:bc:62:2f   C                     eno1
skia-rpi-034             ether   b8:27:eb:a5:2d:f4   C                     eno1
skia-rpi-001             ether   b8:27:eb:4f:5f:60   C                     eno1
skia-rpi-014             ether   b8:27:eb:75:41:da   C                     eno1
skia-rpi-079             ether   b8:27:eb:f7:e1:4a   C                     eno1
skia-rpi-049             ether   b8:27:eb:02:24:cd   C                     eno1
skia-rpi-011             ether   b8:27:eb:40:8a:f1   C                     eno1
skia-rpi-068             ether   b8:27:eb:f0:3c:43   C                     eno1
skia-rpi-030             ether   b8:27:eb:8e:e7:36   C                     eno1
192.168.1.100            ether   94:c6:91:18:57:d8   C                     eno1
skia-rpi-026             ether   b8:27:eb:3e:a5:9c   C                     eno1
skia-rpi-065             ether   b8:27:eb:1e:a9:95   C                     eno1
skia-rpi-045             ether   b8:27:eb:28:ad:64   C                     eno1
skia-rpi-007             ether   b8:27:eb:77:a5:1a   C                     eno1
192.168.1.39             ether   fc:ec:da:7f:11:1f   C                     eno1
skia-rpi-080             ether   b8:27:eb:cc:c9:43   C                     eno1
skia-rpi-042             ether   b8:27:eb:83:06:91   C                     eno1
skia-rpi-022             ether   b8:27:eb:80:ba:ce   C                     eno1`

func Test_FakeExe_EdgeSwitch_ReturnsTable(t *testing.T) {
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	args := executil.OriginalArgs()
	require.Equal(t, []string{"sshpass", "-p", fakePassword, "ssh", "power@192.168.1.117"}, args)

	// We expect the command to be sent over standard in once the ssh connection is established.
	input, err := ioutil.ReadAll(os.Stdin)
	require.NoError(t, err)

	assert.Equal(t, "enable\nshow mac-addr-table all\nmmmmmmmmm\n", string(input))
	fmt.Println(edgeOutput)
}

// This is mostly real edgeswitch output from rack 1 circa April 2020. Some entries have been
// removed or replaced. Note that a lot of things claim to be on port 45 because that's what
// connects switch 1 to switch 2.
const edgeOutput = `____    _
| ____|__| | __ _  ___          (c) 2010-2019
|  _| / _  |/ _  |/ _ \         Ubiquiti Networks, Inc.
| |__| (_| | (_| |  __/
|_____\__._|\__. |\___|         https://www.ui.com
|___/

Welcome to EdgeSwitch

By logging in, accessing or using Ubiquiti Inc. (UI) products, you
acknowledge that you have read and understood the Ubiquiti Licence
Agreement (available in the WebUI and at https://www.ui.com/eula/)
and agree to be bound by its terms.


(rack01-shelf1-poe-switch) >enable

(rack01-shelf1-poe-switch) #show mac-addr-table all

VLAN ID  MAC Address         Interface              IfIndex  Status
-------  ------------------  ---------------------  -------  ------------
1        70:88:6B:80:B5:AF   0/45                   45       Learned
1        b8:27:eb:cc:c9:43   0/45                   45       Learned
1        70:88:6B:83:4C:03   0/45                   45       Learned
1        b8:27:eb:83:06:91   0/42                   42       Learned
1        b8:27:eb:77:a5:1a   0/7                    7        Learned
1        94:C6:91:18:57:D8   0/45                   45       Learned
1        94:c6:91:18:57:d8   0/20                   20       Learned
1        b8:27:eb:3e:a5:9c   0/45                   45       Learned
1        b8:27:eb:1e:a9:95   0/45                   45       Learned
1        b8:27:eb:3e:a5:9c   0/26                   26       Learned
1        b8:27:eb:8e:e7:36   0/30                   30       Learned
1        b8:27:eb:40:8a:f1   0/11                   11       Learned
1        b8:27:eb:a5:2d:f4   0/34                   34       Learned
1        b8:27:eb:75:41:da   0/14                   14       Learned
1        b8:27:eb:4f:5f:60   0/1                     1       Learned
1        FC:EC:DA:7F:05:01   5/1                    65       Management
1        FC:EC:DA:7F:11:1F   0/45                   45       Learned
(rack01-shelf1-poe-switch) #mm`

const (
	fakePassword = "not-the-real-password"
	fakeAddress  = "192.168.1.117"
	fakeUser     = "power"
)
