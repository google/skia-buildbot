// nanostat computes and compares statistics about nanobench results.
//
// Usage:
//
//    nanostat [--sort order] [--iqrr] old.json [new.json]
//
// Each input file should be a JSON output file from nanobench.
//
// If --iqrr is specified then outliers are removed using the interquartile
// range rule.
//
// If invoked on a pair of input files, nanostat adds to the output a column
// showing the statistics from the second file and a column showing the percent
// change in mean from the first to the second file. Next to the percent change,
// nanostat shows the p-value and sample sizes from a test of the two
// distributions of nanobench results. Small p-values indicate that the two
// distributions are significantly different. If the test indicates that there
// was no significant change between the two benchmarks (defined as p > alpha),
// nanostat displays a single ~ instead of the percent change.
//
// The -sort option specifies an order in which to list the results: delta
// (percent improvement), or name (benchmark name). A leading “-”prefix, as in
// “-delta”, reverses the order. Default is name.
//
// Example
//
// Suppose we collect benchmark results from running
//
//    out/Release/nanobench --config gl 8888 --outResultsFile old.json
//
// before and after a particular change. Then nanostat summarizes and compares:
//
//    $ nanostat --iqrr ~/nanobench_old.json  ~/nanobench_new.json
//              old          new  delta               stats  name
//       0.78 ±  3%   0.72 ±  3%   -8%   (p=0.000, n=10+10)  desk_wowwiki.skp
//       2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp
//       3.08 ±  2%   2.96 ±  3%   -4%   (p=0.001, n= 9+10)  desk_facebook.skp
//       0.71 ±  2%   0.69 ±  3%   -3%   (p=0.028, n= 9+10)  desk_ebay.skp
//       4.59 ±  1%   4.46 ±  1%   -3%   (p=0.000, n=10+ 8)  desk_linkedin.skp
//       1.40 ±  1%   1.39 ±  0%   -1%   (p=0.011, n= 9+ 9)  desk_css3gradients.skp
//
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/nanostat"
)

var osExit = os.Exit // replaced during testing

func usage() {
	fmt.Printf("usage: nanostat [options] old.json [new.json]\n")
	fmt.Printf("options:\n")
	flag.PrintDefaults()
	osExit(2)
}

var (
	flagAlpha = flag.Float64("alpha", 0.05, "consider change significant if p < `α`")
	flagSort  = flag.String("sort", "delta", "sort by `order`: [-]delta, [-]name")
	flagIQRR  = flag.Bool("iqrr", false, "If true then remove outliers using the Interquartile Rule")
	flagAll   = flag.Bool("all", false, "If true then include insignificant changes in output.")
	flagTest  = flag.String("test", string(nanostat.UTest), "The type of test to do, 'utest' for Mann-Whitney U test, and 'ttest' for a Two Sample Welch T test.")
)

var sortNames = map[string]nanostat.Order{
	"name":  nanostat.ByName,
	"delta": nanostat.ByDelta,
}

var validTests = map[string]nanostat.Test{
	"t-test": nanostat.TTest,
	"t":      nanostat.TTest,
	"ttest":  nanostat.TTest,
	"u-test": nanostat.UTest,
	"u":      nanostat.UTest,
	"utest":  nanostat.UTest,
}

func main() {
	flag.Usage = usage
	flag.Parse()
	sortName := *flagSort
	reverse := false
	if strings.HasPrefix(sortName, "-") {
		reverse = true
		sortName = sortName[1:]
	}
	order, orderOK := sortNames[sortName]
	test, testOK := validTests[*flagTest]
	if flag.NArg() != 2 || !orderOK || !testOK {
		flag.Usage()
	}

	if reverse {
		order = nanostat.Reverse(order)
	}

	config := nanostat.Config{
		Alpha: *flagAlpha,
		IQRR:  *flagIQRR,
		All:   *flagAll,
		Test:  test,
		Order: order,
	}
	beforeSamples := loadFileByName(flag.Args()[0])
	afterSamples := loadFileByName(flag.Args()[1])
	rows := nanostat.Analyze(config, beforeSamples, afterSamples)

	rowsAsTabbedStrings := formatRows(config, rows)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	for _, line := range rowsAsTabbedStrings {
		_, err := fmt.Fprintln(w, line)
		if err != nil {
			log.Fatal(err)
		}
	}
	err := w.Flush()
	if err != nil {
		log.Fatal(err)
	}
}

func formatRows(config nanostat.Config, rows []nanostat.Row) []string {
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
		// if a key has more than one value that it's important we display it.
		if len(values) > 1 {
			importantKeys = append(importantKeys, key)
		}
	}
	sort.Strings(importantKeys)

	// The name of the test always goes last.
	importantKeys = append(importantKeys, "name")

	header := "old\tnew\tdelta\tstats\t" + strings.Join(importantKeys, "\t  ")

	ret = append(ret, header)

	for _, row := range rows {
		if math.IsNaN(row.Delta) && !config.All {
			continue
		}
		delta := "~"
		if !math.IsNaN(row.Delta) {
			delta = fmt.Sprintf("%.0f%%", row.Delta)
		}

		// Create the full name from all the important keys.
		fullName := []string{}
		for _, key := range importantKeys {
			fullName = append(fullName, row.Params[key])
		}
		ret = append(ret, fmt.Sprintf("%0.2f ± %2.0f%%\t%0.2f ± %2.0f%%\t%s %s\t(p=%0.3f, n=%2d+%2d)\t%s",
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
