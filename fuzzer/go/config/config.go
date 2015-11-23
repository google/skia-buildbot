package config

import "time"

type generatorConfig struct {
	SkiaRoot          string
	AflRoot           string
	AflOutputPath     string
	ClangPath         string
	ClangPlusPlusPath string
	NumFuzzProcesses  int
}

type aggregatorConfig struct {
	BinaryFuzzPath          string
	ExecutablePath          string
	NumAggregationProcesses int
	RescanPeriod            time.Duration
	AnalysisTimeout         time.Duration
}

var Generator = generatorConfig{}
var Aggregator = aggregatorConfig{}
