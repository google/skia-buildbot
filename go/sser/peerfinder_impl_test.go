package sser

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/k8s/mocks"
	watchmocks "go.skia.org/infra/go/k8s/watch/mocks"
	"go.skia.org/infra/go/testutils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	testIPAddress1 = "192.168.1.1"
	testIPAddress2 = "192.168.1.2"

	testLabelSelector       = "app=myTestApp"
	testNamespace           = "default"
	testResourceVersion     = "123"
	testResourceVersionNext = "124"
)

var (
	myMockErr = errors.New("my mock error")
)

func buildClientSetOnPodListMocks(t *testing.T, items []v1.Pod, err error) (kubernetes.Interface, *mocks.PodInterface) {
	// Build up all the mocks to handle this chain of calls: p.clientset.CoreV1().Pods(...).List(...)
	podList := &v1.PodList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: testResourceVersionNext,
		},
		Items: items,
	}
	podInterface := mocks.NewPodInterface(t)
	podInterface.On("List", testutils.AnyContext, metav1.ListOptions{
		LabelSelector: testLabelSelector,
	}).Return(podList, err)

	coreV1 := mocks.NewCoreV1Interface(t)
	coreV1.On("Pods", testNamespace).Return(podInterface)

	clientset := mocks.NewInterface(t)
	clientset.On("CoreV1").Return(coreV1)

	return clientset, podInterface
}

var listOptionsNext = metav1.ListOptions{
	LabelSelector:   testLabelSelector,
	ResourceVersion: testResourceVersionNext,
}

func addWatchMocksSuccess(t *testing.T, podsMock *mocks.PodInterface) {
	watch := watchmocks.NewInterface(t)
	podsMock.On("Watch", testutils.AnyContext, listOptionsNext).Return(watch, nil)
	watch.On("Stop")
}

func addWatchMocksError(t *testing.T, podsMock *mocks.PodInterface) {
	watch := watchmocks.NewInterface(t)
	podsMock.On("Watch", testutils.AnyContext, listOptionsNext).Return(watch, myMockErr)
}

func buildPeerFinderImplOnPodListMocks(t *testing.T, items []v1.Pod, err error) *PeerFinderImpl {
	clientset, _ := buildClientSetOnPodListMocks(t, items, err)
	p, err := NewPeerFinder(clientset, testNamespace, testLabelSelector)
	require.NoError(t, err)

	return p
}

func TestPeerFinderImplFindAllPeerPodIPAddresses_HappyPath(t *testing.T) {
	items := []v1.Pod{
		{
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				PodIP: testIPAddress1,
			},
		},
		{
			Status: v1.PodStatus{
				Phase: v1.PodFailed, // This pod will be ignored because it's not Running.
				PodIP: testIPAddress2,
			},
		},
		{
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				PodIP: "", // This pod will be ignored because the IP address is empty.
			},
		},
	}

	p := buildPeerFinderImplOnPodListMocks(t, items, nil)

	peers, version, err := p.findAllPeerPodIPAddresses(context.Background())
	require.NoError(t, err)
	require.Equal(t, testResourceVersionNext, version)
	require.Len(t, peers, 1)
	require.Equal(t, testIPAddress1, peers[0])
}

func TestPeerFinderImplFindAllPeerPodIPAddresses_ListReturnsError_ReturnsError(t *testing.T) {
	p := buildPeerFinderImplOnPodListMocks(t, []v1.Pod{}, myMockErr)

	_, _, err := p.findAllPeerPodIPAddresses(context.Background())
	require.Contains(t, err.Error(), myMockErr.Error())
}

func TestPeerFinderImplStart_ListReturnsError_ReturnsError(t *testing.T) {
	p := buildPeerFinderImplOnPodListMocks(t, []v1.Pod{}, myMockErr)

	_, _, err := p.Start(context.Background())
	require.Contains(t, err.Error(), myMockErr.Error())
}

func TestPeerFinderImplStart_WatchReturnsError_ResourceVersionGetsCleared(t *testing.T) {
	clientset, podMock := buildClientSetOnPodListMocks(t, []v1.Pod{}, nil)
	addWatchMocksError(t, podMock)
	p, err := NewPeerFinder(clientset, testNamespace, testLabelSelector)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ch, err := p.Start(ctx)
	require.NoError(t, err)
	<-ch
	require.Empty(t, p.resourceVersion)
}

func TestPeerFinderImplStart_WatchDoesNotReturnsError_ResourceVersionGetsUpdated(t *testing.T) {
	clientset, podMock := buildClientSetOnPodListMocks(t, []v1.Pod{}, nil)
	addWatchMocksSuccess(t, podMock)
	p, err := NewPeerFinder(clientset, testNamespace, testLabelSelector)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ch, err := p.Start(ctx)
	require.NoError(t, err)

	// Since the context was cancelled the returned PeerURL channel will be
	// closed.
	_, ok := <-ch
	require.False(t, ok)
	require.Equal(t, testResourceVersionNext, p.resourceVersion)
}

func TestPeerFinderImplStart_WatchDoesNotReturnError_NewPeerURLsSentOnChannel(t *testing.T) {
	clientset, podMock := buildClientSetOnPodListMocks(t, []v1.Pod{}, nil)
	p, err := NewPeerFinder(clientset, testNamespace, testLabelSelector)
	require.NoError(t, err)

	// Create cancellable context so we can stop the Go routine.
	ctx, cancel := context.WithCancel(context.Background())

	// Create a mock for Watch that can be called repeatedly.
	watch := watchmocks.NewInterface(t)
	podMock.On("Watch", testutils.AnyContext, listOptionsNext).Run(func(_ mock.Arguments) {
		// Prepare the next call to PodList which returns a different set of
		// items than our first call to buildClientSetOnPodListMocks().
		items := []v1.Pod{
			{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					PodIP: testIPAddress2,
				},
			},
		}
		clientset, podMock := buildClientSetOnPodListMocks(t, items, nil)
		p.clientset = clientset
		addWatchMocksSuccess(t, podMock)
	}).Return(watch, nil)
	watch.On("Stop")

	_, ch, err := p.Start(ctx)
	require.NoError(t, err)

	// Wait for the new PeerURLs to be sent down the channel.
	peers := <-ch
	require.Equal(t, testIPAddress2, peers[0])

	// Cancel the context so the Go routine in Start() exits.
	cancel()

	// Wait for the channel to be closed.
	for {
		_, ok := <-ch
		if !ok {
			return
		}
	}
	// Test will hang forever if 'ch' is never closed.
}
