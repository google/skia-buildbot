package config

import "time"

type generatorConfig struct {
	SkiaRoot          string
	AflRoot           string
	FuzzSamples       string
	AflOutputPath     string
	WorkingPath       string
	ClangPath         string
	ClangPlusPlusPath string
	NumFuzzProcesses  int
	WatchAFL          bool
}

type aggregatorConfig struct {
	Bucket               string
	BinaryFuzzPath       string
	ExecutablePath       string
	NumAnalysisProcesses int
	NumUploadProcesses   int
	RescanPeriod         time.Duration
	StatusPeriod         time.Duration
	AnalysisTimeout      time.Duration
}

var Generator = generatorConfig{}
var Aggregator = aggregatorConfig{}
