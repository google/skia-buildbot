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
	"go.skia.org/infra/go/exec"
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

	gsutilPackage = &swarming_api.SwarmingRpcsCipdPackage{
		PackageName: "infra/gsutil",
		Path:        "cipd_bin_packages",
		Version:     "version:4.28",
	}
)

func SwarmingInit(serviceAccountFile string) error {
	// Public Isolate client.
	var err error
	isolateClientPublic, err = isolate.NewClientWithServiceAccount(*workdir, isolate.ISOLATE_SERVER_URL, serviceAccountFile)
	if err != nil {
		return fmt.Errorf("Failed to create public isolate client: %s", err)
	}
	// Private Isolate client.
	isolateClientPrivate, err = isolate.NewClientWithServiceAccount(*workdir, isolate.ISOLATE_SERVER_URL_PRIVATE, serviceAccountFile)
	if err != nil {
		return fmt.Errorf("Failed to create private isolate client: %s", err)
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(*local, swarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	httpClient := auth.ClientFromTokenSource(ts)

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

type IsolateDetails struct {
	Command     []string `json:"command"`
	RelativeCwd string   `json:"relative_cwd"`
	IsolateDep  string
	CipdInput   *swarming_api.SwarmingRpcsCipdInput
}

func GetIsolateDetails(ctx context.Context, properties *swarming_api.SwarmingRpcsTaskProperties) (*IsolateDetails, error) {
	details := &IsolateDetails{}
	inputsRef := properties.InputsRef

	f, err := ioutil.TempFile(*workdir, inputsRef.Isolated+"_")
	if err != nil {
		return details, fmt.Errorf("Could not create tmp file in %s: %s", *workdir, err)
	}
	defer util.Remove(f.Name())
	cmd := []string{
		isolateServerPath, "download",
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
		BaseDir:     path.Join(*resourcesDir, "isolates"),
		Blacklist:   []string{},
		IsolateFile: path.Join(*resourcesDir, "isolates", "leasing.isolate"),
	}
	if isolateDep != "" {
		isolateTask.Deps = []string{isolateDep}
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, err := isolateClient.IsolateTasks(ctx, isolateTasks)
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

func TriggerSwarmingTask(pool, requester, datastoreId, osType, deviceType, arch, botId, serverURLstring, isolateHash string, isolateDetails *IsolateDetails, setupDebugger bool) (string, error) {
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

	// Arguments that will be passed to leasing.py
	extraArgs := []string{
		"--task-id", datastoreId,
		"--os-type", osType,
		"--leasing-server", serverURL,
		"--debug-command", strings.Join(isolateDetails.Command, " "),
		"--command-relative-dir", isolateDetails.RelativeCwd,
	}
	if setupDebugger {
		skiaserveGSPath, err := GetSkiaServeGSPath(arch)
		if err != nil {
			return "", fmt.Errorf("Could not find skiaserve for %s: %s", arch, err)
		}
		extraArgs = append(extraArgs, "--skiaserve-gs-path", skiaserveGSPath)

		// Add GsUtil CIPD package to isolate input. It will be used to download
		// the skiaserve binary from Google Storage.
		if isolateDetails.CipdInput == nil {
			isolateDetails.CipdInput = &swarming_api.SwarmingRpcsCipdInput{}
		}
		if isolateDetails.CipdInput.Packages == nil {
			isolateDetails.CipdInput.Packages = []*swarming_api.SwarmingRpcsCipdPackage{gsutilPackage}
		} else {
			isolateDetails.CipdInput.Packages = append(isolateDetails.CipdInput.Packages, gsutilPackage)
		}
	}
	// Construct the command.
	command := []string{"python", "leasing.py"}
	// All all extra arguments to the command.
	command = append(command, extraArgs...)

	isolateServer := GetSwarmingInstance(pool).IsolateServer
	expirationSecs := int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	executionTimeoutSecs := int64(SWARMING_HARD_TIMEOUT.Seconds())
	ioTimeoutSecs := int64(SWARMING_HARD_TIMEOUT.Seconds())
	taskName := fmt.Sprintf("Leased by %s using leasing.skia.org", requester)
	taskRequest := &swarming_api.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: expirationSecs,
		Name:           taskName,
		Priority:       LEASE_TASK_PRIORITY,
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
