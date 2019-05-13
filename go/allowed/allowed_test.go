package allowed

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAllowed(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		allowed  []string
		value    string
		expected bool
		message  string
	}{
		{
			allowed:  []string{},
			value:    "test@example.org",
			expected: false,
			message:  "empty",
		},
		{
			allowed:  []string{""},
			value:    "test@",
			expected: false,
			message:  "empty domain",
		},
		{
			allowed:  []string{"test@example.org"},
			value:    "test@example.org",
			expected: true,
			message:  "single email",
		},
		{
			allowed:  []string{"example.org"},
			value:    "test@example.org",
			expected: true,
			message:  "single domain",
		},
		{
			allowed:  []string{"example.org"},
			value:    "test@google.com",
			expected: false,
			message:  "single domain fail",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "test@google.com",
			expected: true,
			message:  "multi domain",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "foo@chromium.org",
			expected: true,
			message:  "multi domain 2",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "special@example.com",
			expected: true,
			message:  "multi domain 3",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "missing@example.com",
			expected: false,
			message:  "multi domain 4",
		},
	}

	for _, tc := range testCases {
		w := NewAllowedFromList(tc.allowed)
		if got, want := w.Member(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}

func TestAllowedFromFile(t *testing.T) {
	unittest.SmallTest(t)
	dirname, err := ioutil.TempDir("", "allowed_file")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, dirname)
	emails := `fred@example.com
barney@example.com`
	filename := filepath.Join(dirname, "allowed")
	err = ioutil.WriteFile(filename, []byte(emails), 0644)
	assert.NoError(t, err)
	w, err := NewAllowedFromFile(filename)
	assert.NoError(t, err)
	assert.True(t, w.Member("fred@example.com"))
	assert.True(t, w.Member("barney@example.com"))

	emails = `fred@example.com`
	err = ioutil.WriteFile(filename, []byte(emails), 0644)
	assert.NoError(t, err)

	end := time.Now().Add(10 * time.Second)
	for {
		if false == w.Member("barney@example.com") {
			break
		}
		if time.Now().After(end) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	assert.True(t, w.Member("fred@example.com"))
	assert.False(t, w.Member("barney@example.com"))

	// Removing the file doesn't clear the list, instead the old one is kept.
	err = os.Remove(filename)
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.True(t, w.Member("fred@example.com"))
	assert.False(t, w.Member("barney@example.com"))
}
