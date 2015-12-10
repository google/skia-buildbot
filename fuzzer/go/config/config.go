package config

import (
	"sync"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

type generatorConfig struct {
	SkiaRoot         string
	AflRoot          string
	FuzzSamples      string
	AflOutputPath    string
	WorkingPath      string
	NumFuzzProcesses int
	WatchAFL         bool
}

type aggregatorConfig struct {
	BinaryFuzzPath       string
	ExecutablePath       string
	NumAnalysisProcesses int
	NumUploadProcesses   int
	RescanPeriod         time.Duration
	StatusPeriod         time.Duration
	AnalysisTimeout      time.Duration
}

type frontendConfig struct {
	SkiaRoot string
}

type gsConfig struct {
	Bucket string
}

type commonConfig struct {
	ClangPath         string
	ClangPlusPlusPath string
	DepotToolsPath    string
	SkiaVersion       *vcsinfo.LongCommit
	versionMutex      sync.Mutex
}

var Generator = generatorConfig{}
var Aggregator = aggregatorConfig{}
var GS = gsConfig{}
var Common = commonConfig{}
var FrontEnd = frontendConfig{}

// SetSkiaVersion safely stores the LongCommit of the skia version that is being used.
func SetSkiaVersion(lc *vcsinfo.LongCommit) {
	Common.versionMutex.Lock()
	defer Common.versionMutex.Unlock()
	Common.SkiaVersion = lc
}
