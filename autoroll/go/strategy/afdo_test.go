package strategy

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

const (
	afdoRevPrev = "chromeos-chrome-amd64-66.0.3336.3_rc-r0-merged.afdo.bz2"
	afdoRevBase = "chromeos-chrome-amd64-66.0.3336.3_rc-r1-merged.afdo.bz2"
	afdoRevNext = "chromeos-chrome-amd64-66.0.3337.3_rc-r1-merged.afdo.bz2"
)

func TestParseAFDOVersion(t *testing.T) {
	testutils.SmallTest(t)

	// Success cases.
	testS := func(s string, expect [AFDO_VERSION_LENGTH]int) {
		actual, err := parseAFDOVersion(s)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
	}
	testS(afdoRevPrev, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 0})
	testS(afdoRevBase, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1})
	testS(afdoRevNext, [AFDO_VERSION_LENGTH]int{66, 0, 3337, 3, 1})
	testS("chromeos-chrome-amd64-67.0.3.222222_rc-r32823-merged.afdo.bz2", [AFDO_VERSION_LENGTH]int{67, 0, 3, 222222, 32823})

	// Failure cases.
	testF := func(s string) {
		_, err := parseAFDOVersion(s)
		assert.NotNil(t, err)
	}
	testF("chromeos-chrome-amd64-66.0.3336.3_rc-rr-merged.afdo.bz2")
	testF("chromeos-chrome-amd64-66.0.3336.d_rc-r1-merged.afdo.bz2")
	testF("chromeos-chrome-amd64-66.0.333b.3_rc-r1-merged.afdo.bz2")
	testF("chromeos-chrome-amd64-L6.0.3336.3_rc-r1-merged.afdo.bz2")
	testF("66.0.3336.3_rc-r1")
	testF("chromeos-chrome-amd64-66.0.3336.3_rc-r1")
	testF("66.0.3336.3_rc-rr-merged.afdo.bz2")
	testF("")
}

func TestAFDOVersionGreater(t *testing.T) {
	testutils.SmallTest(t)

	// Success cases.
	test := func(a, b string, expect bool) {
		actual, err := AFDOVersionGreater(a, b)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
	}
	test(afdoRevPrev, afdoRevBase, false)
	test(afdoRevBase, afdoRevPrev, true)
	test(afdoRevBase, afdoRevBase, false)
	test(afdoRevBase, afdoRevNext, false)
	test(afdoRevNext, afdoRevBase, true)
	test(afdoRevPrev, afdoRevNext, false)
	test(afdoRevNext, afdoRevPrev, true)

	t2 := func(a, b [AFDO_VERSION_LENGTH]int, expect bool) {
		tmpl := "chromeos-chrome-amd64-%d.%d.%d.%d_rc-r%d-merged.afdo.bz2"
		verA := fmt.Sprintf(tmpl, a[0], a[1], a[2], a[3], a[4])
		verB := fmt.Sprintf(tmpl, b[0], b[1], b[2], b[3], b[4])
		test(verA, verB, expect)
	}

	t2([AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1}, [AFDO_VERSION_LENGTH]int{64, 0, 3282, 165, 1}, true)
	t2([AFDO_VERSION_LENGTH]int{64, 0, 3282, 165, 1}, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 4}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 4}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 3, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 3, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 2, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 2, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 1, 5, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 1, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{0, 5, 5, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{0, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
}
