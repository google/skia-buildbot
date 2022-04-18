// Package netutils contains utilities to work with ports.
package netutils

import (
	"net"

	"go.skia.org/infra/go/skerr"
)

// FindUnusedTCPPort finds an unused TCP port by opening a TCP port on an unused port chosen by the
// operating system, recovering the port number and immediately closing the socket.
//
// This function does not guarantee that multiple calls will return different port numbers, so it
// might cause tests to flake out. However, the odds of this happening are low. In the future, we
// might decide to keep track of previously returned port numbers, and keep probing the OS until
// it returns a previously unseen port number.
func FindUnusedTCPPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(skerr.Wrap(err))
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		panic(skerr.Wrap(err))
	}
	return port
}
