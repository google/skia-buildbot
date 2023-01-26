// extract_comments is a command-line application to extract comments for GMs
// from C++ source code and consolidate them into a single JSON file.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/comments/go/extract"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

// flags
var (
	dir  = flag.String("dir", "", "The directory to search for CPP files in.")
	dest = flag.String("dest", "", "The name of the file to write the JSON output to. If not supplied then it goes to stdout. The value can be a Google Storage URL and the JSON file will be written to GCS.")
)

func main() {
	common.Init()
	ctx := context.Background()
	matches, err := filepath.Glob(filepath.Join(*dir, "*.cpp"))
	if err != nil {
		sklog.Fatalf("Failed searching for files: %s", err)
	}
	comments := []*extract.GM{}
	for _, filename := range matches {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			sklog.Warningf("Failed to read file %s: %s", filename, err)
		}
		comments = append(comments, extract.Extract(string(b), filename)...)
	}
	var w io.WriteCloser = os.Stdout
	if *dest != "" {
		if strings.HasPrefix(*dest, "gs://") {
			ts, err := auth.NewDefaultJWTServiceAccountTokenSource(ctx, auth.ScopeReadWrite)
			if err != nil {
				sklog.Fatalf("Problem setting up client OAuth: %s", err)
			}
			client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
			c, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
			if err != nil {
				sklog.Fatalf("Problem authenticating: %s", err)
			}
			u, err := url.Parse(*dest)
			if err != nil {
				sklog.Fatalf("Failed to parse the destination location: %s", err)
			}
			w = c.Bucket(u.Host).Object(u.Path[1:]).NewWriter(context.Background())
		} else {
			w, err = os.Create(*dest)
			if err != nil {
				sklog.Fatalf("Failed to open destination file to write: %s", err)
			}
		}
	}
	if err := json.NewEncoder(w).Encode(comments); err != nil {
		sklog.Errorf("Failed to encode: %s", err)
	}
	if w != os.Stdout {
		util.Close(w)
	}
}
