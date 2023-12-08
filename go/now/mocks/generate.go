package mocks

import (
	time "time"

	"go.skia.org/infra/go/now"
)

func NewTimeTickerFunc(ch <-chan time.Time) now.NewTimeTickerFunc {
	return func(unused time.Duration) now.TimeTicker {
		rv := &TimeTicker{}
		rv.On("C").Return(ch)
		return rv
	}
}
