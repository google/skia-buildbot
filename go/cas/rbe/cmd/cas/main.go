package main

/* CLI program used for interacting with RBE-CAS. */

import (
	"context"
	"flag"
	"log"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	compute "google.golang.org/api/compute/v0.beta"
)

var (
	instanceFlag = flag.String("instance", "projects/chromium-swarm/instances/default_instance", "CAS instance to use")
	rootFlag     = flag.String("root", ".", "Root of the filesystem tree.")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("Please specify a command") // TODO: Usage.
	}
	command := args[0]
	args = args[1:]

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(true, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}
	cas, err := rbe.NewClient(ctx, *instanceFlag, ts)
	if err != nil {
		log.Fatal(err)
	}

	if command == "upload" {
		digest, err := cas.Upload(ctx, root, args)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(digest)
	} else if command == "download" {
		for _, digest := range args {
			if err := cas.Download(ctx, root, digest); err != nil {
				log.Fatal(err)
			}
		}
	} else if command == "merge" {
		digest, err := cas.Merge(ctx, args)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(digest)
	} else {
		log.Fatalf("Unknown command %q", command) // TODO: Usage.
	}
}
