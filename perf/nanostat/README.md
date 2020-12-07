# nanostat

nanostat compares statistics about nanobench results.

## Installation

You need to have [Go](https://golang.org) installed. Then run:

    go get -u go.skia.org/infra/perf/nanostat

If you have `$GOPATH` or `$GOBIN` set then `nanostat` should appear in your
path, otherwise it will be installed at `$HOME/go/bin/nanostat`.

## Description

Each input file should be a JSON output file from nanobench.

Invoked on a pair of input files, nanostat computes statistics for each
file and a column showing the percent change in mean from the first to
the second file. Next to the percent change, nanostat shows the p-value
and sample sizes from a test of the two distributions of nanobench
results.

For example in the results below, 'p' equals 0.001 or 0.1%, and the
analysis was done with 10 samples from the first file and 8 samples from
the second file.

             old          new  delta     stats            name
      2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp

Small p-values indicate that the two distributions are
significantly different. If the test indicates that there was no
significant change between the two benchmarks (defined as p > alpha),
nanostat displays a single ~ instead of the percent change.

## Example

Suppose we collect benchmark results from running

    out/Release/nanobench --config gl 8888 --outResultsFile old.json

Then make some changes to the code, recompile nanobench and run:

    out/Release/nanobench --config gl 8888 --outResultsFile new.json

Then nanostat summarizes the differences between the old and new runs:

    $ nanostat --iqrr old.json new.json
              old          new  delta         s            name
       0.78 ±  3%   0.72 ±  3%   -8%   (p=0.000, n=10+10)  desk_wowwiki.skp
       2.15 ±  5%   2.00 ±  2%   -7%   (p=0.001, n=10+ 8)  tabl_digg.skp
       3.08 ±  2%   2.96 ±  3%   -4%   (p=0.001, n= 9+10)  desk_facebook.skp
       0.71 ±  2%   0.69 ±  3%   -3%   (p=0.028, n= 9+10)  desk_ebay.skp
       4.59 ±  1%   4.46 ±  1%   -3%   (p=0.000, n=10+ 8)  desk_linkedin.skp
       1.40 ±  1%   1.39 ±  0%   -1%   (p=0.011, n= 9+ 9)  desk_css3gradients.skp
    $

## Usage

    usage: nanostat [options] old.json new.json
    options:
      -all
           If true then include insignificant changes in output.
      -alpha float
           Consider a change significant if p < α. Must be > 0. (default 0.05).
      -iqrr
           If true then remove outliers in the samples using the Interquartile Range Rule.
      -sort order
           Sort by order: [-]delta, [-]name (default "delta")
      -test string
           The type of test to do, 'utest' for Mann-Whitney U test, and 'ttest' for a Two Sample Welch T test. (default "utest")

To get help:

    $ nanostat -h
