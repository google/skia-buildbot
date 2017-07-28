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
	}
	full := paramtools.NewParamSet()
	for _, key := range keys {
		full.AddParamsFromKey(key)
	}
	q := url.Values{
		"config": []string{"565"},
		"res":    []string{"cov", "bytes"},
	}
	r, err := New(q, full)
	assert.NoError(t, err)
	for _, key := range keys {
		r.Add(key)
	}
	expected := paramtools.ParamSet{
		"cpu":    []string{"arm"},
		"config": []string{"565", "gles"},
		"res":    []string{"count", "cov"},
	}
	assert.True(t, expected.Equal(r.Reduce()))

}
