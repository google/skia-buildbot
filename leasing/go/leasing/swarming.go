/*
	Used by the Leasing Server to interact with swarming.
*/
package main

import (
	"context"
	"fmt"
	"strings"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/leasing/go/types"
)

// SwarmingInstanceClients contains all of the API clients needed to interact
// with a given Swarming instance.
type SwarmingInstanceClients struct {
	SwarmingServer string
	SwarmingClient *swarming.ApiClient

	CasClient   *cas.CAS
	CasInstance string
}

var (
	casClientPublic  cas.CAS
	casClientPrivate cas.CAS

	swarmingClientPublic  swarming.ApiClient
	swarmingClientPrivate swarming.ApiClient

	// PublicSwarming contains the API clients needed for the public Swarming
	// instance.
	PublicSwarming *SwarmingInstanceClients = &SwarmingInstanceClients{
		SwarmingServer: swarming.SWARMING_SERVER,
		SwarmingClient: &swarmingClientPublic,
		CasClient:      &casClientPublic,
		CasInstance:    rbe.InstanceChromiumSwarm,
	}

	// InternalSwarming contains the API clients needed for the internal
	// Swarming instance.
	InternalSwarming *SwarmingInstanceClients = &SwarmingInstanceClients{
		SwarmingServer: swarming.SWARMING_SERVER_PRIVATE,
		SwarmingClient: &swarmingClientPrivate,
		CasClient:      &casClientPrivate,
		CasInstance:    rbe.InstanceChromeSwarming,
	}

	// PoolsToSwarmingInstance maps Swarming pool names to Swarming instances.
	PoolsToSwarmingInstance = map[string]*SwarmingInstanceClients{
		"Skia":             PublicSwarming,
		"SkiaCT":           PublicSwarming,
		"SkiaInternal":     InternalSwarming,
		"CT":               InternalSwarming,
		"CTAndroidBuilder": InternalSwarming,
		"CTLinuxBuilder":   InternalSwarming,
	}

	cpythonPackage = &swarming_api.SwarmingRpcsCipdPackage{
		PackageName: "infra/3pp/tools/cpython3/${platform}",
		Path:        "python",
		Version:     "version:2@3.8.10.chromium.19",
	}
)

// SwarmingInit initializes Swarming globally.
func SwarmingInit(ctx context.Context, serviceAccountFile string) error {
	ts, err := google.DefaultTokenSource(ctx, swarming.AUTH_SCOPE, compute.CloudPlatformScope)
	if err != nil {
		return skerr.Wrapf(err, "Problem setting up default token source")
	}

	// Public CAS client.
	casClientPublic, err = rbe.NewClient(context.TODO(), rbe.InstanceChromiumSwarm, ts)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create RBE client")
	}

	// Private CAS client.
	casClientPrivate, err = rbe.NewClient(context.TODO(), rbe.InstanceChromeSwarming, ts)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create RBE client")
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Public Swarming API client.
	swarmingClientPublic, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create public swarming client")
	}
	// Private Swarming API client.
	swarmingClientPrivate, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create private swarming client")
	}

	return nil
}

// GetSwarmingInstance returns the Swarming instance for the given Swarming
// pool.
func GetSwarmingInstance(pool string) *SwarmingInstanceClients {
	return PoolsToSwarmingInstance[pool]
}

// GetSwarmingClient returns the Swarming client for the given Swarming pool.
func GetSwarmingClient(pool string) *swarming.ApiClient {
	return GetSwarmingInstance(pool).SwarmingClient
}

// GetCASClient returns the CAS client for the given Swarming pool.
func GetCASClient(pool string) *cas.CAS {
	return GetSwarmingInstance(pool).CasClient
}

func getPoolDetails(ctx context.Context, pool string) (*types.PoolDetails, error) {
	swarmingClient := *GetSwarmingClient(pool)
	bots, err := swarmingClient.ListBotsForPool(ctx, pool)
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
	return &types.PoolDetails{
		OsTypes:         osTypes,
		OsToDeviceTypes: osToDeviceTypes,
	}, nil
}

// GetDetailsOfAllPools returns details for each of the known Swarming pools.
func GetDetailsOfAllPools(ctx context.Context) (map[string]*types.PoolDetails, error) {
	poolToDetails := map[string]*types.PoolDetails{}
	for pool := range PoolsToSwarmingInstance {
		details, err := getPoolDetails(ctx, pool)
		if err != nil {
			return nil, err
		}
		poolToDetails[pool] = details
	}
	return poolToDetails, nil
}

// AddLeasingArtifactsToCAS uploads the leasing artifacts and merges them into
// the given CAS input if it exists.
func AddLeasingArtifactsToCAS(ctx context.Context, pool string, casInput *swarming_api.SwarmingRpcsCASReference) (string, error) {
	client := *GetCASClient(pool)

	// Upload the leasing artifacts.
	// TODO(rmistry): After this has been done once, we should be able to just
	// use the digest as a constant.
	leasingScriptDigest, err := client.Upload(ctx, *artifactsDir, []string{"leasing.py"}, nil)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if casInput == nil {
		return leasingScriptDigest, nil
	}
	return client.Merge(ctx, []string{leasingScriptDigest, rbe.DigestToString(casInput.Digest.Hash, casInput.Digest.SizeBytes)})
}

// GetSwarmingTask retrieves the given Swarming task.
func GetSwarmingTask(ctx context.Context, pool, taskID string) (*swarming_api.SwarmingRpcsTaskResult, error) {
	swarmingClient := *GetSwarmingClient(pool)
	return swarmingClient.GetTask(ctx, taskID, false)
}

// GetSwarmingTaskMetadata returns the metadata for the given Swarming task.
func GetSwarmingTaskMetadata(ctx context.Context, pool, taskID string) (*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	swarmingClient := *GetSwarmingClient(pool)
	return swarmingClient.GetTaskMetadata(ctx, taskID)
}

// IsBotIDValid returns true iff the given bot exists in the given pool.
func IsBotIDValid(ctx context.Context, pool, botID string) (bool, error) {
	swarmingClient := *GetSwarmingClient(pool)
	dims := map[string]string{
		"pool": pool,
		"id":   botID,
	}
	bots, err := swarmingClient.ListBots(ctx, dims)
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
	if bots[0].BotId == botID {
		return true, nil
	}
	return false, fmt.Errorf("%s returned %s instead of the expected %s", dims, bots[1].BotId, botID)
}

// TriggerSwarmingTask triggers the given Swarming task.
func TriggerSwarmingTask(ctx context.Context, pool, requester, datastoreID, osType, deviceType, botID, serverURL, casDigest, relativeCwd string, cipdInput *swarming_api.SwarmingRpcsCipdInput, cmd []string) (string, error) {
	dimsMap := map[string]string{
		"pool": pool,
	}
	if osType != "" {
		dimsMap["os"] = osType
	}
	if deviceType != "" {
		dimsMap["device_type"] = deviceType
	}
	if botID != "" {
		dimsMap["id"] = botID
	}
	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dimsMap))
	for k, v := range dimsMap {
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
	}

	// Always include cpython for Windows. See skbug.com/9501 for context and
	// for why we do not include it for all architectures.
	pythonBinary := "python/bin/python3"
	if cipdInput == nil {
		cipdInput = &swarming_api.SwarmingRpcsCipdInput{}
	}
	if cipdInput.Packages == nil {
		cipdInput.Packages = []*swarming_api.SwarmingRpcsCipdPackage{cpythonPackage}
	} else {
		cipdInput.Packages = append(cipdInput.Packages, cpythonPackage)
	}

	// Arguments that will be passed to leasing.py
	extraArgs := []string{
		"--task-id", datastoreID,
		"--os-type", osType,
		"--leasing-server", serverURL,
		"--debug-command", strings.Join(cmd, " "),
		"--command-relative-dir", relativeCwd,
	}

	// Construct the command.
	command := []string{pythonBinary, "leasing.py"}
	command = append(command, extraArgs...)

	swarmingInstance := GetSwarmingInstance(pool)
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
					CipdInput:            cipdInput,
					Dimensions:           dims,
					ExecutionTimeoutSecs: executionTimeoutSecs,
					Command:              command,
					IoTimeoutSecs:        ioTimeoutSecs,
				},
			},
		},
		User: "skiabot@google.com",
	}

	casInput, err := swarming.MakeCASReference(casDigest, swarmingInstance.CasInstance)
	if err != nil {
		return "", skerr.Wrapf(err, "Invalid CAS input")
	}
	taskRequest.TaskSlices[0].Properties.CasInputRoot = casInput

	swarmingClient := *GetSwarmingClient(pool)
	resp, err := swarmingClient.TriggerTask(ctx, taskRequest)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s", err)
	}
	return resp.TaskId, nil
}

// GetSwarmingTaskLink returns a link to the given Swarming task.
func GetSwarmingTaskLink(server, taskID string) string {
	return fmt.Sprintf("https://%s/task?id=%s", server, taskID)
}

// GetSwarmingBotLink returns a link to the given Swarming bot.
func GetSwarmingBotLink(server, botID string) string {
	return fmt.Sprintf("https://%s/bot?id=%s", server, botID)
}
