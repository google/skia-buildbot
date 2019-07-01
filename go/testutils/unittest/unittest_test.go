package unittest_test

import (
	"testing"
	"time"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestSmall1(t *testing.T) {
	unittest.SmallTest(t)
	time.Sleep(1 * time.Second)
}

func TestSmall2(t *testing.T) {
	unittest.SmallTest(t)
	time.Sleep(1 * time.Second)
}

func TestSmall3(t *testing.T) {
	unittest.SmallTest(t)
	time.Sleep(1 * time.Second)
}

func TestSmall4(t *testing.T) {
	unittest.SmallTest(t)
	time.Sleep(1 * time.Second)
}

func TestSmall5(t *testing.T) {
	unittest.SmallTest(t)
	time.Sleep(1 * time.Second)
}

func TestMedium1(t *testing.T) {
	unittest.MediumTest(t)
	time.Sleep(10 * time.Second)
}

func TestMedium2(t *testing.T) {
	unittest.MediumTest(t)
	time.Sleep(10 * time.Second)
}

func TestLarge1(t *testing.T) {
	unittest.LargeTest(t)
	time.Sleep(3 * time.Minute)
}

func TestLarge2(t *testing.T) {
	unittest.LargeTest(t)
	time.Sleep(3 * time.Minute)
}
