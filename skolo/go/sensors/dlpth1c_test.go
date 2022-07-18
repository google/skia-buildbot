package sensors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/serial/mocks"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils/unittest"
)

const invalidPingResponseVal byte = pingResponseVal - 1

func deviceFromMock(p *mocks.Port) DLPTH1C {
	return DLPTH1C{
		portName: "mock",
		port:     p,
	}
}

func deviceFromFake(p *fakeSerialPort) DLPTH1C {
	return DLPTH1C{
		portName: "fake",
		port:     p,
	}
}

func TestOpen_InvalidPortName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	d, err := NewDLPTH1C("<Invalid Serial Port Name>")
	assert.Nil(t, d)
	assert.Error(t, err)
}

func TestPing_ValidDeviceResponse_Success(t *testing.T) {
	unittest.SmallTest(t)

	fsp := (&fakeSerialPort{}).setReadData(pingResponseVal)
	d := deviceFromFake(fsp)
	require.NoError(t, d.Ping())
	assert.Equal(t, []byte{pingCmd}, fsp.getWrittenData())
}

func TestPing_InvalidDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	fsp := (&fakeSerialPort{}).setReadData(invalidPingResponseVal)
	d := deviceFromFake(fsp)
	assert.Error(t, d.Ping())
	assert.Equal(t, []byte{pingCmd}, fsp.getWrittenData())
}

func TestConfirmConnection_ValidPingResponse_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, maxPings int) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			err := d.ConfirmConnection(maxPings)
			require.NoError(t, err)
		})
	}

	test("FirstTime", []byte{pingResponseVal}, 1)
	test("LastTime", []byte{invalidPingResponseVal, pingResponseVal}, 2)
}

func TestConfirmConnection_InvalidMaxPingCount_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	p := mocks.NewPort(t)
	d := deviceFromMock(p)
	assert.Error(t, d.ConfirmConnection(-1), "negative max pings")
	assert.Error(t, d.ConfirmConnection(0), "zero max pings")
}

func TestConfirmConnection_InvalidPingResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, maxPings int) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			err := d.ConfirmConnection(maxPings)
			require.Error(t, err)
		})
	}

	test("FirstTime", []byte{invalidPingResponseVal}, 1)
	test("LastTime", []byte{invalidPingResponseVal, invalidPingResponseVal}, 2)
}

func TestGetTemperature_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedTemp Temperature) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualTemp, err := d.GetTemperature()
			require.NoError(t, err)
			assert.Equal(t, expectedTemp, actualTemp)
			assert.Equal(t, []byte{getTemperatureCmd}, fsp.getWrittenData())
		})
	}

	test("MinValue", []byte{0x0, 0x0}, 0)
	test("IntermediateValue", []byte{0x10, 0x68}, Temperature(42.0))
	test("MaxValue", []byte{0xff, 0xff}, Temperature(655.35))
}

func TestGetHumidity_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedHumidity Humidity) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualHumidity, err := d.GetHumidity()
			require.NoError(t, err)
			assert.Equal(t, expectedHumidity, actualHumidity)
			assert.Equal(t, []byte{getHumidityCmd}, fsp.getWrittenData())
		})
	}

	test("MinValue", []byte{0x0, 0x0, 0x0}, 0)
	test("IntermediateValue", []byte{0x0, 0xb8, 0x00}, Humidity(46.0))
	test("MaxValue", []byte{0xff, 0xff, 0xff}, Humidity(16383.9990234375))
}

func TestGetHumidity_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil).Once()

	d := deviceFromMock(p)
	h, err := d.GetHumidity()
	assert.Error(t, err)
	assert.Zero(t, h)
}

func TestGetPressure_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil).Once()

	d := deviceFromMock(p)
	pressure, err := d.GetPressure()
	assert.Error(t, err)
	assert.Zero(t, pressure)
}

func TestGetPressure_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedPressure Pressure) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualPressure, err := d.GetPressure()
			require.NoError(t, d.Close())
			require.NoError(t, err)
			assert.Equal(t, expectedPressure, actualPressure)
			assert.Equal(t, []byte{getPressureCmd}, fsp.getWrittenData())
		})
	}

	test("MinValue", []byte{0x0, 0x0, 0x0, 0x0}, 0)
	test("IntermediateValue", []byte{0x01, 0x75, 0xd4, 0x00}, Pressure(957.0))
	test("MaxValue", []byte{0xff, 0xff, 0xff, 0xff}, Pressure(167772.159))
}

func TestGetTilt_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedTilt Tilt) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualTilt, err := d.GetTilt()
			require.NoError(t, d.Close())
			require.NoError(t, err)
			assert.Equal(t, expectedTilt, actualTilt)
			assert.Equal(t, []byte{getTiltCmd}, fsp.getWrittenData())
		})
	}

	test("ZeroValues", []byte{0x0, 0x0, 0x0}, [3]Angle{0, 0, 0})
	test("PositiveValues", []byte{0x7, 0x15, 0x7f}, [3]Angle{7, 21, 127})
	test("NegativeValues", []byte{0xf9, 0xeb, 0x80}, [3]Angle{-7, -21, -128})
	test("MaxValues", []byte{0xff, 0xff, 0xff}, [3]Angle{-1, -1, -1})
}

func TestGetTilt_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil)

	d := deviceFromMock(p)
	xyz, err := d.GetTilt()
	assert.Error(t, err)
	assert.Equal(t, xyz, Tilt([3]Angle{0, 0, 0}))
}

// The private readFrequencies() function is used by several public functions.
func TestReadFrequencies_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedFrequencies Frequencies) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualFrequencies, err := d.readFrequencies(getSoundCmd)
			require.NoError(t, d.Close())
			require.NoError(t, err)
			assert.Equal(t, expectedFrequencies, actualFrequencies)
			assert.Equal(t, []byte{getSoundCmd}, fsp.getWrittenData())
		})
	}

	test("ZeroValues", make([]byte, 24), Frequencies{
		Fundamental: Peak{},
		Peaks:       [5]Peak{},
	})
	// This data should return the same values in the Sound section (pg. 7)
	// of the datasheet.
	test("IntermediateValues", []byte{
		0x04, 0x1e, 0x1c, 0xa2, // fundamental (1054 Hz, 73.3)
		0x08, 0x16, 0x11, 0xed, // peak 2 (2070 Hz, 45.89)
		0x01, 0x11, 0x11, 0x2e, // peak 3 (273 Hz, 43.98)
		0x05, 0xa5, 0x0f, 0x6d, // peak 4 (1445 Hz, 39.49)
		0x02, 0xe6, 0x0e, 0xb9, // peak 5 (742 Hz, 37.69)
		0x06, 0xdd, 0x0e, 0x63, // peak 6 (1757 Hz, 36.83)
	}, Frequencies{
		Fundamental: Peak{Freq: 1054, Amplitude: 73.3},
		Peaks: [5]Peak{
			{Freq: 2070, Amplitude: 45.89},
			{Freq: 273, Amplitude: 43.98},
			{Freq: 1445, Amplitude: 39.49},
			{Freq: 742, Amplitude: 37.69},
			{Freq: 1757, Amplitude: 36.83}},
	})
	test("MaxValues", []byte{
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
	}, Frequencies{
		Fundamental: Peak{Freq: 65535, Amplitude: 655.35},
		Peaks: [5]Peak{
			{Freq: 65535, Amplitude: 655.35},
			{Freq: 65535, Amplitude: 655.35},
			{Freq: 65535, Amplitude: 655.35},
			{Freq: 65535, Amplitude: 655.35},
			{Freq: 65535, Amplitude: 655.35}},
	})
}

func TestReadFrequencies_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil).Once()

	d := deviceFromMock(p)
	f, err := d.readFrequencies(getSoundCmd)
	assert.Error(t, err)
	assert.Zero(t, f.Fundamental.Freq)
	assert.Zero(t, f.Fundamental.Amplitude)
	for i := 0; i < len(f.Peaks); i++ {
		assert.Zero(t, f.Peaks[i].Freq)
		assert.Zero(t, f.Peaks[i].Amplitude)
	}
}

func TestGetVibrationX_CorrectCommandSent(t *testing.T) {
	unittest.SmallTest(t)

	// This function uses the common readFrequencies() function which
	// is tested elsewhere. Only verify the correct command is being
	// sent.
	fsp := (&fakeSerialPort{}).setReadData(make([]byte, 24)...)
	d := deviceFromFake(fsp)

	_, err := d.GetVibrationX()
	require.NoError(t, err)
	assert.Equal(t, []byte{getVibrationXCmd}, fsp.getWrittenData())
}

func TestGetVibrationY_CorrectCommandSent(t *testing.T) {
	unittest.SmallTest(t)

	// This function uses the common readFrequencies() function which
	// is tested elsewhere. Only verify the correct command is being
	// sent.
	fsp := (&fakeSerialPort{}).setReadData(make([]byte, 24)...)
	d := deviceFromFake(fsp)
	_, err := d.GetVibrationY()
	require.NoError(t, err)
	assert.Equal(t, []byte{getVibrationYCmd}, fsp.getWrittenData())
}

func TestGetVibrationZ_CorrectCommandSent(t *testing.T) {
	unittest.SmallTest(t)

	// This function uses the common readFrequencies() function which
	// is tested elsewhere. Only verify the correct command is being
	// sent.
	fsp := (&fakeSerialPort{}).setReadData(make([]byte, 24)...)
	d := deviceFromFake(fsp)
	_, err := d.GetVibrationZ()
	require.NoError(t, err)
	assert.Equal(t, []byte{getVibrationZCmd}, fsp.getWrittenData())
}

func TestGetSound_CorrectCommandSent(t *testing.T) {
	unittest.SmallTest(t)

	// This function uses the common readFrequencies() function which
	// is tested elsewhere. Only verify the correct command is being
	// sent.
	fsp := (&fakeSerialPort{}).setReadData(make([]byte, 24)...)
	d := deviceFromFake(fsp)
	_, err := d.GetSound()
	require.NoError(t, err)
	assert.Equal(t, []byte{getSoundCmd}, fsp.getWrittenData())
}

func TestGetBroadbandSound_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expectedSoundLevel SoundLevel) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actualSoundLevel, err := d.GetBroadbandSound()
			require.NoError(t, d.Close())
			require.NoError(t, err)
			assert.Equal(t, expectedSoundLevel, actualSoundLevel)
			assert.Equal(t, []byte{getBroadbandSoundCmd}, fsp.getWrittenData())
		})
	}

	test("ZeroValue", []byte{0x0, 0x0}, 0)
	test("PositiveValue", []byte{0x12, 0x75}, SoundLevel(47.25))
	test("MaxValue", []byte{0xff, 0xff}, SoundLevel(655.35))
}

func TestGetBroadbandSound_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil)

	d := deviceFromMock(p)
	s, err := d.GetBroadbandSound()
	assert.Error(t, err)
	assert.Zero(t, s)
}

func TestGetLight_ValuesInRange_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, readData []byte, expected LightLevel) {
		t.Run(name, func(t *testing.T) {
			fsp := (&fakeSerialPort{}).setReadData(readData...)
			d := deviceFromFake(fsp)
			actual, err := d.GetLight()
			require.NoError(t, d.Close())
			require.NoError(t, err)
			assert.InDelta(t, float64(expected), float64(actual), 0.01)
		})
	}

	test("ZeroValue", []byte{0x0}, 0)
	test("PositiveValue", []byte{128}, LightLevel(0.5))
	test("MaxValue", []byte{0xff}, LightLevel(1.0))
}

func TestGetLight_NoDeviceResponse_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	p := mocks.NewPort(t)
	p.On("Read", mock.Anything).Return(0, skerr.Fmt("expected error"))
	p.On("Write", mock.Anything).Return(1, nil).Once()

	d := deviceFromMock(p)
	val, err := d.GetLight()
	assert.Error(t, err)
	assert.Zero(t, val)
}
