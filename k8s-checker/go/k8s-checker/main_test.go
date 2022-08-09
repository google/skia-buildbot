// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
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
		"gmp-system:rule-evaluator,collector",
		"kube-system:calico-node,calico-typha,fluentbit,gke-metadata-server,ip-masq-agent,kube-dns",
	})
	require.NoError(t, err)
	expected := allowedAppsInNamespace{
		"gmp-system":  []string{"rule-evaluator", "collector"},
		"kube-system": []string{"calico-node", "calico-typha", "fluentbit", "gke-metadata-server", "ip-masq-agent", "kube-dns"},
	}
	require.Equal(t, expected, actual)
}
