package autoscaler

import (
	"fmt"
	"net/http"
	"time"

	assert "github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/swarming"
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
	as, err := NewAutoscaler(testProject, testZone, testSwarmingServer, urlMock.Client(), instances)
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
	urlMock.MockOnce(fmt.Sprintf("https://www.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s?alt=json", testProject, testZone, name), mockhttpclient.MockGetDialogue([]byte(js)))
}

// Mock an API call to retrieve a Swarming bot's status.
func mockSwarmingStatus(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string, busy bool, status string) {
	// We need to test for correct behavior for various response
	// states from Swarming:
	// - Deleted: The bot has connected before but was disconnected
	//       and deleted correctly.
	// - Error: Assume the bot has never connected to Swarming and
	//       is offline. This is not an error.
	// - Dead: This shouldn't occur, but it's possible that the bot
	//       was requested to be shut down but we didn't
	//       successfully finish the process, so it wasn't deleted
	//       in Swarming.
	dimsMap, err := swarming.ParseDimensions(dims.Keys())
	assert.NoError(t, err)
	rv := &swarming_api.SwarmingRpcsBotInfo{
		BotId:      name,
		Dimensions: swarming.StringMapToBotDimensions(dimsMap),
	}
	if busy {
		rv.TaskId = fmt.Sprintf("task-%s", name)
	}
	if status == offlineDeleted {
		rv.Deleted = true
	} else if status == offlineDead {
		rv.IsDead = true
	} else if status == offlineError {
		urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/get?alt=json", testSwarmingServer, name), mockhttpclient.MockGetError(fmt.Sprintf("%s not found", name), http.StatusNotFound))
		return
	}
	js := testutils.MarshalJSON(t, rv)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/get?alt=json", testSwarmingServer, name), mockhttpclient.MockGetDialogue([]byte(js)))
}

// Mock API calls indicating that a bot is online.
func MockOnline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING")
	mockSwarmingStatus(t, urlMock, name, false, online)
}

func MockOnlineAndBusy(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	mockGCEInstanceStatus(t, urlMock, name, "RUNNING")
	mockSwarmingStatus(t, urlMock, name, true, online)
}

// Mock API calls indicating that a bot is offline.
func MockOffline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	mockGCEInstanceStatus(t, urlMock, name, "TERMINATED")
	mockSwarmingStatus(t, urlMock, name, false, offlineDeleted)
}

// Mock API calls indicating that the given bots are online.
func MockAllOnline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, instances []*gce.Instance) {
	for _, instance := range instances {
		MockOnline(t, urlMock, instance.Name)
	}
}

// Mock API calls indicating that the given bots are offline.
func MockAllOffline(t sktest.TestingT, urlMock *mockhttpclient.URLMock, instances []*gce.Instance) {
	for _, instance := range instances {
		MockOffline(t, urlMock, instance.Name)
	}
}

// Mock the API calls to stop an instance.
func MockStop(t sktest.TestingT, urlMock *mockhttpclient.URLMock, name string) {
	taskId := fmt.Sprintf("terminate-%s", name)
	terminate := &swarming_api.SwarmingRpcsTerminateResponse{
		TaskId: taskId,
	}
	js := testutils.MarshalJSON(t, terminate)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/terminate?alt=json", testSwarmingServer, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	task := &swarming_api.SwarmingRpcsTaskResult{
		State: "COMPLETED",
	}
	js = testutils.MarshalJSON(t, task)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/task/%s/result?alt=json", testSwarmingServer, taskId), mockhttpclient.MockGetDialogue([]byte(js)))
	deleted := &swarming_api.SwarmingRpcsDeletedResponse{
		Deleted: true,
	}
	js = testutils.MarshalJSON(t, deleted)
	urlMock.MockOnce(fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/delete?alt=json", testSwarmingServer, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	rv := &compute.Operation{
		Name:   fmt.Sprintf("op-stop-%s", name),
		Status: gce.OPERATION_STATUS_DONE,
		Zone:   testZone,
	}
	js = testutils.MarshalJSON(t, rv)
	urlMock.MockOnce(fmt.Sprintf("https://www.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s/stop?alt=json", testProject, testZone, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	urlMock.MockOnce(fmt.Sprintf("https://www.googleapis.com/compute/beta/projects/%s/zones/%s/operations/%s?alt=json", testProject, testZone, rv.Name), mockhttpclient.MockGetDialogue([]byte(js)))
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
	urlMock.MockOnce(fmt.Sprintf("https://www.googleapis.com/compute/beta/projects/%s/zones/%s/instances/%s/start?alt=json", testProject, testZone, name), mockhttpclient.MockPostDialogue("", nil, []byte(js)))
	urlMock.MockOnce(fmt.Sprintf("https://www.googleapis.com/compute/beta/projects/%s/zones/%s/operations/%s?alt=json", testProject, testZone, rv.Name), mockhttpclient.MockGetDialogue([]byte(js)))
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
