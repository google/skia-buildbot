/*
	Used by the Leasing Server to interact with swarming.
*/
package main

import (
	"fmt"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"path"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/isolate"

	"go.skia.org/infra/go/swarming"
)

var (
	isolateClient  *isolate.Client
	swarmingClient swarming.ApiClient
)

func SwarmingInit() error {
	// Isolate client.
	var err error
	isolateClient, err = isolate.NewClient(*workdir, isolate.ISOLATE_SERVER_URL)
	if err != nil {
		return fmt.Errorf("Failed to create isolate client: %s", err)
	}
	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Failed to create authenticated HTTP client: %s", err)
	}
	// Swarming API client.
	swarmingClient, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		return fmt.Errorf("Failed to create swarming client: %s", err)
	}
	return nil
}

type PoolDetails struct {
	OsTypes     map[string]int
	DeviceTypes map[string]int
}

func GetPoolDetails(pool string) (*PoolDetails, error) {
	bots, err := swarmingClient.ListBotsForPool(pool)
	if err != nil {
		return nil, fmt.Errorf("Could not list bots in pool: %s", err)
	}
	fmt.Println("BOTS IN SKIA POOL:")
	osTypes := map[string]int{}
	deviceTypes := map[string]int{}
	for _, bot := range bots {
		if bot.IsDead || bot.Quarantined {
			// Do not include dead/quarantined bots in the counts below.
			continue
		}
		for _, d := range bot.Dimensions {
			if d.Key == "os" {
				val := ""
				// Use the longest string from the os values because that is what the swarming UI
				// does and it works in all cases we have (atleast as of 10/20/17).
				for _, v := range d.Value {
					if len(v) > len(val) {
						val = v
					}
				}
				osTypes[val]++
			}
			if d.Key == "device_type" {
				// There should only be one value for device type.
				val := d.Value[0]
				deviceTypes[val]++
			}
		}
	}
	return &PoolDetails{
		OsTypes:     osTypes,
		DeviceTypes: deviceTypes,
	}, nil
}

func getIsolateHash(osType string) (string, error) {
	isolateTask := &isolate.Task{
		BaseDir:     "isolates",
		Blacklist:   []string{},
		Deps:        []string{},
		IsolateFile: path.Join("isolates", "leasing.isolate"),
		OsType:      osType,
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, err := isolateClient.IsolateTasks(isolateTasks)
	if err != nil {
		return "", fmt.Errorf("Could not isolate leasing task: %s")
	}
	if len(hashes) != 1 {
		return "", fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}
	return hashes[0], nil
}

func GetSwarmingTask(taskId string) (*swarming_api.SwarmingRpcsTaskResult, error) {
	return swarmingClient.GetTask(taskId, false)
}

func TriggerSwarmingTask(requester, datastoreId, osType, serverURL string) (string, error) {
	isolateHash, err := getIsolateHash(osType)
	if err != nil {
		return "", fmt.Errorf("Could not get isolate hash: %s", err)
	}

	rawDims := []string{"pool:Skia", "id:skia-gce-001"}

	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(rawDims))
	dimsMap := make(map[string]string, len(rawDims))
	for _, d := range rawDims {
		split := strings.SplitN(d, ":", 2)
		key := split[0]
		val := split[1]
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   key,
			Value: val,
		})
		dimsMap[key] = val
	}

	extraArgs := []string{
		"--task-id", datastoreId,
		"--os-type", osType,
		"--leasing-server", serverURL,
	}
	expirationSecs := int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	executionTimeoutSecs := int64(SWARMING_HARD_TIMEOUT.Seconds())
	ioTimeoutSecs := int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	taskName := fmt.Sprintf("Leased by %s using leasing.skia.org", requester)
	taskRequest := &swarming_api.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: expirationSecs,
		Name:           taskName,
		Priority:       LEASE_TASK_PRIORITY,
		Properties: &swarming_api.SwarmingRpcsTaskProperties{
			Dimensions:           dims,
			ExecutionTimeoutSecs: executionTimeoutSecs,
			ExtraArgs:            extraArgs,
			InputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated:       isolateHash,
				Isolatedserver: isolate.ISOLATE_SERVER_URL,
				Namespace:      isolate.DEFAULT_NAMESPACE,
			},
			IoTimeoutSecs: ioTimeoutSecs,
		},
		ServiceAccount: swarming.GetServiceAccountFromTaskDims(dimsMap),
		User:           "skiabot@google.com",
	}

	resp, err := swarmingClient.TriggerTask(taskRequest)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s", err)
	}
	return resp.TaskId, nil
}
