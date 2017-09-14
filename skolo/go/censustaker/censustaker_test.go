package main

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skolo/go/censustaker/bots"
	"go.skia.org/infra/skolo/go/censustaker/common"
	"go.skia.org/infra/skolo/go/censustaker/interfaces"
)

func TestMapping(t *testing.T) {
	testutils.SmallTest(t)

	mn := bots.MockBotNameGetter{}
	mp := interfaces.MockBotPortGetter{}

	mn.On("GetBotNamesAddresses").Return([]common.Bot{
		{Hostname: "rpi-001", MACAddress: "12:32", IPV4Address: "1.1.1.1"},
		{Hostname: "rpi-002", MACAddress: "AB:BA", IPV4Address: "2.2.2.2"},
		{Hostname: "rpi-003", MACAddress: "CA:11", IPV4Address: "3.3.3.3"},
	}, nil)
	mp.On("GetBotPortsAddresses").Return([]common.Bot{
		{MACAddress: "12:32", Port: 10},
		{MACAddress: "AB:BA", Port: 20},
		{MACAddress: "not_seen_above", Port: 30},
	}, nil)

	defer mn.AssertExpectations(t)
	defer mp.AssertExpectations(t)

	botList, err := enumerateBots(&mn, &mp)

	assert.NoError(t, err)
	assert.Len(t, botList, 2, "Only 2 bots have everything listed")
	assert.Contains(t, botList, common.Bot{Hostname: "rpi-001", MACAddress: "12:32", Port: 10, IPV4Address: "1.1.1.1"})
	assert.Contains(t, botList, common.Bot{Hostname: "rpi-002", MACAddress: "AB:BA", Port: 20, IPV4Address: "2.2.2.2"})
}
