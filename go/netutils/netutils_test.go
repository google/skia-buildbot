// Package netutils contains utilities to work with ports and URLs.
package netutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testRootDomain(t *testing.T, host, rootDomain string) {
	t.Helper()
	require.Equal(t, rootDomain, RootDomain(host))
}

func TestRootDomain(t *testing.T) {
	testRootDomain(t, "skia.org", "skia.org")
	testRootDomain(t, "docs.skia.org", "skia.org")
	testRootDomain(t, "docs.skia.org:8000", "skia.org")
	testRootDomain(t, "foo.bar.baz.skia.org", "skia.org")
	testRootDomain(t, "perf.luci.app", "luci.app")
	testRootDomain(t, "localhost", "localhost")
	testRootDomain(t, "", "")
}
