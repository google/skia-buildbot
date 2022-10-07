package mocks

import (
	time "time"

	"go.skia.org/infra/go/now"
)

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name TimeTicker  --srcpkg=go.skia.org/infra/go/now --output ${PWD}

func NewTimeTickerFunc(ch <-chan time.Time) now.NewTimeTickerFunc {
	return func(unused time.Duration) now.TimeTicker {
		rv := &TimeTicker{}
		rv.On("C").Return(ch)
		return rv
	}
}
