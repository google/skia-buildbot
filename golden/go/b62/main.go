package main

import (
	"fmt"
	"os"

	"go.skia.org/infra/golden/go/diffstore"
)

func main() {
	if !((len(os.Args) == 3) || (len(os.Args) == 2)) {
		fmt.Println("Exactly one or two arguments need to be provided.")
	}

	args := os.Args[1:]
	fmt.Printf("LEN: %d %v\n", len(args), args)
	if len(args) == 1 {
		bucket, path := diffstore.ImageIDToGCSPath(args[0])

		fmt.Printf("\n\nGCS path: gs://%s/%s\n", bucket, path)
	} else if len(args) == 2 {
		fmt.Printf("ImageID: %s\n", diffstore.GCSPathToImageID(args[0], args[1]))
	}
}
