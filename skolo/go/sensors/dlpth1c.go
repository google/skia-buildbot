// Package to interact with various sensor modules.
//
// A sensor module is a device that measures one or more aspects
// of the physical world (temperature, humidity, etc.).
package sensors

import (
	"fmt"
	"time"

	"github.com/tarm/serial"
	si "go.skia.org/infra/go/serial"
	"go.skia.org/infra/go/skerr"
)

const (
	pingCmd              byte = 0x22
	pingResponseVal      byte = 0x5A
	getTemperatureCmd    byte = 'T'
	getHumidityCmd       byte = 'H'
	getPressureCmd       byte = 'P'
	getTiltCmd           byte = 'A'
	getVibrationXCmd     byte = 'X'
	getVibrationYCmd     byte = 'V'
	getVibrationZCmd     byte = 'W'
	getLightCmd          byte = 'L'
	getSoundCmd          byte = 'F'
	getBroadbandSoundCmd byte = 'B'
)

// Temperature represents the temperature (°C).
type Temperature float32

func (t Temperature) String() string {
	return fmt.Sprintf("%g °C", t)
}

// Humidity represents the humidity (%RH). Value range: 0.0 → 100.0.
type Humidity float32

func (h Humidity) String() string {
	return fmt.Sprintf("%g %%", h)
}

// Pressure represents atmospheric pressure (hPa).
type Pressure float32

func (p Pressure) String() string {
	return fmt.Sprintf("%g hPa", p)
}

// LightLevel represents the ambient light level (unitless). Value range: 0.0 → 1.0.
type LightLevel float32

func (l LightLevel) String() string {
	return fmt.Sprintf("%g", l)
}

// SoundLevel represents the sound level or amplitude (dB).
type SoundLevel float32

func (s SoundLevel) String() string {
	return fmt.Sprintf("%g dB", s)
}

// Angle represents an angular measurement (degrees).
type Angle int8

// Tilt represents the angular tilt around the three axis: X, Y, and Z.
type Tilt [3]Angle

func (t Tilt) String() string {
	return fmt.Sprintf("(%d, %d, %d)", t[0], t[1], t[2])
}

// Frequency (Hz).
type Frequency int

func (f Frequency) String() string {
	return fmt.Sprintf("%d Hz", f)
}

// Peak stores a frequency and amplitude of sound measured.
type Peak struct {
	Freq      Frequency
	Amplitude SoundLevel
}

func (p Peak) String() string {
	return fmt.Sprintf("{freq: %d, amp: %g}", p.Freq, p.Amplitude)
}

// Frequencies stores a fundamental and five subharmonic peaks.
//
// Note that lower-amplitude peaks can be above or below the fundamental.
// The response is always a single fundamental followed by five peaks as
// per https://www.dlpdesign.com/DLP-TH1C-DS-V10.pdf?page=6.
type Frequencies struct {
	Fundamental Peak
	Peaks       [5]Peak
}

func (f Frequencies) String() string {
	return fmt.Sprintf("fundamental: %v, peaks: %v", f.Fundamental, f.Peaks)
}

// DLPTH1C represents an open serial connection to a DLP-TH1C sensor device.
//
// Additional sensor information available at http://www.dlpdesign.com/usb/th1c.php
type DLPTH1C struct {
	portName string
	port     si.Port
}

// NewDLPTH1C opens a serial connection to a DLP-TH1C sensor device and return a
// sensor object for device interaction.
func NewDLPTH1C(portName string) (*DLPTH1C, error) {
	c := &serial.Config{
		Name:        portName,
		Baud:        115200,
		ReadTimeout: time.Millisecond * 500,
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to open serial port")
	}

	d := DLPTH1C{portName: portName, port: s}
	return &d, nil
}

// Close the open connection to the device.
func (d *DLPTH1C) Close() error {
	return d.port.Close()
}

// Write a single byte to the device.
func (d *DLPTH1C) writeByte(b byte) error {
	buf := make([]byte, 1)
	buf[0] = b
	_, err := d.port.Write(buf)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Read the requested number of bytes from the device.
func (d *DLPTH1C) read(numBytes int) ([]byte, error) {
	ret := make([]byte, 0, numBytes)
	for len(ret) < numBytes {
		var bytesLeft = numBytes - len(ret)
		readBuf := make([]byte, bytesLeft)
		n, err := d.port.Read(readBuf)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if n == 0 {
			// Not sure if this can happen with blocking I/O.
			return nil, skerr.Fmt("eof")
		}
		ret = append(ret, readBuf[0:n]...)
	}
	return ret, nil
}

// Read a 16-bit unsigned integer value from the device.
func (d *DLPTH1C) readUInt16() (uint16, error) {
	b, err := d.read(2)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return uint16(b[0])<<8 | uint16(b[1]), nil
}

// Read a 24-bit integer value from the device.
func (d *DLPTH1C) readUInt24() (uint32, error) {
	b, err := d.read(3)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]), nil
}

// Read a 32-bit unsigned integer value from the device.
func (d *DLPTH1C) readUInt32() (uint32, error) {
	b, err := d.read(4)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// Read an array of 16-bit unsigned int values from the device.
func (d *DLPTH1C) readUInt16Array(numValues int) ([]uint16, error) {
	data, err := d.read(2 * numValues)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var idx = 0
	ret := make([]uint16, numValues)
	for i := 0; i < numValues; i++ {
		ret[i] = uint16(data[idx])<<8 | uint16(data[idx+1])
		idx += 2
	}
	return ret, nil
}

// Read a set of six frequency/amplitude pairs from the device.
func (d *DLPTH1C) readFrequencies(cmd byte) (Frequencies, error) {
	freqs := Frequencies{}
	err := d.writeByte(cmd)
	if err != nil {
		return freqs, skerr.Wrap(err)
	}
	vals, err := d.readUInt16Array(12)
	if err != nil {
		return freqs, skerr.Wrap(err)
	}

	const divisor float32 = 100.0
	freqs.Fundamental.Freq = Frequency(vals[0])
	freqs.Fundamental.Amplitude = SoundLevel(float32(vals[1]) / divisor)
	// Copy the subharmonics. There are always five subharmonic frequencies.
	// Start reading after the fundamental. Each frequency/amplitude is
	// stored in a pair of bytes.
	readPos := 2
	for i := 0; i < 5; i++ {
		freqs.Peaks[i].Freq = Frequency(vals[readPos])
		readPos++
		freqs.Peaks[i].Amplitude = SoundLevel(float32(vals[readPos]) / divisor)
		readPos++
	}
	return freqs, nil
}

// Read a single byte from the device.
func (d *DLPTH1C) readByte() (byte, error) {
	buf := make([]byte, 1)
	n, err := d.port.Read(buf)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if n != 1 {
		return 0, skerr.Fmt("incorrect read size")
	}
	return buf[0], nil
}

// Ping the connected device and verify response.
//
// Note: a device that is disconnected before its response value has been
// fully read may still have one byte in its internal response buffer.
// The first ping may fail in this case. It may be necessary to ping
// more than once to reset the device to a known state ready to receive
// new commands. ConfirmConnection can be used to perform multiple pings.
func (d *DLPTH1C) Ping() error {
	err := d.writeByte(pingCmd)
	if err != nil {
		return skerr.Wrapf(err, "failed to write byte")
	}
	b, err := d.readByte()
	if err != nil {
		return skerr.Wrapf(err, "failed to read byte")
	}
	if b != pingResponseVal {
		return skerr.Fmt("incorrect ping response; expected 0x%x, actual 0x%x", pingResponseVal, b)
	}
	return nil
}

// ConfirmConnection will confirm a good connection to the device
// by performing up to a specified maximum number of pings. If all fail,
// the error returned by the final call to Ping() will be returned.
func (d *DLPTH1C) ConfirmConnection(maxPingCount int) error {
	if maxPingCount < 1 {
		return skerr.Fmt("invalid max ping count: %d", maxPingCount)
	}
	var err error = nil
	for i := 0; i < maxPingCount; i++ {
		err = d.Ping()
		if err == nil {
			return nil
		}
	}
	return skerr.Wrapf(err, "unable to ping in %d attempts", maxPingCount)
}

// GetTemperature retrieves the current temperature value.
func (d *DLPTH1C) GetTemperature() (Temperature, error) {
	err := d.writeByte(getTemperatureCmd)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to write command")
	}
	st, err := d.readUInt16()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to read temp response")
	}
	return Temperature(float32(st) / 100.0), nil
}

// GetHumidity retrieves the current humidity value.
func (d *DLPTH1C) GetHumidity() (Humidity, error) {
	err := d.writeByte(getHumidityCmd)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to write command")
	}
	sh, err := d.readUInt24()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to read response")
	}
	return Humidity(float32(sh) / 1024.0), nil
}

// GetPressure retrieves the current pressure value.
func (d *DLPTH1C) GetPressure() (Pressure, error) {
	err := d.writeByte(getPressureCmd)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to write command")
	}
	sp, err := d.readUInt32()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to read response")
	}
	return Pressure(float32(sp) / 25600.0), nil
}

// GetTilt retrieves the current X, Y, & Z tilt values.
func (d *DLPTH1C) GetTilt() (Tilt, error) {
	var tilt Tilt
	err := d.writeByte(getTiltCmd)
	if err != nil {
		return tilt, skerr.Wrap(err)
	}
	data, err := d.read(3)
	if err != nil {
		return tilt, skerr.Wrapf(err, "failed to read response")
	}
	for i := 0; i < 3; i++ {
		tilt[i] = Angle(data[i])
	}
	return tilt, nil
}

// GetVibrationX retrieve the X-axis fundamental (peak-amplitude) frequency of
// vibration and five lower-amplitude peaks.
func (d *DLPTH1C) GetVibrationX() (Frequencies, error) {
	return d.readFrequencies(getVibrationXCmd)
}

// GetVibrationY retrieve the Y-axis fundamental (peak-amplitude) frequency of
// vibration and five lower-amplitude peaks.
func (d *DLPTH1C) GetVibrationY() (Frequencies, error) {
	return d.readFrequencies(getVibrationYCmd)
}

// GetVibrationZ retrieve the Z-axis fundamental (peak-amplitude) frequency of
// vibration and five lower-amplitude peaks.
func (d *DLPTH1C) GetVibrationZ() (Frequencies, error) {
	return d.readFrequencies(getVibrationZCmd)
}

// GetLight returns the ambient light level.
func (d *DLPTH1C) GetLight() (LightLevel, error) {
	err := d.writeByte(getLightCmd)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to write command")
	}
	v, err := d.readByte()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to read response")
	}
	return LightLevel(float32(v) / float32(255)), nil
}

// GetSound returns the fundamental (peak-amplitude) frequency of ambient sound
// and five lower-amplitude peaks.
func (d *DLPTH1C) GetSound() (Frequencies, error) {
	return d.readFrequencies(getSoundCmd)
}

// GetBroadbandSound returns the broadband ambient sound level.
func (d *DLPTH1C) GetBroadbandSound() (SoundLevel, error) {
	err := d.writeByte(getBroadbandSoundCmd)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to write command")
	}
	v, err := d.readUInt16()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to read response")
	}
	return SoundLevel(float32(v) / 100.0), nil
}
