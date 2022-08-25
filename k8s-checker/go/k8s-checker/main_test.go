// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseNamespaceAllowFilterFlag_MalFormed_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := parseNamespaceAllowFilterFlag([]string{
		"this flag value contains no colon",
	})
	require.Error(t, err)
}

func TestParseNamespaceAllowFilterFlag_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	actual, err := parseNamespaceAllowFilterFlag([]string{
		"gmp-system:rule-evaluator",
		"gmp-system:collector",
		"kube-system:calico-node",
		"kube-system:calico-typha",
		"kube-system:fluentbit",
		"kube-system:gke-metadata-server",
		"kube-system:ip-masq-agent",
		"kube-system:kube-dns",
	})
	require.NoError(t, err)
	expected := allowedAppsInNamespace{
		"gmp-system":  []string{"rule-evaluator", "collector"},
		"kube-system": []string{"calico-node", "calico-typha", "fluentbit", "gke-metadata-server", "ip-masq-agent", "kube-dns"},
	}
	require.Equal(t, expected, actual)
}

func TestAddMetricForImageAge_NameIsSHA256_UpdatesMetricWithAZeroValue(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	metrics := map[metrics2.Int64Metric]struct{}{}
	imageNameWithSHA256 := "gcr.io/skia-public/k8s-deployer@sha256:9f506a343f3e63174384d85e4ae75a1c1d16b896122170fe7ecc282bdfbdcf2d"

	err := addMetricForImageAge(ctx, "my-app", "my-app-container", "my-namepspace", "my-yaml", "my-repo", imageNameWithSHA256, metrics)
	require.NoError(t, err)

	// Confirm only one metric was added and that it has a zero value.
	require.Len(t, metrics, 1)
	for staleMetric := range metrics {
		require.Equal(t, int64(0), staleMetric.Get())
	}
}

func TestAddMetricForImageAge_NameHasDateEncoded_UpdatesMetricWithDaysSinceThatDate(t *testing.T) {
	unittest.SmallTest(t)
	imageTime := time.Date(2020, time.January, 1, 1, 1, 0, 0, time.UTC)
	ctx := now.TimeTravelingContext(imageTime.Add(time.Hour * 49)) // Two days (plus a smidge).

	//  gcr.io/skia-public/emailservice:2022-07-06T16_08_06Z-jcgregorio-e0bf15f-clean
	imageName := fmt.Sprintf("gcr.io/skia-public/emailservice:%s-jcgregorio-e0bf15f-clean", imageTime.Format("2006-01-02T15_04_05Z07:00"))

	metrics := map[metrics2.Int64Metric]struct{}{}

	err := addMetricForImageAge(ctx, "my-app", "my-app-container", "my-namepspace", "my-yaml", "my-repo", imageName, metrics)
	require.NoError(t, err)

	// Confirm only one metric was added and that it has a value of 2 days.
	require.Len(t, metrics, 1)
	for staleMetric := range metrics {
		require.Equal(t, int64(2), staleMetric.Get())
	}
}

func TestAddMetricForImageAge_NameHasInvalidDateEncoded_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	invalidDate := "gcr.io/skia-public/emailservice:ThisIsNotAValidDateZ-jcgregorio-e0bf15f-clean"

	metrics := map[metrics2.Int64Metric]struct{}{}
	err := addMetricForImageAge(context.Background(), "my-app", "my-app-container", "my-namepspace", "my-yaml", "my-repo", invalidDate, metrics)
	require.Error(t, err)
}
