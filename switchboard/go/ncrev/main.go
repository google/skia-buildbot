// ncrev is essentially "nc -l NNNN", i.e. netcat listening on a port and
// directing the traffic to stdin/stdout, but it first checks that nothing is
// already listening on port NNNN.
package main

import (
	"flag"
	"io"
	"net"
	"os"
	"sync"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	port    = flag.String("port", "", `The port to listen on, e.g. ":4000"`)
	machine = flag.String("machine", "", "The name of the machine making the connection, e.g. skia-rpi2-rack4-shelf1-001.")
)

func main() {
	common.Init()

	if *port == "" {
		sklog.Fatal("Port is required.")
	}

	sklog.Infof("Machine: %s", *machine)

	sklog.Info("Checking for existing listener.")
	// First start a connection to the local port. If there is already a listener
	// there then we should fail out.
	conn, err := net.Dial("tcp", *port)
	if err == nil {
		util.Close(conn)
		sklog.Fatal("Found a listener at that port.")
	}

	sklog.Info("Begin listening.")
	// Now listen for a connection.
	ln, err := net.Listen("tcp", *port)
	if err != nil {
		sklog.Fatal("Failed to listen on port %q: %s", *port, err)
	}
	conn, err = ln.Accept()
	if err != nil {
		sklog.Fatal("Failed to accept connection on port %q: %s", *port, err)
	}

	// Once a connection is done stream the connection to stdin/stdout.
	defer util.Close(conn)

	var wg sync.WaitGroup
	wg.Add(1) // We will exit once one of the streams is closed.

	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, os.Stdin); err != nil {
			sklog.Fatal("Failed while streaming stdin to port %q: %s", *port, err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, conn); err != nil {
			sklog.Fatal("Failed while streaming stdout from port %q: %s", *port, err)
		}
	}()

	wg.Wait()
}
