// Application that requests the issuetracker Discovery document.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

func main() {
	apiKey := flag.String("apikey", "", "The API Key https://pantheon.corp.google.com/apis/credentials/key/d74adcc4-7f3f-4c6f-a406-8e7ea69daed3?project=skia-public")
	flag.Parse()

	if *apiKey == "" {
		sklog.Fatal("--apikey is required")
	}
	c, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/buganizer")
	if err != nil {
		sklog.Fatal(err)
	}
	resp, err := c.Get(fmt.Sprintf("https://issuetracker.googleapis.com/$discovery/rest?version=v1&labels=GOOGLE_PUBLIC&key=%s", *apiKey))
	if err != nil {
		sklog.Fatal(err)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println(string(b))
}
