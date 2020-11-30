// nanostat computes and compares statistics about nanobench results.
//
// Usage:
//
//  nanostat [--sort order] [--iqrr] old.json [new.json]
//
// Each input file should contain the concatenated output of a number of runs of
// ``go test -bench.'' For each different benchmark listed in an input file,
// nanostat computes the mean, minimum, and maximum.
//
// If --iqrr is specified then outliers are removed using the interquartile
// range rule.
//
// If invoked on a single input file, nanostat prints statistics for that file.
//
// If invoked on a pair of input files, nanostat adds to the output a column
// showing the statistics from the second file and a column showing the percent
// change in mean from the first to the second file. Next to the percent change,
// nanostat shows the p-value and sample sizes from a test of the two
// distributions of nanobench results. Small p-values indicate that the two
// distributions are significantly different. If the test indicates that there
// was no significant change between the two benchmarks (defined as p > 0.05),
// nanostat displays a single ~ instead of the percent change.
//
// The -sort option specifies an order in which to list the results: delta
// (percent improvement), or name (benchmark name). A leading “-”prefix, as in
// “-delta”, reverses the order. Default is name.
//
// Example
//
// Suppose we collect benchmark results from running ``out/Release/nanobench --config gl 8888 --outResultsFile old json''
// before and after a particular change.
//
// If run with just one input file, nanostat summarizes that file:
//
//  $ nanostat old.json
//  name        time/op
//  GobEncode   13.6ms ± 1%
//  JSONEncode  32.1ms ± 1%
//  $
//
// If run with two input files, nanostat summarizes and compares:
//
//  $ nanostat old.json new.json
//  name        old time/op  new time/op  delta
//  GobEncode   13.6ms ± 1%  11.8ms ± 1%  -13.31% (p=0.016 n=4+5)
//  JSONEncode  32.1ms ± 1%  31.8ms ± 1%     ~    (p=0.286 n=4+5)
//  $
//
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"text/tabwriter"

	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
)

var exit = os.Exit // replaced during testing

func usage() {
	fmt.Fprintf(os.Stderr, "usage: nanostat [options] old.json [new.json]\n")
	fmt.Fprintf(os.Stderr, "options:\n")
	flag.PrintDefaults()
	exit(2)
}

var (
	flagAlpha = flag.Float64("alpha", 0.05, "consider change significant if p < `α`")
	flagSort  = flag.String("sort", "delta", "sort by `order`: [-]delta, [-]name")
	flagIQRR  = flag.Bool("iqrr", false, "If true then remove outliers using the Interquartile Rule")
	flagAll   = flag.Bool("all", false, "If true then include insignificant changes in output.")
	flagTest  = flag.String("test", string(UTest), "The type of test to do, 'utest' for Mann-Whitney U test, and 'ttest' for a Two Sample Welch T test.")
)

var sortNames = map[string]Order{
	"name":  ByName,
	"delta": ByDelta,
}

var validTests = map[string]Test{
	"utest":  UTest,
	"ttest":  TTest,
	"u-test": UTest,
	"t-test": TTest,
	"u":      UTest,
	"t":      TTest,
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

	config := Config{
		Alpha: *flagAlpha,
		IQRR:  *flagIQRR,
		All:   *flagAll,
		Test:  test,
	}
	if order != nil {
		if reverse {
			order = Reverse(order)
		}
		config.Order = order
	}
	beforeSamples := loadFileByName(flag.Args()[0])
	afterSamples := loadFileByName(flag.Args()[1])
	rows := Analyze(config, beforeSamples, afterSamples)

	rowsAsTabbedStrings := formatRows(config, rows)
	// Convert rows to []string with tabs and then use tabwrite to print the table.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	for _, line := range rowsAsTabbedStrings {
		fmt.Fprintln(w, line)
	}
	err := w.Flush()
	if err != nil {
		log.Fatal(err)
	}
}

func formatRows(config Config, rows []Row) []string {
	ret := make([]string, 0, len(rows))

	ret = append(ret, "old\tnew\tdelta\tstats\t  name")

	for _, row := range rows {
		if math.IsNaN(row.Delta) && !config.All {
			continue
		}
		delta := "~"
		if !math.IsNaN(row.Delta) {
			delta = fmt.Sprintf("%.0f%%", row.Delta)
		}
		ret = append(ret, fmt.Sprintf("%0.2f ± %2.0f%%\t%0.2f ± %2.0f%%\t%s %s\t(p=%0.3f, n=%2d+%2d)\t  %s",
			row.Samples[0].Mean,
			row.Samples[0].Percent,
			row.Samples[1].Mean,
			row.Samples[1].Percent,
			delta,
			row.Note,
			row.P,
			len(row.Samples[0].Values),
			len(row.Samples[1].Values),
			row.Params["name"], // TODO - if there are multiple values of config then add config here.
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

var htmlHeader = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Performance Result Comparison</title>
<style>
.benchstat { border-collapse: collapse; }
.benchstat th:nth-child(1) { text-align: left; }
.benchstat tbody td:nth-child(1n+2):not(.note) { text-align: right; padding: 0em 1em; }
.benchstat tr:not(.configs) th { border-top: 1px solid #666; border-bottom: 1px solid #ccc; }
.benchstat .nodelta { text-align: center !important; }
.benchstat .better td.delta { font-weight: bold; }
.benchstat .worse td.delta { font-weight: bold; color: #c00; }
</style>
</head>
<body>
`
var htmlFooter = `</body>
</html>
`
