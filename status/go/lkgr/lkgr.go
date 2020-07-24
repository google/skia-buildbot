package lkgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// LKGR is a struct used for tracking the Last Known Good Revision of Skia.
type LKGR struct {
	hash string
	mtx  sync.RWMutex
}

// Return an LKGR instance.
func New(ctx context.Context) (*LKGR, error) {
	rv := &LKGR{}
	return rv, rv.Update(ctx)
}

// Return the LKGR.
func (r *LKGR) Get() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.hash
}

// Update LKGR.
func (r *LKGR) Update(ctx context.Context) error {
	depsContents, err := gitiles.NewRepo(common.REPO_CHROMIUM, nil).ReadFile(ctx, deps_parser.DepsFileName)
	if err != nil {
		return err
	}
	deps, err := deps_parser.ParseDeps(string(depsContents))
	if err != nil {
		return err
	}
	dep := deps.Get(common.REPO_SKIA)
	if dep == nil {
		return fmt.Errorf("Unable to find skia_revision in DEPS!")
	}
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.hash = dep.Id
	return nil
}

// Start updating LKGR in a loop.
func (r *LKGR) UpdateLoop(freq time.Duration, ctx context.Context) {
	lv := metrics2.NewLiveness("last_successful_lkgr_update")
	go util.RepeatCtx(ctx, freq, func(ctx context.Context) {
		if err := r.Update(ctx); err != nil {
			sklog.Errorf("Failed to update LKGR: %s", err)
		} else {
			lv.Reset()
		}
	})
}
