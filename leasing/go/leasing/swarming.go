/*
	Used by the Leasing Server to interact with swarming.
*/
package main

import (
	"fmt"
	"path"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/isolate"

	"go.skia.org/infra/go/swarming"
)

type SwarmingInstanceClients struct {
	SwarmingServer string
	SwarmingClient *swarming.ApiClient

	IsolateServer string
	IsolateClient **isolate.Client
}

var (
	isolateClientPublic  *isolate.Client
	isolateClientPrivate *isolate.Client

	swarmingClientPublic  swarming.ApiClient
	swarmingClientPrivate swarming.ApiClient

	PublicSwarming *SwarmingInstanceClients = &SwarmingInstanceClients{
		SwarmingServer: swarming.SWARMING_SERVER,
		IsolateServer:  isolate.ISOLATE_SERVER_URL,
		SwarmingClient: &swarmingClientPublic,
		IsolateClient:  &isolateClientPublic,
	}

	InternalSwarming *SwarmingInstanceClients = &SwarmingInstanceClients{
		SwarmingServer: swarming.SWARMING_SERVER_PRIVATE,
		IsolateServer:  isolate.ISOLATE_SERVER_URL_PRIVATE,
		SwarmingClient: &swarmingClientPrivate,
		IsolateClient:  &isolateClientPrivate,
	}

	PoolsToSwarmingInstance = map[string]*SwarmingInstanceClients{
		"Skia":         PublicSwarming,
		"SkiaCT":       PublicSwarming,
		"SkiaInternal": InternalSwarming,
		"CT":           InternalSwarming,
	}
)

func SwarmingInit() error {
	// Public Isolate client.
	var err error
	isolateClientPublic, err = isolate.NewClient(*workdir, isolate.ISOLATE_SERVER_URL)
	if err != nil {
		return fmt.Errorf("Failed to create public isolate client: %s", err)
	}
	// Private Isolate client.
	isolateClientPrivate, err = isolate.NewClient(*workdir, isolate.ISOLATE_SERVER_URL_PRIVATE)
	if err != nil {
		return fmt.Errorf("Failed to create private isolate client: %s", err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Failed to create authenticated HTTP client: %s", err)
	}
	// Public Swarming API client.
	swarmingClientPublic, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		return fmt.Errorf("Failed to create public swarming client: %s", err)
	}
	// Private Swarming API client.
	swarmingClientPrivate, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		return fmt.Errorf("Failed to create private swarming client: %s", err)
	}

	return nil
}

func GetSwarmingInstance(pool string) *SwarmingInstanceClients {
	return PoolsToSwarmingInstance[pool]
}

func GetSwarmingClient(pool string) *swarming.ApiClient {
	return GetSwarmingInstance(pool).SwarmingClient
}

func GetIsolateClient(pool string) **isolate.Client {
	return GetSwarmingInstance(pool).IsolateClient
}

type PoolDetails struct {
	OsTypes     map[string]int
	DeviceTypes map[string]int
}

func GetPoolDetails(pool string) (*PoolDetails, error) {
	swarmingClient := *GetSwarmingClient(pool)
	bots, err := swarmingClient.ListBotsForPool(pool)
	if err != nil {
		return nil, fmt.Errorf("Could not list bots in pool: %s", err)
	}
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
				// does and it works in all cases we have (atleast as of 11/1/17).
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
	hashes, err := isolateClientPublic.IsolateTasks(isolateTasks)
	if err != nil {
		return "", fmt.Errorf("Could not isolate leasing task: %s", err)
	}
	if len(hashes) != 1 {
		return "", fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}
	return hashes[0], nil
}

func GetSwarmingTask(pool, taskId string) (*swarming_api.SwarmingRpcsTaskResult, error) {
	swarmingClient := *GetSwarmingClient(pool)
	return swarmingClient.GetTask(taskId, false)
}

func TriggerSwarmingTask(pool, requester, datastoreId, osType, deviceType, botId, serverURL string) (string, error) {
	isolateHash, err := getIsolateHash(osType)
	if err != nil {
		return "", fmt.Errorf("Could not get isolate hash: %s", err)
	}

	dimsMap := map[string]string{
		"pool": pool,
		"os":   osType,
	}
	if deviceType != "" {
		dimsMap["device_type"] = deviceType
	}
	if botId != "" {
		dimsMap["id"] = botId
	}
	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dimsMap))
	for k, v := range dimsMap {
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
	}

	// Arguments that will be passed to leasing.py
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

	swarmingClient := *GetSwarmingClient(pool)
	resp, err := swarmingClient.TriggerTask(taskRequest)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s", err)
	}
	return resp.TaskId, nil
}

func GetSwarmingTaskLink(server, taskId string) string {
	return fmt.Sprintf("https://%s/task?id=%s", server, taskId)
}
