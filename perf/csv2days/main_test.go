package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var input = `
config,2022-11-19T10:50:37.000Z,2022-11-19T14:20:05.000Z,2022-11-20T07:13:11.000Z
a,1,,
b,,2,
c,,,3
`

var output = `config,2022-11-19,2022-11-20
a,1,
b,2,
c,,3
`

func TestTransformCSV_HappyPath(t *testing.T) {
	inBuffer := strings.NewReader(input)
	var outBuffer bytes.Buffer

	err := transformCSV(inBuffer, &outBuffer)
	require.NoError(t, err)
	require.Equal(t, output, outBuffer.String())
}
