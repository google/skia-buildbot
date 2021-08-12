package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
	"golang.org/x/oauth2"
)

type testTokenSource struct {
	token *oauth2.Token
}

func newTestToken() *testTokenSource {
	return &testTokenSource{
		token: &oauth2.Token{
			AccessToken: "foo",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(10 * time.Minute),
		},
	}
}

func (t *testTokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

func TestBaseImageName(t *testing.T) {
	unittest.SmallTest(t)
	Init()

	assert.Equal(t, "", baseImageName(""))
	assert.Equal(t, "", baseImageName("debian"))
	assert.Equal(t, "fiddler", baseImageName("gcr.io/skia-public/fiddler"))
	assert.Equal(t, "fiddler", baseImageName("gcr.io/skia-public/fiddler:prod"))
	assert.Equal(t, "docserver", baseImageName("gcr.io/skia-public/docserver:123456"))
}

func TestAddDockerProdTag(t *testing.T) {
	unittest.SmallTest(t)
	Init()
	docker = "docker"

	loginCalled := false
	pullCalled := false
	tagCalled := false
	pushCalled := false
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		assert.Equal(t, "docker", cmd.Name)
		if cmd.Args[0] == "login" {
			assert.Equal(t, []string{"login", "-u", "oauth2accesstoken", "-p", "foo", "https://gcr.io"}, cmd.Args)
			loginCalled = true
		} else if cmd.Args[0] == "pull" {
			assert.Equal(t, []string{"pull", "TestImageName:TestTag"}, cmd.Args)
			pullCalled = true
		} else if cmd.Args[0] == "tag" {
			assert.Equal(t, []string{"tag", "TestImageName:TestTag", "TestImageName:prod"}, cmd.Args)
			tagCalled = true
		} else if cmd.Args[0] == "push" {
			assert.Equal(t, []string{"push", "TestImageName:prod"}, cmd.Args)
			pushCalled = true
		}
		return nil
	})
	mockRunCtx := exec.NewContext(context.Background(), mockRun.Run)

	b := pubsub.BuildInfo{
		ImageName: "TestImageName",
		Tag:       "TestTag",
	}
	err := addDockerProdTag(mockRunCtx, newTestToken(), b)
	require.NoError(t, err)
	assert.True(t, loginCalled)
	assert.True(t, pullCalled)
	assert.True(t, tagCalled)
	assert.True(t, pushCalled)
}

func TestDeployImage(t *testing.T) {
	unittest.SmallTest(t)
	Init()
	pushk = "pushk"

	pushkCalled := false
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		assert.Equal(t, "pushk", cmd.Name)
		assert.Equal(t, []string{"--do-not-override-dirty-image", "--running-in-k8s", "test-app"}, cmd.Args)
		pushkCalled = true
		return nil
	})
	mockRunCtx := exec.NewContext(context.Background(), mockRun.Run)

	err := deployImage(mockRunCtx, "test-app")
	require.NoError(t, err)
	assert.True(t, pushkCalled)
}
