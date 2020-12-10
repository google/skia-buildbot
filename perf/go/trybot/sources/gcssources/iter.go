package gcssources

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/types"
)

type iter struct {
	end       types.CommitNumber
	begin     types.CommitNumber
	rangeSize int32
	n         int32
	max       int32
}

func newIter(end types.CommitNumber, tileSize int32, max int32) (*iter, error) {
	if end < 0 {
		return nil, skerr.Fmt("end is invalid: %d", end)
	}
	if tileSize < 0 {
		return nil, skerr.Fmt("tileSize is invalid: %d", tileSize)
	}
	if max < 0 {
		return nil, skerr.Fmt("max is invalid: %d", max)
	}

	return &iter{
		end:       end,
		begin:     end.Add(-(tileSize - 1)),
		rangeSize: tileSize,
		n:         -1,
		max:       max,
	}, nil
}

func (i *iter) Next() bool {
	i.n += 1
	if i.n >= 1 {
		i.end = i.end.Add(-i.rangeSize)
		i.rangeSize *= 2
		i.begin = i.end.Add(-(i.rangeSize - 1))
	}
	if i.end <= 0 {
		return false
	}
	return i.n != i.max
}

func (i *iter) Range() (types.CommitNumber, types.CommitNumber) {
	if i.begin < 0 {
		return 0, i.end
	}
	return i.begin, i.end
}
