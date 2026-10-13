# cert-apply

Application that runs in `skia-public` (`namespace: default`) and
`skia-infra-public` (`namespace: envoy`) and periodically polls GCP Secret
Manager (`skia-infra-corp`) for updated TLS certificate PEM bundles
(`skia-org-tls` and `luci-app-tls`), synchronizing them into Kubernetes
`kubernetes.io/tls` Secrets consumed by GKE Ingress (`skia-ingress` and
`skia-ingress-ipv6`).

See http://go/skia-ssl-cert for the full certificate renewal architecture and
operational runbook.

## Flags

- `--project` (default `skia-infra-corp`): GCP project ID containing the Secret
  Manager TLS secrets.
- `--namespace` (default `default`): Kubernetes namespace for TLS Secrets
  (`default` in `skia-public`, `envoy` in `skia-infra-public`).
- `--secret` (repeatable, required): Name of a secret to copy; the same name is
  used in Secret Manager and Kubernetes (`skia-org-tls`, `luci-app-tls`).
- `--interval` (default `5m`): How often to sync certificates from Secret
  Manager.
- `--prom_port` (default `:20000`): Metrics service address.

## Monitoring and Alerting

The application exports the following Prometheus metrics with alert rules
defined in `k8s-config/monitoring/appgroups/cert-apply.yml`.

## Permissions

1. **GCP Secret Manager (Workload Identity)**:
   - `cert-apply@skia-public.iam.gserviceaccount.com` (bound to Kubernetes SA
     `default/cert-apply` in `skia-public`) and
     `cert-apply@skia-infra-public.iam.gserviceaccount.com` (bound to Kubernetes
     SA `envoy/cert-apply` in `skia-infra-public`) require
     `roles/secretmanager.secretAccessor` on `skia-org-tls` and `luci-app-tls`
     in project `skia-infra-corp`.
2. **Kubernetes RBAC**:
   - The `cert-apply` `Role` in the target namespace grants `get` and `update`
     on `secrets` (`resourceNames: ["skia-org-tls", "luci-app-tls"]`) and
     `create` on `secrets`.
