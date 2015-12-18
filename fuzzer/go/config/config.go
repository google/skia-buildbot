package config

import (
	"sync"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

type generatorConfig struct {
	SkiaRoot             string
	AflRoot              string
	FuzzSamples          string
	AflOutputPath        string
	WorkingPath          string
	NumFuzzProcesses     int
	NumDownloadProcesses int
	WatchAFL             bool
	VersionCheckPeriod   time.Duration
	SkiaVersion          *vcsinfo.LongCommit
	versionMutex         sync.Mutex
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
	SkiaRoot             string
	BoltDBPath           string
	NumDownloadProcesses int
	SkiaVersion          *vcsinfo.LongCommit
	versionMutex         sync.Mutex
}

type gsConfig struct {
	Bucket string
}

type commonConfig struct {
	ClangPath         string
	ClangPlusPlusPath string
	DepotToolsPath    string
}

var Generator = generatorConfig{}
var Aggregator = aggregatorConfig{}
var GS = gsConfig{}
var Common = commonConfig{}
var FrontEnd = frontendConfig{}

type VersionSetter interface {
	SetSkiaVersion(lc *vcsinfo.LongCommit)
}

// SetSkiaVersion safely stores the LongCommit of the skia version that is being used.
func (f *frontendConfig) SetSkiaVersion(lc *vcsinfo.LongCommit) {
	f.versionMutex.Lock()
	defer f.versionMutex.Unlock()
	f.SkiaVersion = lc
}

// SetSkiaVersion safely stores the LongCommit of the skia version that is being used.
func (g *generatorConfig) SetSkiaVersion(lc *vcsinfo.LongCommit) {
	g.versionMutex.Lock()
	defer g.versionMutex.Unlock()
	g.SkiaVersion = lc
}
