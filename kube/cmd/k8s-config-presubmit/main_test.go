package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/k8s-checker/go/k8s_config"
)

func TestValidateWorkerDeploymentConnectionRef_MissingConnection_ReturnsError(t *testing.T) {
	clusterDir := t.TempDir()
	wdFile := filepath.Join(clusterDir, "perf-bisect-worker.yml")
	const wdYAML = `apiVersion: temporal.io/v1alpha1
kind: WorkerDeployment
metadata:
  name: perf-bisect-worker
  namespace: perf
spec:
  workerOptions:
    connectionRef:
      name: temporal-connection
  template:
    spec:
      containers:
        - name: worker
          image: gcr.io/skia-public/bisect_workflow@sha256:5c97df33dc3e6c26d921de0e09b9f5dc10de6e8515248cc4d410baadb045468e
`
	require.NoError(t, os.WriteFile(wdFile, []byte(wdYAML), 0644))
	k8sConfigs, _, err := k8s_config.ParseK8sConfigFile([]byte(wdYAML))
	require.NoError(t, err)
	require.Len(t, k8sConfigs.WorkerDeployment, 1)

	err = validateWorkerDeploymentConnectionRef(wdFile, k8sConfigs.WorkerDeployment[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), `references Connection "temporal-connection" in namespace "perf", which was not found`)
}

func TestValidateWorkerDeploymentConnectionRef_ConnectionPresent_Success(t *testing.T) {
	clusterDir := t.TempDir()
	wdFile := filepath.Join(clusterDir, "perf-bisect-worker.yml")
	connFile := filepath.Join(clusterDir, "temporal-connection-perf.yaml")
	const wdYAML = `apiVersion: temporal.io/v1alpha1
kind: WorkerDeployment
metadata:
  name: perf-bisect-worker
  namespace: perf
spec:
  workerOptions:
    connectionRef:
      name: temporal-connection
  template:
    spec:
      containers:
        - name: worker
          image: gcr.io/skia-public/bisect_workflow@sha256:5c97df33dc3e6c26d921de0e09b9f5dc10de6e8515248cc4d410baadb045468e
`
	const connYAML = `apiVersion: temporal.io/v1alpha1
kind: Connection
metadata:
  name: temporal-connection
  namespace: perf
spec:
  hostPort: temporal.temporal:7233
`
	require.NoError(t, os.WriteFile(wdFile, []byte(wdYAML), 0644))
	require.NoError(t, os.WriteFile(connFile, []byte(connYAML), 0644))
	k8sConfigs, _, err := k8s_config.ParseK8sConfigFile([]byte(wdYAML))
	require.NoError(t, err)
	require.Len(t, k8sConfigs.WorkerDeployment, 1)

	require.NoError(t, validateWorkerDeploymentConnectionRef(wdFile, k8sConfigs.WorkerDeployment[0]))
}
