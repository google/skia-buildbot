package main

import (
	"crypto/tls"
	"flag"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	common.Init()

	target := flag.Arg(0)
	if target == "" {
		sklog.Fatalf("Expected <host:port> but got as argument")
	}

	// Run it in a closure so we always close the open connection.
	err := func() error {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}

		conn, err := tls.Dial("tcp", target, conf)
		if err != nil {
			return sklog.FmtErrorf("Error dialing: %s", err)
		}
		defer util.Close(conn)

		if err := conn.Handshake(); err != nil {
			return sklog.FmtErrorf("Handshake with %s failed: %s", err)
		}

		cert := conn.ConnectionState().PeerCertificates[0]
		sklog.Infof("Cert valid from %s to %s", cert.NotBefore, cert.NotAfter)
		sklog.Infof("CERT: \n%s\n", spew.Sdump(cert))
		return nil
	}()

	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
}
