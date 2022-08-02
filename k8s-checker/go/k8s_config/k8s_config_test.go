package k8s_config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const k8sConfig = `
apiVersion: v1
kind: Service
metadata:
  name: datahopper
  labels:
    app: datahopper
  annotations:
    beta.cloud.google.com/backend-config: '{"ports": {"8000":"skia-default-backendconfig"}}'
spec:
  selector:
    app: datahopper
  type: NodePort
  ports:
    - port: 8000
      name: http
    - port: 20000
      name: metrics
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: datahopper
  labels:
    app: datahopper
spec:
  replicas: 1
  selector:
    matchLabels:
      app: datahopper
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: datahopper
      annotations:
        prometheus.io.scrape: "true"
        prometheus.io.port: "20000"
    spec:
      securityContext:
        runAsUser: 2000 # aka skia
        fsGroup: 2000   # aka skia
      automountServiceAccountToken: false
      containers:
        - name: datahopper
          image: gcr.io/skia-public/datahopper:2022-06-24T14_25_21Z-borenet-17bb7f4-clean
          command: ["/usr/local/bin/datahopper"]
          args:
            - "--fake-flag"
          ports:
            - containerPort: 8000
            - containerPort: 20000
              name: prom
          resources:
            requests:
              memory: "48Gi"
              cpu: 8
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8000
            initialDelaySeconds: 30
            periodSeconds: 3
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: autoroll-be-skia-autoroll
spec:
  serviceName: "autoroll-be-skia-autoroll"
  replicas: 1
  selector:
    matchLabels:
      app: autoroll-be-skia-autoroll
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: autoroll-be-skia-autoroll  # Pod template's label selector
        appgroup: autoroll
        owner-primary: borenet
        owner-secondary: rmistry
      annotations:
        prometheus.io.scrape: "true"
        prometheus.io.port: "20000"
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsUser: 2000 # aka skia
        fsGroup: 2000   # aka skia

      containers:
        - name: autoroll-be-skia-autoroll
          image: gcr.io/skia-public/autoroll-be:2022-06-21T13_56_34Z-borenet-33d442f-clean
          command: ["luci-auth"]
          args:
            - "context"
            - "-service-account-json"
            - "/var/secrets/google/key.json"
            - "--"
            - "/usr/local/bin/autoroll-be"
            - "--config=cm9sbGVyX25hbWU6InNraWEtYXV0b3JvbGwiICBjaGlsZF9idWdfbGluazoiaHR0cHM6Ly9idWdzLmNocm9taXVtLm9yZy9wL3NraWEvaXNzdWVzL2VudHJ5IiAgY2hpbGRfZGlzcGxheV9uYW1lOiJTa2lhIiAgcGFyZW50X2J1Z19saW5rOiJodHRwczovL2J1Z3MuY2hyb21pdW0ub3JnL3AvY2hyb21pdW0vaXNzdWVzL2VudHJ5IiAgcGFyZW50X2Rpc3BsYXlfbmFtZToiQ2hyb21pdW0iICBwYXJlbnRfd2F0ZXJmYWxsOiJodHRwczovL2J1aWxkLmNocm9taXVtLm9yZyIgIG93bmVyX3ByaW1hcnk6ImJvcmVuZXQiICBvd25lcl9zZWNvbmRhcnk6InJtaXN0cnkiICBjb250YWN0czoiYm9yZW5ldEBnb29nbGUuY29tIiAgc2VydmljZV9hY2NvdW50OiJjaHJvbWl1bS1hdXRvcm9sbEBza2lhLXB1YmxpYy5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIgIHJldmlld2VyOiJodHRwczovL2Nocm9tZS1vcHMtcm90YXRpb24tcHJveHkuYXBwc3BvdC5jb20vY3VycmVudC9ncm90YXRpb246c2tpYS1nYXJkZW5lciIgIHN1cHBvcnRzX21hbnVhbF9yb2xsczp0cnVlICBjb21taXRfbXNnOntidWdfcHJvamVjdDoiY2hyb21pdW0iICBjaGlsZF9sb2dfdXJsX3RtcGw6Imh0dHBzOi8vc2tpYS5nb29nbGVzb3VyY2UuY29tL3NraWEuZ2l0Lytsb2cve3suUm9sbGluZ0Zyb219fS4ue3suUm9sbGluZ1RvfX0iICBjcV9leHRyYV90cnlib3RzOiJsdWNpLmNocm9taXVtLnRyeTphbmRyb2lkX29wdGlvbmFsX2dwdV90ZXN0c19yZWwiICBjcV9leHRyYV90cnlib3RzOiJsdWNpLmNocm9taXVtLnRyeTpsaW51eC1ibGluay1yZWwiICBjcV9leHRyYV90cnlib3RzOiJsdWNpLmNocm9taXVtLnRyeTpsaW51eC1jaHJvbWVvcy1jb21waWxlLWRiZyIgIGNxX2V4dHJhX3RyeWJvdHM6Imx1Y2kuY2hyb21pdW0udHJ5OmxpbnV4X29wdGlvbmFsX2dwdV90ZXN0c19yZWwiICBjcV9leHRyYV90cnlib3RzOiJsdWNpLmNocm9taXVtLnRyeTptYWNfb3B0aW9uYWxfZ3B1X3Rlc3RzX3JlbCIgIGNxX2V4dHJhX3RyeWJvdHM6Imx1Y2kuY2hyb21pdW0udHJ5Ondpbl9vcHRpb25hbF9ncHVfdGVzdHNfcmVsIiAgY3FfZG9fbm90X2NhbmNlbF90cnlib3RzOnRydWUgIGluY2x1ZGVfbG9nOnRydWUgIGluY2x1ZGVfcmV2aXNpb25fY291bnQ6dHJ1ZSAgaW5jbHVkZV90YnJfbGluZTp0cnVlICBpbmNsdWRlX3Rlc3RzOnRydWUgIGJ1aWx0X2luOkRFRkFVTFR9ICBnZXJyaXQ6e3VybDoiaHR0cHM6Ly9jaHJvbWl1bS1yZXZpZXcuZ29vZ2xlc291cmNlLmNvbSIgIHByb2plY3Q6ImNocm9taXVtL3NyYyIgIGNvbmZpZzpDSFJPTUlVTV9CT1RfQ09NTUlUfSAga3ViZXJuZXRlczp7Y3B1OiIxIiAgbWVtb3J5OiIyR2kiICByZWFkaW5lc3NfZmFpbHVyZV90aHJlc2hvbGQ6MTAgIHJlYWRpbmVzc19pbml0aWFsX2RlbGF5X3NlY29uZHM6MzAgIHJlYWRpbmVzc19wZXJpb2Rfc2Vjb25kczozMCAgaW1hZ2U6Imdjci5pby9za2lhLXB1YmxpYy9hdXRvcm9sbC1iZToyMDIyLTA2LTIxVDEzXzU2XzM0Wi1ib3JlbmV0LTMzZDQ0MmYtY2xlYW4ifSAgcGFyZW50X2NoaWxkX3JlcG9fbWFuYWdlcjp7Z2l0aWxlc19wYXJlbnQ6e2dpdGlsZXM6e2JyYW5jaDoibWFpbiIgIHJlcG9fdXJsOiJodHRwczovL2Nocm9taXVtLmdvb2dsZXNvdXJjZS5jb20vY2hyb21pdW0vc3JjLmdpdCJ9ICBkZXA6e3ByaW1hcnk6e2lkOiJodHRwczovL3NraWEuZ29vZ2xlc291cmNlLmNvbS9za2lhLmdpdCIgIHBhdGg6IkRFUFMifX0gIGdlcnJpdDp7dXJsOiJodHRwczovL2Nocm9taXVtLXJldmlldy5nb29nbGVzb3VyY2UuY29tIiAgcHJvamVjdDoiY2hyb21pdW0vc3JjIiAgY29uZmlnOkNIUk9NSVVNX0JPVF9DT01NSVR9fSAgZ2l0aWxlc19jaGlsZDp7Z2l0aWxlczp7YnJhbmNoOiJtYWluIiAgcmVwb191cmw6Imh0dHBzOi8vc2tpYS5nb29nbGVzb3VyY2UuY29tL3NraWEuZ2l0In19fSAgbm90aWZpZXJzOntsb2dfbGV2ZWw6V0FSTklORyAgZW1haWw6e2VtYWlsczoiYm9yZW5ldEBnb29nbGUuY29tIn19"
            - "--firestore_instance=production"
            - "--port=:8000"
            - "--prom_port=:20000"
            - "--recipes_cfg=/usr/local/share/autoroll/recipes.cfg"
            - "--workdir=/tmp"
          ports:
            - containerPort: 8000
            - containerPort: 20000
              name: prom
          volumeMounts:
            - name: autoroll-be-chromium-autoroll-sa
              mountPath: /var/secrets/google
          env:
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /var/secrets/google/key.json
            - name: TMPDIR
              value: /tmp
          resources:
            requests:
              memory: "2Gi"
              cpu: 1
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8000
            initialDelaySeconds: 30.000000
            periodSeconds: 30.000000
            failureThreshold: 10.000000
      volumes:
        - name: autoroll-be-chromium-autoroll-sa
          secret:
            secretName: chromium-autoroll
---
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: comp-ui-gitcron
spec:
  schedule: '0 5 * * *'
  concurrencyPolicy: 'Forbid'
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          serviceAccountName: perf-comp-ui
          securityContext:
            runAsUser: 2000 # aka skia
            fsGroup: 2000 # aka skia
          containers:
            - name: comp-ui-gitcron
              image: gcr.io/skia-public/comp-ui-gitcron:2022-04-08T11_33_16Z-jcgregorio-72d31c9-clean
              env:
                - name: HOME
                  value: /home/skia
`

func TestParseK8sConfigFile_Success(t *testing.T) {
	unittest.SmallTest(t)

	deployments, statefulSets, cronJobs, daemonSets, err := ParseK8sConfigFile([]byte(k8sConfig))
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	require.Len(t, statefulSets, 1)
	require.Len(t, cronJobs, 1)
	require.Len(t, daemonSets, 0)
	require.Equal(t, "gcr.io/skia-public/datahopper:2022-06-24T14_25_21Z-borenet-17bb7f4-clean", deployments[0].Spec.Template.Spec.Containers[0].Image)
	require.Equal(t, "gcr.io/skia-public/autoroll-be:2022-06-21T13_56_34Z-borenet-33d442f-clean", statefulSets[0].Spec.Template.Spec.Containers[0].Image)
	require.Equal(t, "gcr.io/skia-public/comp-ui-gitcron:2022-04-08T11_33_16Z-jcgregorio-72d31c9-clean", cronJobs[0].Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)
}
