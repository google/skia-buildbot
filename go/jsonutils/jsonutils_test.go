package jsonutils

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestNumber(t *testing.T) {
	testutils.SmallTest(t)
	type testCase struct {
		in  string
		out int64
		err error
	}
	parser := "strconv.Atoi"
	if strings.HasPrefix(runtime.Version(), "1.7") {
		// TODO(kjlubick): remove this logic when go 1.7 feels ancient
		parser = "strconv.ParseInt"
	}
	cases := []testCase{
		{
			in:  "abc",
			out: 0,
			err: fmt.Errorf("invalid character 'a' looking for beginning of value"),
		},
		{
			in:  "0",
			out: 0,
			err: nil,
		},
		{
			in:  "\"0\"",
			out: 0,
			err: nil,
		},
		{
			in:  "9.345",
			out: 0,
			err: fmt.Errorf("%s: parsing \"9.345\": invalid syntax", parser),
		},
		{
			in:  "2147483647",
			out: 2147483647,
			err: nil,
		},
		{
			in:  "2147483648",
			out: 2147483648,
			err: nil,
		},
		{
			in:  "9223372036854775807",
			out: 9223372036854775807,
			err: nil,
		},
		{
			in:  "\"9223372036854775807\"",
			out: 9223372036854775807,
			err: nil,
		},
		{
			in:  "9223372036854775808",
			out: 0,
			err: fmt.Errorf("%s: parsing \"9223372036854775808\": value out of range", parser),
		},
	}
	for _, tc := range cases {
		var got Number
		err := json.Unmarshal([]byte(tc.in), &got)
		if tc.err != nil {
			assert.EqualError(t, err, tc.err.Error())
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.out, int64(got))
		}
	}
}

func TestTime(t *testing.T) {
	testutils.SmallTest(t)
	type testCase struct {
		in  time.Time
		out string
		err error
	}
	cases := []testCase{
		{
			in:  time.Unix(0, 0),
			out: "0",
			err: nil,
		},
		{
			in:  time.Unix(1475508449, 0),
			out: "1475508449000000",
			err: nil,
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
