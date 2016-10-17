package fuzzpool

import (
	"bytes"
	"encoding/gob"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/data"
)

type FuzzPool struct {
	staging data.SortedFuzzReports
	// Current is exported so it can be stored to fuzzcache
	Current data.SortedFuzzReports
}

func NewForTests(r []data.FuzzReport) *FuzzPool {
	return &FuzzPool{
		staging: data.SortedFuzzReports{},
		Current: r,
	}
}

func New() *FuzzPool {
	return &FuzzPool{
		staging: data.SortedFuzzReports{},
		Current: data.SortedFuzzReports{},
	}
}

func (p *FuzzPool) AddFuzzReport(r data.FuzzReport) {
	p.staging = p.staging.Append(r)
}

func (p *FuzzPool) ClearStaging() {
	p.staging = data.SortedFuzzReports{}
}

func (p *FuzzPool) CurrentFromStaging() {
	p.Current = cloneReports(p.staging)
}

func (p *FuzzPool) StagingFromCurrent() {
	p.staging = cloneReports(p.Current)
}

func (p FuzzPool) Reports() []data.FuzzReport {
	return p.Current
}

func (p FuzzPool) FindFuzzDetailForFuzz(name string) ([]data.FuzzReport, error) {
	return p.Current, nil
}

func (p FuzzPool) FindFuzzDetails(category, architecture, file, function string, line int) ([]data.FuzzReport, error) {
	return p.Current, nil
}

// cloneReport makes a copy of the input using the gob library.
func cloneReports(r data.SortedFuzzReports) data.SortedFuzzReports {
	var temp bytes.Buffer
	enc := gob.NewEncoder(&temp)
	dec := gob.NewDecoder(&temp)

	if err := enc.Encode(r); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	var clone data.SortedFuzzReports
	if err := dec.Decode(&clone); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	return clone
}
