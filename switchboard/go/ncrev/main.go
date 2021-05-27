package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"go.skia.org/infra/go/util"
)

func main() {
	fmt.Println("Begin")
	const port = ":4000"
	// First start a connection to the local port. If there is already a listener
	// there then we should fail out.
	conn, err := net.Dial("tcp", port)
	if err == nil {
		util.Close(conn)
		log.Fatal("Found a listener at that port.")
	}

	// Now listen for a connection.
	ln, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}
	conn, err = ln.Accept()
	if err != nil {
		log.Fatal(err)
	}

	// Once a connection is done stream the connection to stdin/stdout.
	defer util.Close(conn)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, os.Stdin); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, conn); err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
