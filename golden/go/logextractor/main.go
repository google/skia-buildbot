// Simple command line app the applies our image diff library to two PNGs.
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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	out = flag.String("out", "", "Filename to write the diff image to.")
)

func main() {
	defer common.LogPanic()
	common.Init()
	if flag.NArg() != 1 {
		sklog.Fatalf("Usage: %s log_export_dir", os.Args[0])
	}

	logExportDir := flag.Arg(0)
	result := []string{}
	err := filepath.Walk(logExportDir, func(path string, info os.FileInfo, err error) error {
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

			RAW_QUERY_STR := "RawQuery:\""
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
