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
	gcs, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("%v\n", os.Args)
	r, err := gcs.Bucket(*gcsBucketDebug).Object(*object).NewReader(ctx)
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

	// Write the prompt to a file.
	if err := os.WriteFile(filepath.Join(tmp, "prompt.txt"), []byte(debug.Prompt), os.ModePerm); err != nil {
		util.RemoveAll(tmp)
		sklog.Fatal(err)
	}

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
	if err := os.WriteFile(filepath.Join(tmp, resultFileName), resultContents, os.ModePerm); err != nil {
		util.RemoveAll(tmp)
		sklog.Fatal(err)
	}

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

		if err := os.WriteFile(filepath.Join(tmp, fmt.Sprintf("tool_call_%d.txt", idx)), []byte(sb.String()), os.ModePerm); err != nil {
			util.RemoveAll(tmp)
			sklog.Fatal(err)
		}
	}
	fmt.Printf("Wrote debug info to %s\n", tmp)
}
