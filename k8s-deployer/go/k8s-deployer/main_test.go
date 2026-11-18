// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"
)

func TestWriteKubeconfig_ValidKubeconfigWithTokenAndCA(t *testing.T) {
	dir := t.TempDir()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-token"})

	p, err := writeKubeconfig(dir, "https://172.16.200.18:443", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", ts)
	require.NoError(t, err)

	fi, err := os.Stat(p)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), fi.Mode().Perm())

	b, err := os.ReadFile(p)
	require.NoError(t, err)
	var cfg struct {
		CurrentContext string `json:"current-context"`
		Clusters       []struct {
			Name    string `json:"name"`
			Cluster struct {
				Server               string `json:"server"`
				CertificateAuthority string `json:"certificate-authority"`
			} `json:"cluster"`
		} `json:"clusters"`
		Users []struct {
			Name string `json:"name"`
			User struct {
				Token string `json:"token"`
			} `json:"user"`
		} `json:"users"`
		Contexts []struct {
			Name    string `json:"name"`
			Context struct {
				Cluster string `json:"cluster"`
				User    string `json:"user"`
			} `json:"context"`
		} `json:"contexts"`
	}
	require.NoError(t, yaml.Unmarshal(b, &cfg))
	require.Equal(t, "default", cfg.CurrentContext)
	require.Len(t, cfg.Clusters, 1)
	require.Equal(t, "https://172.16.200.18:443", cfg.Clusters[0].Cluster.Server)
	require.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", cfg.Clusters[0].Cluster.CertificateAuthority)
	require.Len(t, cfg.Users, 1)
	require.Equal(t, "fake-token", cfg.Users[0].User.Token)
	require.Len(t, cfg.Contexts, 1)
	require.Equal(t, cfg.Clusters[0].Name, cfg.Contexts[0].Context.Cluster)
	require.Equal(t, cfg.Users[0].Name, cfg.Contexts[0].Context.User)
}

func TestWriteKubeconfig_TokenError_ReturnsError(t *testing.T) {
	_, err := writeKubeconfig(t.TempDir(), "https://1.2.3.4", "/ca.crt", errTokenSource{})
	require.Error(t, err)
}

func TestWriteKubeconfig_EmptyToken_ReturnsError(t *testing.T) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ""})
	_, err := writeKubeconfig(t.TempDir(), "https://1.2.3.4", "/ca.crt", ts)
	require.ErrorContains(t, err, "empty access token")
}

type errTokenSource struct{}

func (errTokenSource) Token() (*oauth2.Token, error) {
	return nil, os.ErrPermission
}
