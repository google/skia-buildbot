/*
	Used by the Leasing Server to interact with swarming.
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
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
		"Skia":             PublicSwarming,
		"SkiaCT":           PublicSwarming,
		"SkiaInternal":     InternalSwarming,
		"CT":               InternalSwarming,
		"CTAndroidBuilder": InternalSwarming,
		"CTLinuxBuilder":   InternalSwarming,
	}

	isolateServerPath string

	cpythonPackage = &swarming_api.SwarmingRpcsCipdPackage{
		PackageName: "infra/python/cpython/${platform}",
		Path:        "python",
		Version:     "version:2.7.14.chromium14",
	}
)

func SwarmingInit(serviceAccountFile string) error {
	// Public Isolate client.
	var err error
	isolateClientPublic, err = isolate.NewLegacyClientWithServiceAccount(*workdir, isolate.ISOLATE_SERVER_URL, serviceAccountFile)
	if err != nil {
		return fmt.Errorf("Failed to create public isolate client: %s", err)
	}
	// Private Isolate client.
	isolateClientPrivate, err = isolate.NewLegacyClientWithServiceAccount(*workdir, isolate.ISOLATE_SERVER_URL_PRIVATE, serviceAccountFile)
	if err != nil {
		return fmt.Errorf("Failed to create private isolate client: %s", err)
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, swarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

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

	// Set path to the isolateserver.py script.
	isolateServerPath = path.Join(*workdir, "client-py", "isolateserver.py")

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
	OsTypes         map[string]int
	OsToDeviceTypes map[string]map[string]int
}

func getPoolDetails(pool string) (*PoolDetails, error) {
	swarmingClient := *GetSwarmingClient(pool)
	bots, err := swarmingClient.ListBotsForPool(pool)
	if err != nil {
		return nil, fmt.Errorf("Could not list bots in pool: %s", err)
	}
	osTypes := map[string]int{}
	osToDeviceTypes := map[string]map[string]int{}
	for _, bot := range bots {
		if bot.IsDead || bot.Quarantined {
			// Do not include dead/quarantined bots in the counts below.
			continue
		}
		osType := ""
		deviceType := ""
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
				osType = val
			}
			if d.Key == "device_type" {
				// There should only be one value for device type.
				deviceType = d.Value[0]
			}
		}
		osTypes[osType]++
		if _, ok := osToDeviceTypes[osType]; !ok {
			osToDeviceTypes[osType] = map[string]int{}
		}
		if deviceType != "" {
			osToDeviceTypes[osType][deviceType]++
		}
	}
	return &PoolDetails{
		OsTypes:         osTypes,
		OsToDeviceTypes: osToDeviceTypes,
	}, nil
}

func GetDetailsOfAllPools() (map[string]*PoolDetails, error) {
	poolToDetails := map[string]*PoolDetails{}
	for pool := range PoolsToSwarmingInstance {
		details, err := getPoolDetails(pool)
		if err != nil {
			return nil, err
		}
		poolToDetails[pool] = details
	}
	return poolToDetails, nil
}

type IsolateDetails struct {
	Command     []string `json:"command"`
	RelativeCwd string   `json:"relative_cwd"`
	IsolateDep  string
	CipdInput   *swarming_api.SwarmingRpcsCipdInput
}

func GetIsolateDetails(ctx context.Context, serviceAccountFile string, properties *swarming_api.SwarmingRpcsTaskProperties) (*IsolateDetails, error) {
	details := &IsolateDetails{}
	inputsRef := properties.InputsRef

	f, err := ioutil.TempFile(*workdir, inputsRef.Isolated+"_")
	if err != nil {
		return details, fmt.Errorf("Could not create tmp file in %s: %s", *workdir, err)
	}
	defer util.Remove(f.Name())
	cmd := []string{
		isolateServerPath, "download",
		"--auth-service-account-json", serviceAccountFile,
		"-I", inputsRef.Isolatedserver,
		"--namespace", inputsRef.Namespace,
		"-f", inputsRef.Isolated, path.Base(f.Name()),
		"-t", *workdir,
	}
	output, err := exec.RunCwd(ctx, *workdir, cmd...)
	if err != nil {
		return details, fmt.Errorf("Failed to run cmd %s: %s", cmd, err)
	}

	if err := json.NewDecoder(f).Decode(&details); err != nil {
		return details, fmt.Errorf("Could not decode %s: %s", output, err)
	}
	details.IsolateDep = inputsRef.Isolated
	details.CipdInput = properties.CipdInput
	if len(details.Command) == 0 {
		details.Command = append(details.Command, properties.Command...)
	}
	// Append extra arguments to the command.
	details.Command = append(details.Command, properties.ExtraArgs...)

	return details, nil
}

func GetIsolateHash(ctx context.Context, pool, isolateDep string) (string, error) {
	isolateClient := *GetIsolateClient(pool)
	isolateTask := &isolate.Task{
		BaseDir:     path.Join(*isolatesDir),
		IsolateFile: path.Join(*isolatesDir, "leasing.isolate"),
	}
	if isolateDep != "" {
		isolateTask.Deps = []string{isolateDep}
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, _, err := isolateClient.IsolateTasks(ctx, isolateTasks)
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

func GetSwarmingTaskMetadata(pool, taskId string) (*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	swarmingClient := *GetSwarmingClient(pool)
	return swarmingClient.GetTaskMetadata(taskId)
}

func IsBotIdValid(pool, botId string) (bool, error) {
	swarmingClient := *GetSwarmingClient(pool)
	dims := map[string]string{
		"pool": pool,
		"id":   botId,
	}
	bots, err := swarmingClient.ListBots(dims)
	if err != nil {
		return false, fmt.Errorf("Could not query swarming bots with %s: %s", dims, err)
	}
	if len(bots) > 1 {
		return false, fmt.Errorf("Something went wrong, more than 1 bot was returned with %s: %s", dims, err)
	}
	if len(bots) == 0 {
		// There were no matches for the pool + botId combination.
		return false, nil
	}
	if bots[0].BotId == botId {
		return true, nil
	} else {
		return false, fmt.Errorf("%s returned %s instead of the expected %s", dims, bots[1].BotId, botId)
	}
}

func TriggerSwarmingTask(pool, requester, datastoreId, osType, deviceType, botId, serverURL, isolateHash string, isolateDetails *IsolateDetails) (string, error) {
	dimsMap := map[string]string{
		"pool": pool,
	}
	if osType != "" {
		dimsMap["os"] = osType
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

	// Always isolate cpython for Windows. See skbug.com/9501 for context and for
	// why we do not isolate it for all architectures.
	pythonBinary := "python"
	if strings.HasPrefix(osType, "Windows") {
		if isolateDetails.CipdInput == nil {
			isolateDetails.CipdInput = &swarming_api.SwarmingRpcsCipdInput{}
		}
		if isolateDetails.CipdInput.Packages == nil {
			isolateDetails.CipdInput.Packages = []*swarming_api.SwarmingRpcsCipdPackage{cpythonPackage}
		} else {
			isolateDetails.CipdInput.Packages = append(isolateDetails.CipdInput.Packages, cpythonPackage)
		}
		pythonBinary = "python/bin/python"
	}

	// Arguments that will be passed to leasing.py
	extraArgs := []string{
		"--task-id", datastoreId,
		"--os-type", osType,
		"--leasing-server", serverURL,
		"--debug-command", strings.Join(isolateDetails.Command, " "),
		"--command-relative-dir", isolateDetails.RelativeCwd,
	}

	// Construct the command.
	command := []string{pythonBinary, "leasing.py"}
	command = append(command, extraArgs...)

	isolateServer := GetSwarmingInstance(pool).IsolateServer
	expirationSecs := int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	executionTimeoutSecs := int64(swarmingHardTimeout.Seconds())
	ioTimeoutSecs := int64(swarmingHardTimeout.Seconds())
	taskName := fmt.Sprintf("Leased by %s using leasing.skia.org", requester)
	taskRequest := &swarming_api.SwarmingRpcsNewTaskRequest{
		Name:     taskName,
		Priority: leaseTaskPriority,
		TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
			{
				ExpirationSecs: expirationSecs,
				Properties: &swarming_api.SwarmingRpcsTaskProperties{
					CipdInput:            isolateDetails.CipdInput,
					Dimensions:           dims,
					ExecutionTimeoutSecs: executionTimeoutSecs,
					Command:              command,
					InputsRef: &swarming_api.SwarmingRpcsFilesRef{
						Isolated:       isolateHash,
						Isolatedserver: isolateServer,
						Namespace:      isolate.DEFAULT_NAMESPACE,
					},
					IoTimeoutSecs: ioTimeoutSecs,
				},
			},
		},
		User: "skiabot@google.com",
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

func GetSwarmingBotLink(server, botId string) string {
	return fmt.Sprintf("https://%s/bot?id=%s", server, botId)
}
