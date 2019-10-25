// Simple command line tool that parses logs extracted from cloud-logging, extracts
// the query strings and prints them to stdout.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// RAW_QUERY_STR contains the string that indicates the beginning of the search
	// query logged by skiacorrectness.
	RAW_QUERY_STR = "RawQuery:\""
)

var (
	logInputDir = flag.String("input_dir", "", "Directory with the input files. Can be nested.")
)

func main() {
	common.Init()

	if (*logInputDir == "") || (!fileutil.FileExists(*logInputDir)) {
		sklog.Error("Flag 'input_dir' must be set and the directory must exist.")
		os.Exit(1)
	}

	var result []string
	err := filepath.Walk(*logInputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		queries, err := extractQueries(path)
		if err != nil {
			return err
		}

		result = append(result, queries...)
		return nil
	})
	if err != nil {
		sklog.Fatalf("Err: %s", err)
	}

	for _, q := range result {
		fmt.Println(q)
	}
}

type LogEntry struct {
	TextPayload string
}

func extractQueries(path string) ([]string, error) {
	if strings.HasSuffix(path, ".json") {
		fmt.Println("Path: " + path)
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer util.Close(file)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			rec := LogEntry{}
			recText := scanner.Text()
			if err := json.Unmarshal([]byte(recText), &rec); err != nil {
				return nil, err
			}

			s := rec.TextPayload
			start := strings.Index(s, RAW_QUERY_STR)
			if start != -1 {
				start += len(RAW_QUERY_STR)
				end := start
				for (end < len(s)) && (s[end] != '"') {
					end++
				}
				if s[end] == '"' {
					target := s[start:end]
					if target != "" {
						fmt.Println(target)
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
