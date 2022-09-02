package query

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
)

func TestValidateKey(t *testing.T) {
	testCases := []struct {
		key    string
		valid  bool
		reason string
	}{
		{
			key:    ",arch=x86,config=565,",
			valid:  true,
			reason: "",
		},
		{
			key:    ",arch=x86,con-fig=5-65,",
			valid:  true,
			reason: "",
		},
		{
			key:    ",arch=x86,config=5 65,",
			valid:  false,
			reason: "",
		},
		{
			key:    "arch=x86,config=565,",
			valid:  false,
			reason: "No comma at beginning.",
		},
		{
			key:    ",arch=x86,config=565",
			valid:  false,
			reason: "No comma at end.",
		},
		{
			key:    ",arch=x86,",
			valid:  true,
			reason: "Short is fine.",
		},
		{
			key:    "",
			valid:  false,
			reason: "Empty is invalid.",
		},
		{
			key:    ",config=565,arch=x86,",
			valid:  false,
			reason: "Unsorted.",
		},
		{
			key:    ",config=565,config=8888,",
			valid:  false,
			reason: "Duplicate param names.",
		},
		{
			key:    ",arch=x85,config=565,config=8888,",
			valid:  false,
			reason: "Duplicate param names.",
		},
		{
			key:    ",arch=x85,archer=x85,",
			valid:  true,
			reason: "Param name prefix of another param name.",
		},
		{
			key:    ",,",
			valid:  false,
			reason: "Degenerate case.",
		},
		{
			key:    ",browser=chrome,browser-version=106.0.5196.0,channel=canary,sub-test=date-format-xparb-SP,test=JetStream,type=sub-test,value=score,version=2,",
			valid:  true,
			reason: "Only check sort order on the key values, not on key=value, which breaks on this case between 'browser=' and 'browser-'",
		},
	}
	for _, tc := range testCases {
		if got, want := ValidateKey(tc.key), tc.valid; got != want {
			t.Errorf("Failed validation for %q. Got %v Want %v. %s", tc.key, got, want, tc.reason)
		}
	}
}

func TestMakeKey(t *testing.T) {
	testCases := []struct {
		m      map[string]string
		key    string
		valid  bool
		reason string
	}{
		{
			m:      map[string]string{"arch": "x86", "config": "565"},
			key:    ",arch=x86,config=565,",
			valid:  true,
			reason: "",
		},
		{
			m:      map[string]string{"arch": "x86", "archer": "x86"},
			key:    ",arch=x86,archer=x86,",
			valid:  true,
			reason: "",
		},
		{
			m:      map[string]string{"bad,key": "x86"},
			key:    "",
			valid:  false,
			reason: "Bad key.",
		},
		{
			m:      map[string]string{"key": "bad,value"},
			key:    "",
			valid:  false,
			reason: "Bad value.",
		},
		{
			m:      map[string]string{},
			key:    "",
			valid:  false,
			reason: "Empty map is invalid.",
		},
	}
	for _, tc := range testCases {
		key, err := MakeKey(tc.m)
		if tc.valid && err != nil {
			t.Errorf("Failed to make key for %#v: %s", tc.m, err)
		}
		if key != tc.key {
			t.Errorf("Failed to make key for %#v. Got %q Want %q. %s", tc.m, key, tc.key, tc.reason)
		}
	}
}

func TestNew(t *testing.T) {
	q, err := New(url.Values{"config": []string{"565", "8888"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, false, q.params[0].isWildCard)

	q, err = NewFromString("config=565&config=8888")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, false, q.params[0].isWildCard)

	q, err = NewFromString("config=%ZZ")
	assert.Error(t, err, "Invalid query strings are caught.")

	q, err = New(url.Values{"debug": []string{"false"}, "config": []string{"565", "8888"}})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, ",debug=", q.params[1].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, false, q.params[1].isWildCard)
	assert.Equal(t, false, q.params[0].isNegative)
	assert.Equal(t, false, q.params[1].isNegative)

	q, err = New(url.Values{"debug": []string{"*"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, ",debug=", q.params[0].keyMatch)
	assert.Equal(t, true, q.params[0].isWildCard)
	assert.Equal(t, false, q.params[0].isNegative)

	q, err = New(url.Values{"config": []string{"!565"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, true, q.params[0].isNegative)

	q, err = New(url.Values{"config": []string{"!565", "!8888"}, "debug": []string{"*"}})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, "565", q.params[0].values[0])
	assert.Equal(t, "8888", q.params[0].values[1])
	assert.Equal(t, ",debug=", q.params[1].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, true, q.params[0].isNegative)
	assert.Equal(t, true, q.params[1].isWildCard)
	assert.Equal(t, false, q.params[1].isNegative)

	q, err = New(url.Values{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(q.params))
}

func TestMatches(t *testing.T) {
	testCases := []struct {
		key     string
		query   url.Values
		matches bool
		reason  string
	}{
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{},
			matches: true,
			reason:  "Empty query matches everything.",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"565"}},
			matches: true,
			reason:  "Simple match",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"8888"}},
			matches: false,
			reason:  "Simple miss",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"565", "8888"}},
			matches: true,
			reason:  "Simple OR",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"8888", "565"}},
			matches: true,
			reason:  "Simple OR 2",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"565"}, "debug": []string{"true"}},
			matches: true,
			reason:  "Simple AND",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"565"}, "debug": []string{"false"}},
			matches: false,
			reason:  "Simple AND miss",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"foo": []string{"bar"}},
			matches: false,
			reason:  "Unknown param",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"arch": []string{"*"}},
			matches: true,
			reason:  "Wildcard param value match",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"foo": []string{"*"}},
			matches: false,
			reason:  "Wildcard param value missing",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"!565"}},
			matches: false,
			reason:  "Negative miss",
		},
		{
			key:     ",arch=x86,config=8888,debug=true,",
			query:   url.Values{"config": []string{"!565"}},
			matches: true,
			reason:  "Negative match",
		},
		{
			key:     ",arch=x86,config=8888,debug=true,",
			query:   url.Values{"config": []string{"!565", "!8888"}},
			matches: false,
			reason:  "Negative multi miss",
		},
		{
			key:     ",arch=x86,config=8888,debug=true,",
			query:   url.Values{"config": []string{"!565"}, "debug": []string{"*"}},
			matches: true,
			reason:  "Negative and wildcard",
		},
		{
			key:     ",arch=x86,config=8888,",
			query:   url.Values{"config": []string{"!565"}, "debug": []string{"*"}},
			matches: false,
			reason:  "Negative and wildcard, miss wildcard.",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"!565"}, "debug": []string{"*"}},
			matches: false,
			reason:  "Negative and wildcard, miss negative",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{},
			matches: true,
			reason:  "Empty query matches everything.",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"~5.5"}},
			matches: true,
			reason:  "Regexp match",
		},
		{
			key:     ",arch=x86,config=565,debug=true,",
			query:   url.Values{"config": []string{"~8+"}},
			matches: false,
			reason:  "Regexp match",
		},
		{
			key:     ",arch=x86,config=8888,debug=true,",
			query:   url.Values{"arch": []string{"~^x"}, "config": []string{"!565"}, "debug": []string{"*"}},
			matches: true,
			reason:  "Negative, wildcard, and regexp",
		},
		{
			key:     ",arch=x86,config=8888,debug=true,",
			query:   url.Values{"arch": []string{"~^y.*"}, "config": []string{"!565"}, "debug": []string{"*"}},
			matches: false,
			reason:  "Negative, wildcard, and miss regexp",
		},
	}

	for _, tc := range testCases {
		q, err := New(tc.query)
		assert.NoError(t, err)
		if got, want := q.Matches(tc.key), tc.matches; got != want {
			t.Errorf("Failed matching %q to %#v. Got %v Want %v. %s", tc.key, tc.query, got, want, tc.reason)
		}
	}
}

func TestParseKey(t *testing.T) {
	testCases := []struct {
		key      string
		parsed   map[string]string
		hasError bool
		reason   string
	}{
		{
			key: ",arch=x86,config=565,debug=true,",
			parsed: map[string]string{
				"arch":   "x86",
				"config": "565",
				"debug":  "true",
			},
			hasError: false,
			reason:   "Simple parse",
		},
		{
			key:      ",config=565,arch=x86,",
			parsed:   map[string]string{},
			hasError: true,
			reason:   "Unsorted",
		},
		{
			key:      ",,",
			parsed:   map[string]string{},
			hasError: true,
			reason:   "Invalid regex",
		},
		{
			key:      "x/y",
			parsed:   map[string]string{},
			hasError: true,
			reason:   "Invalid",
		},
		{
			key:      "",
			parsed:   map[string]string{},
			hasError: true,
			reason:   "Empty string",
		},
		{
			key: ",browser=chrome,browser-version=106.0.5196.0,channel=canary,sub-test=date-format-xparb-SP,test=JetStream,type=sub-test,value=score,version=2,",
			parsed: map[string]string{
				"browser":         "chrome",
				"browser-version": "106.0.5196.0",
				"channel":         "canary",
				"sub-test":        "date-format-xparb-SP",
				"test":            "JetStream",
				"type":            "sub-test",
				"value":           "score",
				"version":         "2",
			},
			hasError: false,
			reason:   "Only check sort order on the key values, not on key=value, which breaks on this case between 'browser=' and 'browser-'",
		},
	}
	for _, tc := range testCases {
		p, err := ParseKey(tc.key)
		if got, want := (err != nil), tc.hasError; got != want {
			t.Errorf("Failed error status parsing %q, Got %v Want %v. %s", tc.key, got, want, tc.reason)
		}
		if err != nil {
			continue
		}
		if got, want := p, tc.parsed; !reflect.DeepEqual(got, want) {
			t.Errorf("Failed matching parsed values. Got %v Want %v. %s", got, want, tc.reason)
		}
	}
}

func TestParseKeyFast(t *testing.T) {
	p, err := ParseKeyFast(",arch=x86,config=565,debug=true,")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"arch":   "x86",
		"config": "565",
		"debug":  "true",
	}, p)
}

// Test a variety of inputs to make sure the code doesn't panic.
func TestParseKeyFastNoPanics(t *testing.T) {

	testCases := []string{
		",config=565,arch=x86,", // unsorted
		",,",
		"x/y",
		"",
		",",
		"foo=bar",
		",foo=bar",
		",foo,",
		",foo,bar,baz,",
		",foo=,bar=,",
		",=,",
		",space=spaces ok here,",
	}

	for _, tc := range testCases {
		_, _ = ParseKeyFast(tc)
	}
}

func TestForceValue(t *testing.T) {
	testCases := []struct {
		input map[string]string
		want  map[string]string
	}{
		{
			input: map[string]string{"arch": "x86", "config": "565"},
			want:  map[string]string{"arch": "x86", "config": "565"},
		},
		{
			input: map[string]string{"arch": "x86", "con-fig": "5-65"},
			want:  map[string]string{"arch": "x86", "con-fig": "5-65"},
		},
		{
			input: map[string]string{"arch": "x86", "config": "5 65"},
			want:  map[string]string{"arch": "x86", "config": "5_65"},
		},
		{
			input: map[string]string{"arch": "x86", "config": ""},
			want:  map[string]string{"arch": "x86", "config": "_"},
		},
		{
			input: map[string]string{"arch::this": "x!~@#$%^&*()86"},
			want:  map[string]string{"arch__this": "x___________86"},
		},
		{
			input: map[string]string{},
			want:  map[string]string{},
		},
	}
	for _, tc := range testCases {
		if got, want := ForceValid(tc.input), tc.want; !reflect.DeepEqual(got, want) {
			t.Errorf("Failed to force a map to be valid: Got %#v Want %#v", got, want)
		}
	}
}

func TestQueryPlan(t *testing.T) {

	rops := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565", "gpu"},
		"arch":   []string{"x86", "arm"},
		"debug":  []string{"true", "false"},
		"foo":    []string{"bar"},
	}

	testCases := []struct {
		query    url.Values
		want     paramtools.ParamSet
		hasError bool
		reason   string
	}{
		{
			query:  url.Values{},
			want:   paramtools.ParamSet{},
			reason: "Empty query matches everything.",
		},
		{
			query:  url.Values{"config": []string{"565"}},
			want:   paramtools.ParamSet{"config": []string{"565"}},
			reason: "Simple",
		},
		{
			query:  url.Values{"config": []string{"565", "8888"}},
			want:   paramtools.ParamSet{"config": []string{"8888", "565"}},
			reason: "Simple OR",
		},
		{
			query: url.Values{"config": []string{"565"}, "debug": []string{"true"}},
			want: paramtools.ParamSet{
				"config": []string{"565"},
				"debug":  []string{"true"},
			},
			reason: "Simple AND",
		},
		{
			query:  url.Values{"fizz": []string{"buzz"}},
			want:   paramtools.ParamSet{},
			reason: "Unknown param",
		},
		{
			query:    url.Values{"config": []string{"buzz"}},
			want:     paramtools.ParamSet{},
			hasError: true,
			reason:   "Unknown value",
		},
		{
			query:  url.Values{"arch": []string{"*"}},
			want:   paramtools.ParamSet{"arch": []string{"x86", "arm"}},
			reason: "Wildcard param value match",
		},
		{
			query:  url.Values{"fizz": []string{"*"}},
			want:   paramtools.ParamSet{},
			reason: "Wildcard param value missing",
		},
		{
			query:  url.Values{"foo": []string{"*"}},
			want:   paramtools.ParamSet{"foo": []string{"bar"}},
			reason: "Wildcard param value missing",
		},
		{
			query:  url.Values{"config": []string{"!565"}},
			want:   paramtools.ParamSet{"config": []string{"8888", "gpu"}},
			reason: "Negative",
		},
		{
			query:  url.Values{"config": []string{"!565", "!8888"}},
			want:   paramtools.ParamSet{"config": []string{"gpu"}},
			reason: "Negative multi miss",
		},
		{
			query: url.Values{"config": []string{"!565"}, "debug": []string{"*"}},
			want: paramtools.ParamSet{
				"config": []string{"8888", "gpu"},
				"debug":  []string{"true", "false"},
			},
			reason: "Negative and wildcard",
		},
		{
			query:  url.Values{},
			want:   paramtools.ParamSet{},
			reason: "Empty query matches everything.",
		},
		{
			query:  url.Values{"config": []string{"~5.5"}},
			want:   paramtools.ParamSet{"config": []string{"565"}},
			reason: "Regexp match",
		},
		{
			query:  url.Values{"config": []string{"~8+"}},
			want:   paramtools.ParamSet{"config": []string{"8888"}},
			reason: "Regexp match",
		},
		{
			query:  url.Values{"arch": []string{"~^x"}, "config": []string{"!565"}, "debug": []string{"*"}},
			want:   paramtools.ParamSet{"arch": []string{"x86"}, "config": []string{"8888", "gpu"}, "debug": []string{"true", "false"}},
			reason: "Negative, wildcard, and regexp",
		},
		{
			query:    url.Values{"arch": []string{"~^y.*"}, "config": []string{"!565"}, "debug": []string{"*"}},
			hasError: true,
			reason:   "Negative, wildcard, and miss regexp",
		},
	}

	for _, tc := range testCases {
		q, err := New(tc.query)
		assert.NoError(t, err, tc.reason)
		ps, err := q.QueryPlan(rops)
		if tc.hasError {
			assert.Error(t, err, tc.reason)
		} else {
			assert.NoError(t, err, tc.reason)
			assert.Equal(t, tc.want, ps, tc.reason)
		}
	}
}

func TestValidateParamSet(t *testing.T) {

	assert.NoError(t, ValidateParamSet(nil))
	assert.NoError(t, ValidateParamSet(paramtools.ParamSet{}))
	assert.Error(t, ValidateParamSet(paramtools.ParamSet{"": []string{}}))
	assert.Error(t, ValidateParamSet(paramtools.ParamSet{"); DROP TABLE": []string{}}))
	assert.Error(t, ValidateParamSet(paramtools.ParamSet{"good": []string{"); DROP TABLE"}}))
}
