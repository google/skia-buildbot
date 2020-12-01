// nanostat compares statistics about nanobench results.
//
// Each input file should be a JSON output file from nanobench.
//
// Invoked on a pair of input files, nanostat computes statistics for each
// file and a column showing the percent change in mean from the first to
// the second file. Next to the percent change, nanostat shows the p-value
// and sample sizes from a test of the two distributions of nanobench
// results.
//
// For example in the results below, 'p' equals 0.001 or 0.1%, and the
// analysis was done with 10 samples from the first file and 8 samples from
// the second file.
//
//              old          new  delta               stats  name
//       2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp
//
// Small p-values indicate that the two distributions are
// significantly different. If the test indicates that there was no
// significant change between the two benchmarks (defined as p > alpha),
// nanostat displays a single ~ instead of the percent change.
//
// Example
//
// Suppose we collect benchmark results from running
//
//    out/Release/nanobench --config gl 8888 --outResultsFile old.json
//
// Then make some changes to the code, recompile nanobench and run:
//
//    out/Release/nanobench --config gl 8888 --outResultsFile new.json
//
// Then nanostat summarizes the differences between the old and new runs:
//
//    $ nanostat --iqrr old.json new.json
//              old          new  delta               stats  name
//       0.78 ±  3%   0.72 ±  3%   -8%   (p=0.000, n=10+10)  desk_wowwiki.skp
//       2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp
//       3.08 ±  2%   2.96 ±  3%   -4%   (p=0.001, n= 9+10)  desk_facebook.skp
//       0.71 ±  2%   0.69 ±  3%   -3%   (p=0.028, n= 9+10)  desk_ebay.skp
//       4.59 ±  1%   4.46 ±  1%   -3%   (p=0.000, n=10+ 8)  desk_linkedin.skp
//       1.40 ±  1%   1.39 ±  0%   -1%   (p=0.011, n= 9+ 9)  desk_css3gradients.skp
//    $
//
// usage: nanostat [options] old.json new.json
// options:
//   -all
//         If true then include insignificant changes in output.
//   -alpha float
//         Consider a change significant if p < α. Must be > 0. (default 0.05)
//   -iqrr
//         If true then remove outliers in the samples using the Interquartile Rule
//   -sort order
//         Sort by order: [-]delta, [-]name (default "delta")
//   -test string
//         The type of test to do, 'utest' for Mann-Whitney U test, and 'ttest' for a Two Sample Welch T test. (default "utest")

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/samplestats"
)

const help = `
Each input file should be a JSON output file from nanobench.

Invoked on a pair of input files, nanostat computes statistics for each
file and a column showing the percent change in mean from the first to
the second file. Next to the percent change, nanostat shows the p-value
and sample sizes from a test of the two distributions of nanobench
results.

For example in the results below, 'p' equals 0.001 or 0.1%, and the
analysis was done with 10 samples from the first file and 8 samples from
the second file.

             old          new  delta               stats  name
      2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp

Small p-values indicate that the two distributions are
significantly different. If the test indicates that there was no
significant change between the two benchmarks (defined as p > alpha),
nanostat displays a single ~ instead of the percent change.

Example

Suppose we collect benchmark results from running

   out/Release/nanobench --config gl 8888 --outResultsFile old.json

Then make some changes to the code, recompile nanobench and run:

   out/Release/nanobench --config gl 8888 --outResultsFile new.json

Then nanostat summarizes the differences between the old and new runs:

   $ nanostat --iqrr old.json new.json
             old          new  delta               stats  name
      0.78 ±  3%   0.72 ±  3%   -8%   (p=0.000, n=10+10)  desk_wowwiki.skp
      2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp
      3.08 ±  2%   2.96 ±  3%   -4%   (p=0.001, n= 9+10)  desk_facebook.skp
      0.71 ±  2%   0.69 ±  3%   -3%   (p=0.028, n= 9+10)  desk_ebay.skp
      4.59 ±  1%   4.46 ±  1%   -3%   (p=0.000, n=10+ 8)  desk_linkedin.skp
      1.40 ±  1%   1.39 ±  0%   -1%   (p=0.011, n= 9+ 9)  desk_css3gradients.skp
   $
`

// sortNames maps --sort flag values to the matching Order function.
var sortNames = map[string]samplestats.Order{
	"name":  samplestats.ByName,
	"delta": samplestats.ByDelta,
}

// validTests maps --test values to the right Test type.
var validTests = map[string]samplestats.Test{
	"t-test": samplestats.TTest,
	"t":      samplestats.TTest,
	"ttest":  samplestats.TTest,
	"u-test": samplestats.UTest,
	"u":      samplestats.UTest,
	"utest":  samplestats.UTest,
}

func main() {
	actualMain(os.Stdout)
}

func actualMain(stdout io.Writer) {
	// Use a flagSet so we don't end up with the glog cluttering up the flags.
	flagSet := flag.NewFlagSet("nanostat", flag.ContinueOnError)

	flagAlpha := flagSet.Float64("alpha", 0.05, "Consider a change significant if p < α. Must be > 0.")
	flagSort := flagSet.String("sort", "delta", "Sort by `order`: [-]delta, [-]name")
	flagIQRR := flagSet.Bool("iqrr", false, "If true then remove outliers in the samples using the Interquartile Rule")
	flagAll := flagSet.Bool("all", false, "If true then include insignificant changes in output.")
	flagTest := flagSet.String("test", string(samplestats.UTest), "The type of test to do, 'utest' for Mann-Whitney U test, and 'ttest' for a Two Sample Welch T test.")

	usage := func() {
		fmt.Println(help)
		fmt.Printf("usage: nanostat [options] old.json new.json\n")
		fmt.Printf("options:\n")
		flagSet.PrintDefaults()
		os.Exit(2)
	}

	flagSet.Usage = usage

	// Ignore the output since failures will call our usage() which exits.
	_ = flagSet.Parse(os.Args[1:])

	sortName := *flagSort
	reverse := false
	if strings.HasPrefix(sortName, "-") {
		reverse = true
		sortName = sortName[1:]
	}
	order, orderOK := sortNames[sortName]
	test, testOK := validTests[*flagTest]
	if flagSet.NArg() != 2 || !orderOK || !testOK {
		usage()
	}

	if reverse {
		order = samplestats.Reverse(order)
	}

	config := samplestats.Config{
		Alpha: *flagAlpha,
		IQRR:  *flagIQRR,
		All:   *flagAll,
		Test:  test,
		Order: order,
	}
	beforeSamples := loadFileByName(flagSet.Args()[0])
	afterSamples := loadFileByName(flagSet.Args()[1])
	result := samplestats.Analyze(config, beforeSamples, afterSamples)

	if result.Skipped > 0 {
		_, err := fmt.Fprintf(stdout, "\nSkipped: %d \n", result.Skipped)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(result.Rows) > 0 {
		rowsAsTabbedStrings := formatRows(config, result.Rows)
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		for _, line := range rowsAsTabbedStrings {
			_, err := fmt.Fprintln(tw, line)
			if err != nil {
				log.Fatal(err)
			}
		}
		err := tw.Flush()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if !config.All {
			_, err := fmt.Fprintln(stdout, "No significant deltas found. Add --all to see non-significant results.")
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func formatRows(config samplestats.Config, rows []samplestats.Row) []string {
	ret := make([]string, 0, len(rows)+1)

	// Find all the keys that have more than one value. Such as 'config', which
	// might be ['gl', 'gles'], which means we have different results for each
	// config, and the config value needs to be printed when we display results.
	ps := paramtools.NewParamSet()
	for _, row := range rows {
		ps.AddParams(row.Params)
	}

	// Remove keys we know we don't want, "test", and keys we want at the end of
	// the list, "name".
	delete(ps, "test")
	delete(ps, "name")
	importantKeys := []string{}
	for key, values := range ps {
		// if a key has more than one value than it's important we display it.
		if len(values) > 1 {
			importantKeys = append(importantKeys, key)
		}
	}
	sort.Strings(importantKeys)

	// The name of the test always goes last.
	importantKeys = append(importantKeys, "name")

	header := "old\tnew\tdelta\tstats\t\t  " + strings.Join(importantKeys, "\t  ")

	ret = append(ret, header)

	for _, row := range rows {
		delta := "~"
		if !math.IsNaN(row.Delta) {
			delta = fmt.Sprintf("%.0f%%", row.Delta)
		}

		// Create the full name from all the important keys.
		fullName := []string{}
		for _, key := range importantKeys {
			fullName = append(fullName, row.Params[key])
		}
		ret = append(ret, fmt.Sprintf("%0.2f ± %2.0f%%\t%0.2f ± %2.0f%%\t%s %s\t(p=%0.3f,\tn=%d+%d)\t  %s",
			row.Samples[0].Mean,
			row.Samples[0].Percent,
			row.Samples[1].Mean,
			row.Samples[1].Percent,
			delta,
			row.Note,
			row.P,
			len(row.Samples[0].Values),
			len(row.Samples[1].Values),
			strings.Join(fullName, "\t  "),
		))
	}
	return ret
}

func loadFileByName(filename string) map[string]parser.Samples {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	benchData, err := format.ParseLegacyFormat(f)
	if err != nil {
		log.Fatal(err)
	}
	return parser.GetSamplesFromLegacyFormat(benchData)
}
