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
