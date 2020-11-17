package main

/* CLI program used for interacting with RBE-CAS. */

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	compute "google.golang.org/api/compute/v0.beta"
)

var (
	instanceFlag = flag.String("instance", "projects/chromium-swarm/instances/default_instance", "CAS instance to use")
	rootFlag     = flag.String("root", ".", "Root of the filesystem tree.")
)

func setup() (context.Context, *rbe.Client) {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(true, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}
	cas, err := rbe.NewClient(ctx, *instanceFlag, ts)
	if err != nil {
		log.Fatal(err)
	}
	return ctx, cas
}

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <upload|download|merge>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
	}
	command := args[0]
	args = args[1:]
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		log.Fatal(err)
	}

	if command == "upload" {
		ctx, cas := setup()
		digest, err := cas.Upload(ctx, rbe.NewInputSpec(root, args))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(digest)
	} else if command == "download" {
		ctx, cas := setup()
		for _, digest := range args {
			if err := cas.Download(ctx, root, digest); err != nil {
				log.Fatal(err)
			}
		}
	} else if command == "merge" {
		ctx, cas := setup()
		digest, err := cas.Merge(ctx, args)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(digest)
	} else {
		flag.Usage()
	}
}
