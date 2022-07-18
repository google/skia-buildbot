package sensors

import (
	"go.skia.org/infra/go/serial"
	"go.skia.org/infra/go/skerr"
)

// fakeSerialPort implements a serial.Port interface for test purposes. It
// returns the given read data when Read() is called, and keeps a copy
// of all writtten data when Write() is used.
//
// It tracks port state to detect (and fail) functions called after the
// port has been closed.
type fakeSerialPort struct {
	readData    []byte
	nextReadPos int
	writtenData []byte
	closed      bool
}

func (p *fakeSerialPort) Close() error {
	if p.closed {
		return skerr.Fmt("port already closed")
	}
	p.closed = true
	return nil
}

func (p *fakeSerialPort) Flush() error {
	if p.closed {
		return skerr.Fmt("port is closed")
	}
	return nil
}

func (p *fakeSerialPort) Read(b []byte) (int, error) {
	if p.closed {
		return 0, skerr.Fmt("port is closed")
	}
	bytesRemaining := len(p.readData) - p.nextReadPos
	if bytesRemaining == 0 {
		return 0, skerr.Fmt("no more data")
	}
	toRead := len(b)
	if toRead > bytesRemaining {
		toRead = bytesRemaining
	}
	for i := 0; i < toRead; i++ {
		b[i] = p.readData[p.nextReadPos]
		p.nextReadPos++
	}
	return toRead, nil
}

func (p *fakeSerialPort) Write(b []byte) (n int, err error) {
	if p.closed {
		return 0, skerr.Fmt("port is closed")
	}
	p.writtenData = append(p.writtenData, b...)
	return len(b), nil
}

func (p *fakeSerialPort) setReadData(bytes ...byte) *fakeSerialPort {
	p.nextReadPos = 0
	p.readData = bytes
	return p
}

func (p *fakeSerialPort) getWrittenData() []byte {
	return p.writtenData
}

// Make sure fakeSerialPort fulfills the serial.Port interface.
var _ serial.Port = (*fakeSerialPort)(nil)
