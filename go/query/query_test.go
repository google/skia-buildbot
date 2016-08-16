package query

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
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
	q := New(url.Values{"config": []string{"565", "8888"}})
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, false, q.params[0].isWildCard)

	q = New(url.Values{"debug": []string{"false"}, "config": []string{"565", "8888"}})
	assert.Equal(t, 2, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, ",debug=", q.params[1].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, false, q.params[1].isWildCard)
	assert.Equal(t, false, q.params[0].isNegative)
	assert.Equal(t, false, q.params[1].isNegative)

	q = New(url.Values{"debug": []string{"*"}})
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, ",debug=", q.params[0].keyMatch)
	assert.Equal(t, true, q.params[0].isWildCard)
	assert.Equal(t, false, q.params[0].isNegative)

	q = New(url.Values{"config": []string{"!565"}})
	assert.Equal(t, 1, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, true, q.params[0].isNegative)

	q = New(url.Values{"config": []string{"!565", "!8888"}, "debug": []string{"*"}})
	assert.Equal(t, 2, len(q.params))
	assert.Equal(t, ",config=", q.params[0].keyMatch)
	assert.Equal(t, "565", q.params[0].values[0])
	assert.Equal(t, "8888", q.params[0].values[1])
	assert.Equal(t, ",debug=", q.params[1].keyMatch)
	assert.Equal(t, false, q.params[0].isWildCard)
	assert.Equal(t, true, q.params[0].isNegative)
	assert.Equal(t, true, q.params[1].isWildCard)
	assert.Equal(t, false, q.params[1].isNegative)

	q = New(url.Values{})
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
	}
	for _, tc := range testCases {
		if got, want := New(tc.query).Matches(tc.key), tc.matches; got != want {
			t.Errorf("Failed matching %q to %#v. Got %v Want %v. %s", tc.key, tc.query, got, want, tc.reason)
		}
	}
}
