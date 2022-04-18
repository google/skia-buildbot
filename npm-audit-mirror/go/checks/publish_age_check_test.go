package checks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

var (
	publishAgeCheckTests = []struct {
		publishAge  time.Duration
		checkPassed bool
		name        string
	}{
		{
			publishAge:  publishTimeCuttoff + time.Hour,
			checkPassed: true,
			name:        "Publish age is 1 hour more than the cutoff time",
		},
		{
			publishAge:  publishTimeCuttoff,
			checkPassed: true,
			name:        "Publish age is the same as the cutoff time",
		},
		{
			publishAge:  publishTimeCuttoff - time.Hour,
			checkPassed: false,
			name:        "Publish age is 1 hour less than the cutoff time",
		},
		{
			publishAge:  time.Hour,
			checkPassed: false,
			name:        "Publish age is only 1 hour ago",
		},
	}
)

func TestPerformPublishAgeCheck_PackageExists(t *testing.T) {
	unittest.SmallTest(t)

	pac := PublishAgeCheck{}
	testPackageVersion := "1.1.0"
	for _, test := range publishAgeCheckTests {
		publishTime := time.Now().Add(-test.publishAge)
		npm := &types.NpmPackage{
			Time: map[string]string{
				testPackageVersion: publishTime.Format(time.RFC3339),
			},
		}
		checkPassed, result, err := pac.PerformCheck("test-package", testPackageVersion, npm)
		require.NoError(t, err, test.name)
		require.Equal(t, test.checkPassed, checkPassed, test.name)
		if checkPassed {
			require.Empty(t, result, test.name)
		} else {
			require.NotEmpty(t, result, test.name)
		}
	}
}

func TestPerformPublishAgeCheck_PackageDoesNotExist(t *testing.T) {
	unittest.SmallTest(t)

	pac := PublishAgeCheck{}
	testPackageVersion := "1.1.0"
	for _, test := range publishAgeCheckTests {
		publishTime := time.Now().Add(-test.publishAge)
		npm := &types.NpmPackage{
			Time: map[string]string{
				testPackageVersion: publishTime.Format(time.RFC3339),
			},
		}
		checkPassed, result, err := pac.PerformCheck("test-package", "does-not-exist", npm)
		require.Error(t, err, test.name)
		require.False(t, checkPassed, test.name)
		require.Empty(t, result, test.name)
	}
}
