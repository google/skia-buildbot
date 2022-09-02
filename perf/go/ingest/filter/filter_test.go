// Package filter implements Accept/Reject filtering of file.Names.
package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReject_BothRegexpEmpty_AcceptsAllFiles(t *testing.T) {
	f, err := New(``, ``)
	require.NoError(t, err)
	assert.False(t, f.Reject("foo"))
}

func TestReject_JustAnAcceptRegex_AcceptsOnlyMatchingFiles(t *testing.T) {
	f, err := New(`/foo/`, ``)
	require.NoError(t, err)
	assert.True(t, f.Reject("foo"))
	assert.False(t, f.Reject("/foo/bar/"))
}

func TestReject_JustARejectRegex_AcceptsOnlyFilesThatDontMatch(t *testing.T) {
	f, err := New(``, `/tx_log/`)
	require.NoError(t, err)
	assert.False(t, f.Reject("foo"))
	assert.True(t, f.Reject("gs://bucket/tx_log/foo"))
}

func TestReject_BothAcceptAndRejectRegex_BothFiltersAreApplied(t *testing.T) {
	f, err := New(`foo`, `/tx_log/`)
	require.NoError(t, err)
	assert.False(t, f.Reject("/foo"), "Passes both filters.")
	assert.True(t, f.Reject("/tx_log/foo"), "Rejected by reject filter.")
	assert.True(t, f.Reject("bar"), "Rejected by not matching accept filter.")
}

func TestReject_ErrorOnInvalidRegex(t *testing.T) {
	_, err := New(`\K`, ``)
	require.Error(t, err)

	_, err = New(``, `\K`)
	require.Error(t, err)
}
