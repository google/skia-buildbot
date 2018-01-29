package main

import (
	"context"
	"flag"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs/mass_process"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/swarming_logger/go/process"
	"google.golang.org/api/option"
)

var (
	workdir = flag.String("workdir", ".", "Working directory")
)

func main() {
	common.Init()

	// Setup.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	oauthCacheFile := path.Join(wdAbs, "google_storage_token.data")
	tp := httputils.NewBackOffTransport().(*httputils.BackOffTransport)
	tp.Transport.Dial = func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, 3*time.Minute)
	}
	c, err := auth.NewClientWithTransport(true, oauthCacheFile, "", tp, storage.ScopeReadWrite)
	if err != nil {
		sklog.Fatal(err)
	}
	gcs, err := storage.NewClient(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		sklog.Fatal(err)
	}

	// Process lots of logs, distill down to one file.
	b := gcs.Bucket(process.GS_BUCKET)
	if err := mass_process.TransformMany(b, "raw", "tmp/2-grams", process.NGramsCsv, 20); err != nil {
		sklog.Fatal(err)
	}
	if err := mass_process.ReduceMany(b, "tmp/2-grams", "tmp/probabilities", &process.MarkovChainReduction{}); err != nil {
		sklog.Fatal(err)
	}

	// Download the probabilities file.
	outFile := path.Join(*workdir, "probabilities.dat")
	if err := os.RemoveAll(outFile); err != nil {
		sklog.Fatal(err)
	}
	downloads := make(chan string)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for p := range downloads {
			b, err := mass_process.ReadObj(b.Object(p))
			if err != nil {
				sklog.Fatal(err)
			}
			if err := ioutil.WriteFile(outFile, b, os.ModePerm); err != nil {
				sklog.Fatal(err)
			}
		}
	}()
	if err := mass_process.SearchObj(b, "tmp/probabilities", downloads, -1); err != nil {
		sklog.Fatal(err)
	}
	wg.Wait()
	if err := mass_process.Delete(b, "tmp"); err != nil {
		sklog.Fatal(err)
	}
}
