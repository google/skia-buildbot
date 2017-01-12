package config

import (
	"sync"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

type generatorConfig struct {
	AflRoot                string
	Architecture           string
	FuzzSamples            string
	AflOutputPath          string
	WorkingPath            string
	NumAPIFuzzProcesses    int
	NumBinaryFuzzProcesses int
	NumDownloadProcesses   int
	WatchAFL               bool
	SkipGeneration         bool
	FuzzesToGenerate       []string
}

type aggregatorConfig struct {
	FuzzPath             string
	WorkingPath          string
	NumAnalysisProcesses int
	NumUploadProcesses   int
	RescanPeriod         time.Duration
	StatusPeriod         time.Duration
	AnalysisTimeout      time.Duration
}

type frontendConfig struct {
	BoltDBPath           string
	NumDownloadProcesses int
	FuzzSyncPeriod       time.Duration
}

type gcsConfig struct {
	Bucket string
}

type commonConfig struct {
	ClangPath           string
	ClangPlusPlusPath   string
	DepotToolsPath      string
	ForceReanalysis     bool
	VerboseBuilds       bool
	ExecutableCachePath string
	SkiaRoot            string
	SkiaVersion         *vcsinfo.LongCommit
	VersionCheckPeriod  time.Duration

	versionMutex sync.Mutex
}

var Generator = generatorConfig{}
var Aggregator = aggregatorConfig{}
var GCS = gcsConfig{}
var Common = commonConfig{}
var FrontEnd = frontendConfig{}

type VersionSetter interface {
	SetSkiaVersion(lc *vcsinfo.LongCommit)
}

// SetSkiaVersion safely stores the LongCommit of the skia version that is being used.
func (f *commonConfig) SetSkiaVersion(lc *vcsinfo.LongCommit) {
	f.versionMutex.Lock()
	defer f.versionMutex.Unlock()
	f.SkiaVersion = lc
}
