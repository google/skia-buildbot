package process

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
)

func TestGetTraceIdsForClustering(t *testing.T) {
	testCases := []struct {
		name     string
		params   []paramtools.Params
		expected []string
	}{
		{
			name:     "No params",
			params:   []paramtools.Params{},
			expected: []string{},
		},
		{
			name: "Basic params without suffix",
			params: []paramtools.Params{
				{"test": "test1", "stat": "value"},
				{"test": "test2", "stat": "value"},
			},
			expected: []string{
				",stat=value,test=test1,",
				",stat=value,test=test2,",
			},
		},
		{
			name: "Suffix without counterpart",
			params: []paramtools.Params{
				{"test": "test1_avg", "stat": "value"},
			},
			expected: []string{
				",stat=value,test=test1_avg,",
			},
		},
		{
			name: "Suffix with counterpart - avg",
			params: []paramtools.Params{
				{"test": "test1", "stat": "value"},
				{"test": "test1_avg", "stat": "value"},
			},
			expected: []string{
				",stat=value,test=test1,",
			},
		},
		{
			name: "Suffix with counterpart - min",
			params: []paramtools.Params{
				{"test": "test1", "stat": "min"},
				{"test": "test1_min", "stat": "value"},
			},
			expected: []string{
				",stat=min,test=test1,",
			},
		},
		{
			name: "Suffix with counterpart - max",
			params: []paramtools.Params{
				{"test": "test1", "stat": "max"},
				{"test": "test1_max", "stat": "value"},
			},
			expected: []string{
				",stat=max,test=test1,",
			},
		},
		{
			name: "Suffix with counterpart - count",
			params: []paramtools.Params{
				{"test": "test1", "stat": "count"},
				{"test": "test1_count", "stat": "value"},
			},
			expected: []string{
				",stat=count,test=test1,",
			},
		},
		{
			name: "Suffix with mismatching counterpart stat",
			params: []paramtools.Params{
				{"test": "test1", "stat": "other"},
				{"test": "test1_avg", "stat": "value"},
			},
			expected: []string{
				",stat=other,test=test1,",
				",stat=value,test=test1_avg,",
			},
		},
		{
			name: "Multiple mixed cases",
			params: []paramtools.Params{
				{"test": "testA", "stat": "value"},     // Keep
				{"test": "testA_avg", "stat": "value"}, // Skip (matches testA, stat=value)
				{"test": "testB", "stat": "min"},       // Keep
				{"test": "testB_min", "stat": "value"}, // Skip (matches testB, stat=min)
				{"test": "testC", "stat": "other"},     // Keep
				{"test": "testC_avg", "stat": "value"}, // Keep (mismatch stat)
				{"test": "testD_avg", "stat": "value"}, // Keep (no counterpart)
			},
			expected: []string{
				",stat=value,test=testA,",
				",stat=min,test=testB,",
				",stat=other,test=testC,",
				",stat=value,test=testC_avg,",
				",stat=value,test=testD_avg,",
			},
		},
		{
			name: "No suffixed traces",
			params: []paramtools.Params{
				{"test": "test1", "stat": "value"},
				{"test": "test2", "stat": "min"},
			},
			expected: []string{
				",stat=value,test=test1,",
				",stat=min,test=test2,",
			},
		},
		{
			name: "Arbitrary keys match",
			params: []paramtools.Params{
				{"test": "test1", "stat": "value", "extra": "foo", "config": "bar"},     // Canonical
				{"test": "test1_avg", "stat": "value", "extra": "foo", "config": "bar"}, // Suffix, matches canonical on all keys
			},
			expected: []string{
				",config=bar,extra=foo,stat=value,test=test1,",
			},
		},
		{
			name: "Arbitrary keys mismatch",
			params: []paramtools.Params{
				{"test": "test1", "stat": "value", "extra": "foo", "config": "bar"},     // Canonical
				{"test": "test1_avg", "stat": "value", "extra": "baz", "config": "bar"}, // Suffix, but 'extra' differs
			},
			expected: []string{
				",config=bar,extra=foo,stat=value,test=test1,",
				",config=bar,extra=baz,stat=value,test=test1_avg,",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getTraceIdsForClustering(tc.params)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}
