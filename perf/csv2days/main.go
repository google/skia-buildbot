// Command line application to filter the output of a CSV file downloaded from
// Perf. This application strips out duplicated columns from the same day,
// keeping only the first column to appear for each day, and then emits the
// altered CSV file on stdout.
//
// See the unit test for an example.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	in = flag.String("in", "", "input filename")

	// Matches RFC3339 dates.
	datetime = regexp.MustCompile(`^((?:(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2}(?:\.\d+)?))(Z|[\+-]\d{2}:\d{2})?)$`)
)

func removeValueFromSliceAtIndex(s []string, index int) []string {
	ret := make([]string, 0, len(s)-1)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

// Note that skipCols should be in reverse sorted order, i.e. largest to
// smallest, for this to work correctly.
func removeAllIndexesFromSlices(s []string, skipCols []int) []string {
	for _, col := range skipCols {
		s = removeValueFromSliceAtIndex(s, col)
	}
	return s
}

func main() {
	var inputFilename string

	// Create and parse flags.
	fs := flag.NewFlagSet("csv2days", flag.ExitOnError)
	fs.StringVar(&inputFilename, "in", "", "input filename")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		sklog.Fatal(err)
	}
	if inputFilename == "" {
		fmt.Println("The --in flag must be supplied.")
		flag.Usage()
		os.Exit(1)
	}
	err = util.WithReadFile(inputFilename, func(f io.Reader) error {
		return transformCSV(f, os.Stdout)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}

func transformCSV(input io.Reader, output io.Writer) error {
	in := csv.NewReader(input)
	out := csv.NewWriter(output)

	// Read in the header.
	header, err := in.Read()
	if err != nil {
		return skerr.Wrap(err)
	}

	// Determine which columns to drop from output.
	lastDate := ""
	skipCols := []int{}
	outHeader := []string{}
	for index, h := range header {
		if !datetime.MatchString(h) {
			outHeader = append(outHeader, h)
			continue
		}
		// Preserve just the date.
		day := h[:10]
		if day == lastDate {
			skipCols = append(skipCols, index)
		} else {
			outHeader = append(outHeader, day)
			lastDate = day
		}
	}

	err = out.Write(outHeader)
	if err != nil {
		return skerr.Wrap(err)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(skipCols)))

	for {
		record, err := in.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return skerr.Wrap(err)
		}
		record = removeAllIndexesFromSlices(record, skipCols)
		err = out.Write(record)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	out.Flush()
	if out.Error() != nil {
		return skerr.Wrap(err)
	}
	return nil
}
