// See README.md
package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var samples = []sampleInfo{
	{
		traceid: ",test=foo,",
		median:  10,
		min:     1,
		ratio:   10,
	},
	{
		traceid: ",test=bar,",
		median:  1,
		min:     1,
		ratio:   1,
	},
}

func TestWriteCSV_HappyPath_Success(t *testing.T) {
	var w bytes.Buffer

	err := writeCSV(samples, 100, &w)
	assert.NoError(t, err)
	expected := `traceid,min,median,ratio
",test=foo,",1.000000,10.000000,10.000000
",test=bar,",1.000000,1.000000,1.000000
`
	assert.Equal(t, expected, w.String())
}

func TestWriteCSV_PrintAll_Success(t *testing.T) {
	var w bytes.Buffer

	// -1 means print all the samples.
	err := writeCSV(samples, -1, &w)
	assert.NoError(t, err)
	expected := `traceid,min,median,ratio
",test=foo,",1.000000,10.000000,10.000000
",test=bar,",1.000000,1.000000,1.000000
`
	assert.Equal(t, expected, w.String())
}

func TestWriteCSV_PrintOne_Success(t *testing.T) {
	var w bytes.Buffer

	err := writeCSV(samples, 1, &w)
	assert.NoError(t, err)
	expected := `traceid,min,median,ratio
",test=foo,",1.000000,10.000000,10.000000
`
	assert.Equal(t, expected, w.String())
}
