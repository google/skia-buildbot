package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	k8s_mocks "go.skia.org/infra/go/k8s/mocks"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	secret_mocks "go.skia.org/infra/go/secret/mocks"
)

func generateTestPEMBundle(t *testing.T, commonName string, notAfter time.Time) (bundlePEM, expectedCertPEM, expectedKeyPEM []byte) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames:              []string{commonName, "skia.org"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	intermediateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "GTS Intermediate CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, intermediateTemplate, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, intermediateTemplate, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	intermediatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER})
	expectedCertPEM = append(append([]byte{}, leafPEM...), intermediatePEM...)

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	expectedKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})

	bundlePEM = append(append([]byte{}, expectedCertPEM...), expectedKeyPEM...)
	return bundlePEM, expectedCertPEM, expectedKeyPEM
}

func TestSplitPEMBundle_Success(t *testing.T) {
	notAfter := time.Now().Add(45 * 24 * time.Hour).UTC().Truncate(time.Second)
	bundle, wantCert, wantKey := generateTestPEMBundle(t, "*.skia.org", notAfter)

	gotCert, gotKey, leaf, err := splitPEMBundle(bundle)
	require.NoError(t, err)
	require.Equal(t, wantCert, gotCert)
	require.Equal(t, wantKey, gotKey)
	require.NotNil(t, leaf)
	require.Equal(t, "*.skia.org", leaf.Subject.CommonName)
	require.Equal(t, notAfter, leaf.NotAfter.UTC())
}

func TestSplitPEMBundle_MissingCertificateOrKey_ReturnsError(t *testing.T) {
	notAfter := time.Now().Add(30 * 24 * time.Hour)
	_, certOnly, keyOnly := generateTestPEMBundle(t, "*.skia.org", notAfter)

	_, _, _, err := splitPEMBundle(certOnly)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private key")

	_, _, _, err = splitPEMBundle(keyOnly)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate")

	_, _, _, err = splitPEMBundle([]byte("not a valid pem"))
	require.Error(t, err)
}

func TestSyncSecret_CreatesSecretWhenNotFound(t *testing.T) {
	ctx := context.Background()
	notAfter := time.Now().Add(60*24*time.Hour + 2*time.Hour)
	bundle, wantCert, wantKey := generateTestPEMBundle(t, "*.skia.org", notAfter)

	secretClient := secret_mocks.NewClient(t)
	k8sClient := k8s_mocks.NewClient(t)

	secretClient.On("Get", ctx, "skia-infra-corp", "skia-org-tls", secret.VersionLatest).Return(string(bundle), nil)

	notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, "skia-org-tls")
	k8sClient.On("GetSecret", ctx, "default", "skia-org-tls", metav1.GetOptions{}).Return(nil, notFoundErr)

	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skia-org-tls",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       wantCert,
			corev1.TLSPrivateKeyKey: wantKey,
		},
	}
	k8sClient.On("CreateSecret", ctx, "default", expectedSecret, metav1.CreateOptions{}).Return(expectedSecret, nil)

	err := syncSecret(ctx, secretClient, k8sClient, "skia-infra-corp", "default", "skia-org-tls")
	require.NoError(t, err)

	metric := metrics2.GetInt64Metric(metricDaysUntilExpiration, map[string]string{
		"secret":    "skia-org-tls",
		"namespace": "default",
	})
	require.Equal(t, int64(60), metric.Get())
}

func TestSyncSecret_UpdatesSecretWhenBytesChange(t *testing.T) {
	ctx := context.Background()
	notAfter := time.Now().Add(89*24*time.Hour + 2*time.Hour)
	bundle, wantCert, wantKey := generateTestPEMBundle(t, "*.luci.app", notAfter)

	secretClient := secret_mocks.NewClient(t)
	k8sClient := k8s_mocks.NewClient(t)

	secretClient.On("Get", ctx, "skia-infra-corp", "luci-app-tls", secret.VersionLatest).Return(string(bundle), nil)

	// An Opaque type verifies the update leaves the immutable type field alone.
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "luci-app-tls",
			Namespace:       "envoy",
			ResourceVersion: "42",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			corev1.TLSCertKey:       []byte("old-cert"),
			corev1.TLSPrivateKeyKey: []byte("old-key"),
		},
	}
	k8sClient.On("GetSecret", ctx, "envoy", "luci-app-tls", metav1.GetOptions{}).Return(existingSecret, nil)

	k8sClient.On("UpdateSecret", ctx, "envoy", mock.MatchedBy(func(s *corev1.Secret) bool {
		return s.Name == "luci-app-tls" &&
			s.Namespace == "envoy" &&
			s.ResourceVersion == "42" &&
			s.Type == corev1.SecretTypeOpaque &&
			string(s.Data[corev1.TLSCertKey]) == string(wantCert) &&
			string(s.Data[corev1.TLSPrivateKeyKey]) == string(wantKey)
	}), metav1.UpdateOptions{}).Return(existingSecret, nil)

	err := syncSecret(ctx, secretClient, k8sClient, "skia-infra-corp", "envoy", "luci-app-tls")
	require.NoError(t, err)

	metric := metrics2.GetInt64Metric(metricDaysUntilExpiration, map[string]string{
		"secret":    "luci-app-tls",
		"namespace": "envoy",
	})
	require.Equal(t, int64(89), metric.Get())
}

func TestSyncSecret_NoOpWhenSecretMatches(t *testing.T) {
	ctx := context.Background()
	notAfter := time.Now().Add(75*24*time.Hour + 2*time.Hour)
	bundle, wantCert, wantKey := generateTestPEMBundle(t, "*.skia.org", notAfter)

	secretClient := secret_mocks.NewClient(t)
	k8sClient := k8s_mocks.NewClient(t)

	secretClient.On("Get", ctx, "skia-infra-corp", "skia-org-tls", secret.VersionLatest).Return(string(bundle), nil)

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skia-org-tls",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       wantCert,
			corev1.TLSPrivateKeyKey: wantKey,
		},
	}
	k8sClient.On("GetSecret", ctx, "default", "skia-org-tls", metav1.GetOptions{}).Return(existingSecret, nil)

	err := syncSecret(ctx, secretClient, k8sClient, "skia-infra-corp", "default", "skia-org-tls")
	require.NoError(t, err)
}

func TestSyncSecret_GetSecretError_ReturnsError(t *testing.T) {
	ctx := context.Background()
	notAfter := time.Now().Add(50 * 24 * time.Hour)
	bundle, _, _ := generateTestPEMBundle(t, "*.skia.org", notAfter)

	secretClient := secret_mocks.NewClient(t)
	k8sClient := k8s_mocks.NewClient(t)

	secretClient.On("Get", ctx, "skia-infra-corp", "skia-org-tls", secret.VersionLatest).Return(string(bundle), nil)

	forbiddenErr := apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "secrets"}, "skia-org-tls", errors.New("rbac"))
	k8sClient.On("GetSecret", ctx, "default", "skia-org-tls", metav1.GetOptions{}).Return(nil, forbiddenErr)

	err := syncSecret(ctx, secretClient, k8sClient, "skia-infra-corp", "default", "skia-org-tls")
	require.Error(t, err)
	require.Contains(t, err.Error(), "getting TLS secret default/skia-org-tls")
}

func TestSyncSecret_SecretManagerError_ReturnsError(t *testing.T) {
	ctx := context.Background()

	secretClient := secret_mocks.NewClient(t)
	k8sClient := k8s_mocks.NewClient(t)

	secretClient.On("Get", ctx, "skia-infra-corp", "skia-org-tls", secret.VersionLatest).Return("", errors.New("permission denied"))

	err := syncSecret(ctx, secretClient, k8sClient, "skia-infra-corp", "default", "skia-org-tls")
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestDaysUntil(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		notAfter time.Time
		want     int64
	}{
		{name: "36h ahead", notAfter: now.Add(36 * time.Hour), want: 1},
		{name: "18h ahead", notAfter: now.Add(18 * time.Hour), want: 0},
		{name: "18h ago", notAfter: now.Add(-18 * time.Hour), want: -1},
		{name: "36h ago", notAfter: now.Add(-36 * time.Hour), want: -2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, daysUntil(tc.notAfter, now))
		})
	}
}
