package autoscaler

import (
	"fmt"
	"strings"
	"time"

	assert "github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	compute "google.golang.org/api/compute/v1"
)

const (
	// Mock bot statuses.
	online         = "online"
	offlineDeleted = "deleted"
	offlineDead    = "dead"
	offlineError   = "error"

	testProject        = "fake-project"
	testZone           = "fake-zone"
	testSwarmingServer = "fake-swarming"
	testAutoscalerName = "fake-autoscaler"
)

var (
	baseInstance = getInstance(0)
	dims         = util.NewStringSet([]string{
		fmt.Sprintf("os:%s", baseInstance.Os),
		fmt.Sprintf("machine_type:%s", baseInstance.MachineType),
	})
)

func getInstance(num int) *gce.Instance {
	return instance_types.LinuxSmall(num, "fake_setup_script.sh")
}

// Setup for autoscaler tests. Returns an Autoscaler instance, a URLMock
// instance and a slice of 100 GCE instances.
func Setup(t sktest.TestingT) (*Autoscaler, *mockhttpclient.URLMock, []*gce.Instance) {
	urlMock := mockhttpclient.NewURLMock()
	instances := autoscaler.GetInstanceRange(1, 100, getInstance)
	MockAllOnline(t, urlMock, instances)
	as, err := newAutoscalerWithClient(testProject, testZone, testSwarmingServer, testAutoscalerName, urlMock.Client(), instances)
	assert.NoError(t, err)
	assert.True(t, urlMock.Empty())
	return as, urlMock, instances
}

// Mock an API call to retrieve a GCE instance's status.
func mockGCEInstanceStatus(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string, status string) {
	rv := &compute.Instance{
		Name: name,
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						NatIP: "192.168.1.1",
					},
				},
			},
		},
		Status: status,
	}
	js := testutils.MarshalJSON(t, rv)
	url := fmt.Sprintf("https://compute.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s?alt=json&prettyPrint=false", testProject, testZone, name)
	urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(js)))
}

// FakeSwarmingStatus represents a Swarming bot status. It is used by the below
// helper functions to mock API results from Swarming.
type FakeSwarmingStatus struct {
	Online bool
	Busy   bool
}

// Mock results for one bots.list call.
func MockSwarmingStatuses(t sktest.TestingT, urlMock *mockhttpclient.URLMock, statuses map[string]*FakeSwarmingStatus) {
	dimensions := make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(dims))
	for dim, _ := range dims {
		split := strings.Split(dim, ":")
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringListPair{
			Key:   split[0],
			Value: []string{split[1]},
		})
	}
	items := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(statuses))
	for name, status := range statuses {
		bot := &swarming_api.SwarmingRpcsBotInfo{
			BotId:      name,
			Dimensions: dimensions,
		}
		// TODO(borenet): Presumably long-dead or deleted bots, or those
		// which have never connected to the Swarming server will simply
		// not appear in the search results.
		if !status.Online {
			bot.IsDead = true
		}
		if status.Busy {
			bot.TaskId = fmt.Sprintf("task-%s", name)
		}
		items = append(items, bot)
	}
	rv := &swarming_api.SwarmingRpcsBotList{
		Items: items,
	}
	js := testutils.MarshalJSON(t, rv)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bots/list?alt=json&dimensions=%s%%3A%s&prettyPrint=false", testSwarmingServer, AUTOSCALER_DIMENSION, testAutoscalerName), mockhttpclient.MockGetDialogue([]byte(js)))
}

// Mock API calls indicating that a bot is online and idle. The returned
// FakeSwarmingStatus is intended to be passed to MockSwarmingStatuses.
func MockOnline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) *FakeSwarmingStatus {
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING")
	return &FakeSwarmingStatus{
		Online: true,
		Busy:   false,
	}
}

// Mock API calls indicating that a bot is online and idle. The returned
// FakeSwarmingStatus is intended to be passed to MockSwarmingStatuses.
func MockOnlineAndBusy(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) *FakeSwarmingStatus {
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING")
	return &FakeSwarmingStatus{
		Online: true,
		Busy:   true,
	}
}

// Mock API calls indicating that a bot is offline. The returned
// FakeSwarmingStatus is intended to be passed to MockSwarmingStatuses.
func MockOffline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) *FakeSwarmingStatus {
	mockGCEInstanceStatus(t, urlMock, name, "TERMINATED")
	return &FakeSwarmingStatus{
		Online: false,
		Busy:   false,
	}
}

// Mock API calls indicating that the given GCE instances (but not Swarming
// bots) are online.
func MockAllGCEInstancesOnline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, instances []*gce.Instance) map[string]*FakeSwarmingStatus {
	status := make(map[string]*FakeSwarmingStatus, len(instances))
	for _, instance := range instances {
		status[instance.Name] = MockOnline(t, urlMock, instance.Name)
	}
	return status
}

// Mock API calls indicating that the given bots are online.
func MockAllOnline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, instances []*gce.Instance) {
	status := MockAllGCEInstancesOnline(t, urlMock, instances)
	MockSwarmingStatuses(t, urlMock, status)
}

// Mock API calls indicating that the given bots are offline.
func MockAllOffline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, instances []*gce.Instance) {
	status := make(map[string]*FakeSwarmingStatus, len(instances))
	for _, instance := range instances {
		status[instance.Name] = MockOffline(t, urlMock, instance.Name)
	}
	MockSwarmingStatuses(t, urlMock, status)
}

// Mock the API calls to stop an instance.
func MockStop(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	taskId := fmt.Sprintf("terminate-%s", name)
	terminate := &swarming_api.SwarmingRpcsTerminateResponse{
		TaskId: taskId,
	}
	js := testutils.MarshalJSON(t, terminate)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/terminate?alt=json&prettyPrint=false", testSwarmingServer, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	task := &swarming_api.SwarmingRpcsTaskResult{
		State: "COMPLETED",
	}
	js = testutils.MarshalJSON(t, task)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/task/%s/result?alt=json&prettyPrint=false", testSwarmingServer, taskId), mockhttpclient.MockGetDialogue([]byte(js)))
	deleted := &swarming_api.SwarmingRpcsDeletedResponse{
		Deleted: true,
	}
	js = testutils.MarshalJSON(t, deleted)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/delete?alt=json&prettyPrint=false", testSwarmingServer, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	rv := &compute.Operation{
		Name:   fmt.Sprintf("op-stop-%s", name),
		Status: gce.OPERATION_STATUS_DONE,
		Zone:   testZone,
	}
	js = testutils.MarshalJSON(t, rv)
	urlMock.MockOnce(fmt.Sprintf("https://compute.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s/stop?alt=json&prettyPrint=false", testProject, testZone, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	urlMock.MockOnce(fmt.Sprintf("https://compute.googleapis.com/compute/beta/projects/%s/zones/%s/operations/%s?alt=json&prettyPrint=false", testProject, testZone, rv.Name), mockhttpclient.MockGetDialogue([]byte(js)))
	mockGCEInstanceStatus(t, urlMock, name, "TERMINATED")
}

// Mock the API calls to start an instance.
func MockStart(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	rv := &compute.Operation{
		Name:   fmt.Sprintf("op-start-%s", name),
		Status: gce.OPERATION_STATUS_DONE,
		Zone:   testZone,
	}
	js := testutils.MarshalJSON(t, rv)
	urlMock.MockOnce(fmt.Sprintf("https://compute.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s/start?alt=json&prettyPrint=false", testProject, testZone, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	urlMock.MockOnce(fmt.Sprintf("https://compute.googleapis.com/compute/beta/projects/%s/zones/%s/operations/%s?alt=json&prettyPrint=false", testProject, testZone, rv.Name), mockhttpclient.MockGetDialogue([]byte(js)))
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING")
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING") // for GetIpAddress
}

// Wait for the bot shutdown goroutine to finish. Note that this will never
// finish if there are extraneous mocks in the URLMock!
func WaitForStop(urlMock *mockhttpclient.URLMock) {
	for {
		if urlMock.Empty() {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
