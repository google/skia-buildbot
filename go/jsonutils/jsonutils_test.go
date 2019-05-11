package jsonutils

import (
	"encoding/json"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNumber(t *testing.T) {
	unittest.SmallTest(t)
	type testCase struct {
		in  string
		out int64
		err string
	}
	cases := []testCase{
		{
			in:  "abc",
			out: 0,
			err: "invalid character 'a' looking for beginning of value",
		},
		{
			in:  "0",
			out: 0,
			err: "",
		},
		{
			in:  "\"0\"",
			out: 0,
			err: "",
		},
		{
			in:  "9.345",
			out: 0,
			err: "parsing \"9.345\": invalid syntax",
		},
		{
			in:  "2147483647",
			out: 2147483647,
			err: "",
		},
		{
			in:  "2147483648",
			out: 2147483648,
			err: "",
		},
		{
			in:  "9223372036854775807",
			out: 9223372036854775807,
			err: "",
		},
		{
			in:  "\"9223372036854775807\"",
			out: 9223372036854775807,
			err: "",
		},
		{
			in:  "9223372036854775808",
			out: 0,
			err: "parsing \"9223372036854775808\": value out of range",
		},
	}
	for _, tc := range cases {
		var got Number
		err := json.Unmarshal([]byte(tc.in), &got)
		if tc.err != "" {
			assert.Contains(t, err.Error(), tc.err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.out, int64(got))
		}
	}
}

func TestTime(t *testing.T) {
	unittest.SmallTest(t)
	type testCase struct {
		in  time.Time
		out string
	}
	cases := []testCase{
		{
			in:  time.Unix(0, 0),
			out: "0",
		},
		{
			in:  time.Unix(1475508449, 0),
			out: "1475508449000000",
		},
	}
	for _, tc := range cases {
		inp := Time(tc.in)
		b, err := json.Marshal(&inp)
		assert.NoError(t, err)
		assert.Equal(t, []byte(tc.out), b)
		var got Time
		err = json.Unmarshal(b, &got)
		assert.NoError(t, err)
		assert.Equal(t, tc.in.UTC(), time.Time(got).UTC())
	}
}
