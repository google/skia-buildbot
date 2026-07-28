package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil/browser"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var (
	gcsBucketDebug = flag.String("gcs-bucket-debug", "skia-autogardener", "Optional, GCS bucket name to upload debug information.")
	object         = flag.String("object", "", "GCS object path, eg. \"GetTaskSummary/<taskID>\"")
)

func main() {
	ctx := context.Background()
	flag.Parse()

	ts, err := google.DefaultTokenSource(ctx, auth.ScopeReadWrite)
	if err != nil {
		sklog.Fatal(err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}

	selectedObject := *object
	if selectedObject == "" {
		gcsFs := gcs.NewFS(ctx, client, *gcsBucketDebug)
		var err error
		selectedObject, err = browser.Browse(ctx, gcsFs, "")
		if err == browser.ErrUserCanceled {
			return
		} else if err != nil {
			sklog.Fatal(err)
		}
	}

	r, err := client.Bucket(*gcsBucketDebug).Object(selectedObject).NewReader(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	var debug gemini.DebugInfo
	if err := json.NewDecoder(r).Decode(&debug); err != nil {
		sklog.Fatal(err)
	}
	tmp, err := os.MkdirTemp("", "autogardener-debug-")
	if err != nil {
		sklog.Fatal(err)
	}

	var writtenFiles []string
	writeFile := func(filename string, contents []byte) {
		p := filepath.Join(tmp, filename)
		if err := os.WriteFile(p, contents, os.ModePerm); err != nil {
			util.RemoveAll(tmp)
			sklog.Fatal(err)
		}
		writtenFiles = append(writtenFiles, p)
	}

	// Write the prompt to a file.
	writeFile("prompt.txt", []byte(debug.Prompt))

	// Attempt to decode the result as JSON and reformat it for readability.
	resultFileName := "result.txt"
	resultContents := []byte(debug.Result)
	var result interface{}
	if err := json.NewDecoder(bytes.NewReader(resultContents)).Decode(&result); err == nil {
		resultFileName = "result.json"
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			sklog.Fatal(err)
		}
		resultContents = b
	}
	writeFile(resultFileName, resultContents)

	// Write details for each tool call to files.
	for idx, toolCall := range debug.ToolCalls {
		argNames := make([]string, 0, len(toolCall.Args))
		for arg := range toolCall.Args {
			argNames = append(argNames, arg)
		}
		sort.Strings(argNames)
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "Tool: %s\nArgs:\n", toolCall.Tool)
		for _, argName := range argNames {
			argVal := toolCall.Args[argName]
			_, _ = fmt.Fprintf(&sb, "- %s = %v\n", argName, argVal)
		}
		_, _ = fmt.Fprintf(&sb, "\nResult:\n\n%s\n", toolCall.Result)

		writeFile(fmt.Sprintf("tool_call_%d.txt", idx), []byte(sb.String()))
	}

	fmt.Printf("Wrote debug info to %s:\n", tmp)
	for _, f := range writtenFiles {
		fmt.Printf("- %s\n", f)
	}
}
