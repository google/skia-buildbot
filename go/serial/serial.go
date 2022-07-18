// Package to define an interface to mirror github.com/tarm/serial/Port.
package serial

// Port is an interface definition for functions implemented by serial.Port.
//
// This is used to facilitate testing.
type Port interface {
	Close() (err error)
	Flush() error
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
}
