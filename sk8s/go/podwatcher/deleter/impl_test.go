package deleter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDelete(t *testing.T) {
	unittest.SmallTest(t)

	const podName = "rpi-swarming-12346-987"
	clientSet := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	})

	impl := &Impl{
		clientSet: clientSet,
	}
	ctx := context.Background()
	err := impl.Delete(ctx, podName)
	require.NoError(t, err)

	err = impl.Delete(ctx, "this-is-an-unknown-pod-name")
	require.Error(t, err)
}
