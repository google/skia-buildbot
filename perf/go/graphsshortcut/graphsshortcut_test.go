package graphsshortcut

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIDFromGraphs(t *testing.T) {

	sc := &GraphsShortcut{}
	assert.Equal(t, "", sc.GetID())

	sc = &GraphsShortcut{
		Graphs: []GraphConfig{
			{
				Queries: []string{
					"arch=x86&config=8888",
					"arch=arm&config=8888",
				},
			},
			{
				Keys: "abcdef",
			},
		},
	}
	assert.Equal(t, "c21e3c138176a30ee86c582e2f7689d9", sc.GetID())

	// Test that order of queries in the same GraphConfig doesn't matter.
	sc = &GraphsShortcut{
		Graphs: []GraphConfig{
			{
				Queries: []string{
					"arch=arm&config=8888",
					"arch=x86&config=8888",
				},
			},
			{
				Keys: "abcdef",
			},
		},
	}
	assert.Equal(t, "c21e3c138176a30ee86c582e2f7689d9", sc.GetID())

	// Test that order of graph configs does matter.
	sc = &GraphsShortcut{
		Graphs: []GraphConfig{
			{
				Keys: "abcdef",
			},
			{
				Queries: []string{
					"arch=arm&config=8888",
					"arch=x86&config=8888",
				},
			},
		},
	}
	assert.NotEqual(t, "c21e3c138176a30ee86c582e2f7689d9", sc.GetID())

}
