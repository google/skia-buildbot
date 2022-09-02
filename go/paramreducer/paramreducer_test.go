package paramreducer

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
)

func TestReducer(t *testing.T) {
	keys := []string{
		",config=8888,cpu=x86,res=ms,",
		",config=565,cpu=x86,res=count,",
		",config=565,cpu=arm,res=cov,",
		",config=gles,cpu=arm,res=bytes,",
		",foo=bar,",
	}
	full := paramtools.NewParamSet()
	for _, key := range keys {
		full.AddParamsFromKey(key)
	}
	full.Normalize()
	testCases := []struct {
		query    url.Values
		expected paramtools.ParamSet
		message  string
	}{
		{
			query: url.Values{
				"config": []string{"565"},
				"res":    []string{"cov", "bytes"},
			},
			expected: paramtools.ParamSet{
				"cpu":    []string{"arm"},
				"config": []string{"565", "gles"},
				"res":    []string{"count", "cov"},
			},

			message: "Motivating example.",
		},
		{
			query: url.Values{
				"config": []string{"565"},
			},
			expected: paramtools.ParamSet{
				"cpu":    []string{"arm", "x86"},
				"res":    []string{"count", "cov"},
				"config": []string{"565", "8888", "gles"},
			},

			message: "One key.",
		},
		{
			query:    url.Values{},
			expected: full,
			message:  "Empty query",
		},
		{
			query: url.Values{
				"foo": []string{"bar"},
			},
			expected: paramtools.ParamSet{"foo": []string{"bar"}},
			message:  "Drop key if no values.",
		},
	}

	for _, tc := range testCases {
		r, err := New(tc.query, full)
		assert.NoError(t, err)
		for _, key := range keys {
			r.Add(key)
		}
		assert.Equal(t, tc.expected, r.Reduce(), tc.message)
	}
}
