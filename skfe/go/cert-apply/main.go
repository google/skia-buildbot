// The cert-apply application periodically reads TLS PEM bundles from GCP Secret
// Manager, splits them into certificate chains (tls.crt) and private keys
// (tls.key), and upserts kubernetes.io/tls Secrets in the target GKE namespace.
package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"math"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/k8s"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	metricDaysUntilExpiration = "cert_apply_days_until_expiration"
	livenessName              = "cert_apply_sync"
)

func daysUntil(notAfter, now time.Time) int64 {
	return int64(math.Floor(notAfter.Sub(now).Hours() / 24))
}

func splitPEMBundle(raw []byte) (certPEM, keyPEM []byte, leaf *x509.Certificate, err error) {
	rest := raw
	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch {
		case block.Type == "CERTIFICATE":
			if leaf == nil {
				parsed, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return nil, nil, nil, skerr.Wrapf(err, "parsing leaf certificate")
				}
				leaf = parsed
			}
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
		case strings.HasSuffix(block.Type, "PRIVATE KEY"):
			keyPEM = append(keyPEM, pem.EncodeToMemory(block)...)
		}
	}

	if leaf == nil {
		return nil, nil, nil, skerr.Fmt("no certificate block found in PEM bundle")
	}
	if len(keyPEM) == 0 {
		return nil, nil, nil, skerr.Fmt("no private key block found in PEM bundle")
	}
	return certPEM, keyPEM, leaf, nil
}

func syncSecret(ctx context.Context, sc secret.Client, kc k8s.Client, project, namespace, name string) error {
	raw, err := sc.Get(ctx, project, name, secret.VersionLatest)
	if err != nil {
		return skerr.Wrapf(err, "reading secret %s from project %s", name, project)
	}
	certPEM, keyPEM, leaf, err := splitPEMBundle([]byte(raw))
	if err != nil {
		return skerr.Wrapf(err, "splitting PEM bundle from secret %s", name)
	}

	metrics2.GetInt64Metric(metricDaysUntilExpiration, map[string]string{
		"secret":    name,
		"namespace": namespace,
	}).Update(daysUntil(leaf.NotAfter, time.Now()))

	existing, err := kc.GetSecret(ctx, namespace, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		}
		if _, err := kc.CreateSecret(ctx, namespace, newSecret, metav1.CreateOptions{}); err != nil {
			return skerr.Wrapf(err, "creating TLS secret %s/%s", namespace, name)
		}
		sklog.Infof("Created TLS Secret %s/%s (expires %s)", namespace, name, leaf.NotAfter.Format(time.RFC3339))
		return nil
	}
	if err != nil {
		return skerr.Wrapf(err, "getting TLS secret %s/%s", namespace, name)
	}

	if bytes.Equal(existing.Data[corev1.TLSCertKey], certPEM) &&
		bytes.Equal(existing.Data[corev1.TLSPrivateKeyKey], keyPEM) {
		return nil
	}

	updated := existing.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string][]byte{}
	}
	updated.Data[corev1.TLSCertKey] = certPEM
	updated.Data[corev1.TLSPrivateKeyKey] = keyPEM

	if _, err := kc.UpdateSecret(ctx, namespace, updated, metav1.UpdateOptions{}); err != nil {
		return skerr.Wrapf(err, "updating TLS secret %s/%s", namespace, name)
	}
	sklog.Infof("Updated TLS Secret %s/%s (expires %s)", namespace, name, leaf.NotAfter.Format(time.RFC3339))
	return nil
}

func main() {
	project := flag.String("project", "skia-infra-corp", "GCP project ID containing the Secret Manager TLS secrets.")
	namespace := flag.String("namespace", "default", "Target Kubernetes namespace for TLS Secrets.")
	secrets := common.NewMultiStringFlag("secret", nil, "Name of a secret to copy; the same name is used in Secret Manager and Kubernetes.")
	interval := flag.Duration("interval", 5*time.Minute, "How often to sync certificates from Secret Manager.")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000').")

	common.InitWithMust(
		"cert-apply",
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	if len(*secrets) == 0 {
		sklog.Fatal("At least one --secret flag is required.")
	}
	if *interval <= 0 {
		sklog.Fatalf("--interval must be positive, got %s.", *interval)
	}

	ctx := context.Background()
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create Secret Manager client: %s", err)
	}

	k8sClient, err := k8s.NewInClusterClient(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create in-cluster Kubernetes client: %s", err)
	}

	liveness := metrics2.NewLiveness(livenessName)
	done := make(chan struct{})
	cleanup.Repeat(*interval, func(ctx context.Context) {
		allSucceeded := true
		for _, name := range *secrets {
			if err := syncSecret(ctx, secretClient, k8sClient, *project, *namespace, name); err != nil {
				sklog.Errorf("Failed to sync secret %s/%s: %s", *namespace, name, err)
				allSucceeded = false
			}
		}
		if allSucceeded {
			liveness.Reset()
		}
	}, func() {
		util.Close(secretClient)
		close(done)
	})
	<-done
}
