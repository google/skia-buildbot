// Package netutils contains utilities to work with ports and URLs.
package netutils

import (
	"net"
	"strings"

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

// RootDomain returns the root domain, i.e. "perf.skia.org" => "skia.org".
func RootDomain(url string) string {
	// Strip off port.
	host := strings.Split(url, ":")[0]

	// Break apart the domain.
	parts := strings.Split(host, ".")

	rootDomain := parts[0]
	if len(parts) > 1 {
		rootDomain = strings.Join(parts[len(parts)-2:], ".")
	}
	return rootDomain
}
