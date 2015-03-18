// Package androidbuildinternal provides access to the .
//
// Usage example:
//
//   import "google.golang.org/api/androidbuildinternal/v2beta1"
//   ...
//   androidbuildinternalService, err := androidbuildinternal.New(oauthHttpClient)
package androidbuildinternal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace
var _ = context.Background

const apiId = "androidbuildinternal:v2beta1"
const apiName = "androidbuildinternal"
const apiVersion = "v2beta1"
const basePath = "https://www.googleapis.com/android/internal/build/v2beta1/"

// OAuth2 scopes used by this API.
const (
	// View and manage Internal Android Build status and results
	AndroidbuildInternalScope = "https://www.googleapis.com/auth/androidbuild.internal"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Branch = NewBranchService(s)
	s.Bughash = NewBughashService(s)
	s.Build = NewBuildService(s)
	s.Buildartifact = NewBuildartifactService(s)
	s.Buildattempt = NewBuildattemptService(s)
	s.Buildid = NewBuildidService(s)
	s.Buildrequest = NewBuildrequestService(s)
	s.Deviceblob = NewDeviceblobService(s)
	s.Target = NewTargetService(s)
	s.Testresult = NewTestresultService(s)
	return s, nil
}

type Service struct {
	client   *http.Client
	BasePath string // API endpoint base URL

	Branch *BranchService

	Bughash *BughashService

	Build *BuildService

	Buildartifact *BuildartifactService

	Buildattempt *BuildattemptService

	Buildid *BuildidService

	Buildrequest *BuildrequestService

	Deviceblob *DeviceblobService

	Target *TargetService

	Testresult *TestresultService
}

func NewBranchService(s *Service) *BranchService {
	rs := &BranchService{s: s}
	return rs
}

type BranchService struct {
	s *Service
}

func NewBughashService(s *Service) *BughashService {
	rs := &BughashService{s: s}
	return rs
}

type BughashService struct {
	s *Service
}

func NewBuildService(s *Service) *BuildService {
	rs := &BuildService{s: s}
	return rs
}

type BuildService struct {
	s *Service
}

func NewBuildartifactService(s *Service) *BuildartifactService {
	rs := &BuildartifactService{s: s}
	return rs
}

type BuildartifactService struct {
	s *Service
}

func NewBuildattemptService(s *Service) *BuildattemptService {
	rs := &BuildattemptService{s: s}
	return rs
}

type BuildattemptService struct {
	s *Service
}

func NewBuildidService(s *Service) *BuildidService {
	rs := &BuildidService{s: s}
	return rs
}

type BuildidService struct {
	s *Service
}

func NewBuildrequestService(s *Service) *BuildrequestService {
	rs := &BuildrequestService{s: s}
	return rs
}

type BuildrequestService struct {
	s *Service
}

func NewDeviceblobService(s *Service) *DeviceblobService {
	rs := &DeviceblobService{s: s}
	return rs
}

type DeviceblobService struct {
	s *Service
}

func NewTargetService(s *Service) *TargetService {
	rs := &TargetService{s: s}
	return rs
}

type TargetService struct {
	s *Service
}

func NewTestresultService(s *Service) *TestresultService {
	rs := &TestresultService{s: s}
	return rs
}

type TestresultService struct {
	s *Service
}

type ApkSignResult struct {
	Apk string `json:"apk,omitempty"`

	ErrorMessage string `json:"errorMessage,omitempty"`

	Path string `json:"path,omitempty"`

	SignedApkArtifactName string `json:"signedApkArtifactName,omitempty"`

	Success bool `json:"success,omitempty"`
}

type BranchConfig struct {
	BuildPrefix string `json:"buildPrefix,omitempty"`

	BuildRequest *BranchConfigBuildRequestConfig `json:"buildRequest,omitempty"`

	BuildUpdateAcl string `json:"buildUpdateAcl,omitempty"`

	DevelopmentBranch string `json:"developmentBranch,omitempty"`

	DisplayName string `json:"displayName,omitempty"`

	External *BranchConfigExternalBuildConfig `json:"external,omitempty"`

	Flashstation *BranchConfigFlashStationConfig `json:"flashstation,omitempty"`

	Manifest *ManifestLocation `json:"manifest,omitempty"`

	Name string `json:"name,omitempty"`

	ReleaseBranch bool `json:"releaseBranch,omitempty"`

	SubmitQueue *BranchConfigSubmitQueueBranchConfig `json:"submitQueue,omitempty"`

	Submitted *BranchConfigSubmittedBuildConfig `json:"submitted,omitempty"`

	Targets []*Target `json:"targets,omitempty"`
}

type BranchConfigBuildRequestConfig struct {
	AclName string `json:"aclName,omitempty"`
}

type BranchConfigExternalBuildConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

type BranchConfigFlashStationConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	Products []string `json:"products,omitempty"`
}

type BranchConfigSubmitQueueBranchConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	Weight int64 `json:"weight,omitempty"`
}

type BranchConfigSubmittedBuildConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

type BranchListResponse struct {
	Branches []*BranchConfig `json:"branches,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type Bug struct {
	BugId int64 `json:"bugId,omitempty,string"`

	DuplicateBugId int64 `json:"duplicateBugId,omitempty,string"`

	FixedIn googleapi.Int64s `json:"fixedIn,omitempty"`

	Hotlists googleapi.Int64s `json:"hotlists,omitempty"`

	LineGroups []*BugBugHashLines `json:"lineGroups,omitempty"`

	ModifiedDate int64 `json:"modifiedDate,omitempty,string"`

	Owner string `json:"owner,omitempty"`

	Priority string `json:"priority,omitempty"`

	Resolution string `json:"resolution,omitempty"`

	ResolvedDate int64 `json:"resolvedDate,omitempty,string"`

	Severity string `json:"severity,omitempty"`

	Status string `json:"status,omitempty"`

	Summary string `json:"summary,omitempty"`

	TargetedToVersions []string `json:"targetedToVersions,omitempty"`
}

type BugBugHashLines struct {
	Lines googleapi.Int64s `json:"lines,omitempty"`
}

type BugHash struct {
	Bugs []*Bug `json:"bugs,omitempty"`

	Hash string `json:"hash,omitempty"`

	Namespace string `json:"namespace,omitempty"`

	Revision string `json:"revision,omitempty"`
}

type BugHashListResponse struct {
	Bug_hashes []*BugHash `json:"bug_hashes,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type Build struct {
	AppProps []*BuildApplicationPropEntry `json:"appProps,omitempty"`

	Branch string `json:"branch,omitempty"`

	BuildAttemptStatus string `json:"buildAttemptStatus,omitempty"`

	BuildId string `json:"buildId,omitempty"`

	Changes []*Change `json:"changes,omitempty"`

	CreationTimestamp int64 `json:"creationTimestamp,omitempty,string"`

	PreviousBuildId string `json:"previousBuildId,omitempty"`

	ReleaseCandidateName string `json:"releaseCandidateName,omitempty"`

	Revision string `json:"revision,omitempty"`

	Signed bool `json:"signed,omitempty"`

	Successful bool `json:"successful,omitempty"`

	Target *Target `json:"target,omitempty"`
}

type BuildApplicationPropEntry struct {
	Application string `json:"application,omitempty"`

	Key string `json:"key,omitempty"`

	Value string `json:"value,omitempty"`
}

type BuildArtifactListResponse struct {
	Artifacts []*BuildArtifactMetadata `json:"artifacts,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type BuildArtifactMetadata struct {
	ContentType string `json:"contentType,omitempty"`

	LastModifiedTime int64 `json:"lastModifiedTime,omitempty,string"`

	Md5 string `json:"md5,omitempty"`

	Name string `json:"name,omitempty"`

	Revision string `json:"revision,omitempty"`

	Size int64 `json:"size,omitempty,string"`
}

type BuildAttempt struct {
	BuildProp map[string]string `json:"buildProp,omitempty"`

	ErrorMessage string `json:"errorMessage,omitempty"`

	Id int64 `json:"id,omitempty"`

	LastSuccessfulStatus string `json:"lastSuccessfulStatus,omitempty"`

	OtaFile string `json:"otaFile,omitempty"`

	PartitionSizes map[string]PartitionSize `json:"partitionSizes,omitempty"`

	RepoConfig map[string]string `json:"repoConfig,omitempty"`

	Revision string `json:"revision,omitempty"`

	Status string `json:"status,omitempty"`

	Successful bool `json:"successful,omitempty"`

	SymbolFiles []string `json:"symbolFiles,omitempty"`

	TimestampEnd int64 `json:"timestampEnd,omitempty,string"`

	TimestampStart int64 `json:"timestampStart,omitempty,string"`

	UpdaterFile string `json:"updaterFile,omitempty"`
}

type BuildAttemptListResponse struct {
	Attempts []*BuildAttempt `json:"attempts,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type BuildId struct {
	BuildId string `json:"buildId,omitempty"`
}

type BuildIdListResponse struct {
	Buildids []*BuildId `json:"buildids,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type BuildListResponse struct {
	Builds []*Build `json:"builds,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type BuildRequest struct {
	Branch string `json:"branch,omitempty"`

	Id int64 `json:"id,omitempty,string"`

	Requester *Email `json:"requester,omitempty"`

	Revision string `json:"revision,omitempty"`

	Rollup *BuildRequestRollupConfig `json:"rollup,omitempty"`

	Status string `json:"status,omitempty"`

	Type string `json:"type,omitempty"`
}

type BuildRequestListResponse struct {
	Build_requests []*BuildRequest `json:"build_requests,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type BuildRequestRollupConfig struct {
	BuildId string `json:"buildId,omitempty"`

	CutBuildId string `json:"cutBuildId,omitempty"`
}

type BuildSignResponse struct {
	Results []*ApkSignResult `json:"results,omitempty"`
}

type Change struct {
	Branch string `json:"branch,omitempty"`

	ChangeId string `json:"changeId,omitempty"`

	ChangeNumber int64 `json:"changeNumber,omitempty,string"`

	CreationTime int64 `json:"creationTime,omitempty,string"`

	Host string `json:"host,omitempty"`

	LastModificationTime int64 `json:"lastModificationTime,omitempty,string"`

	LatestRevision string `json:"latestRevision,omitempty"`

	NewPatchsetBuildId string `json:"newPatchsetBuildId,omitempty"`

	Owner *User `json:"owner,omitempty"`

	Patchset int64 `json:"patchset,omitempty"`

	Project string `json:"project,omitempty"`

	Revisions []*Revision `json:"revisions,omitempty"`

	Status string `json:"status,omitempty"`
}

type CommitInfo struct {
	Author *User `json:"author,omitempty"`

	CommitId string `json:"commitId,omitempty"`

	CommitMessage string `json:"commitMessage,omitempty"`

	Committer *User `json:"committer,omitempty"`

	Parent *CommitInfo `json:"parent,omitempty"`

	Subject string `json:"subject,omitempty"`
}

type DeviceBlobListResponse struct {
	Blobs []*BuildArtifactMetadata `json:"blobs,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`
}

type Email struct {
	Email string `json:"email,omitempty"`

	Id int64 `json:"id,omitempty,string"`
}

type FetchConfiguration struct {
	Method string `json:"method,omitempty"`

	Ref string `json:"ref,omitempty"`

	Url string `json:"url,omitempty"`
}

type GitManifestLocation struct {
	Branch string `json:"branch,omitempty"`

	FilePath string `json:"filePath,omitempty"`

	Host string `json:"host,omitempty"`

	RepoPath string `json:"repoPath,omitempty"`
}

type ManifestLocation struct {
	Git *GitManifestLocation `json:"git,omitempty"`

	Url *UrlManifestLocation `json:"url,omitempty"`
}

type PartitionSize struct {
	Limit int64 `json:"limit,omitempty,string"`

	Reserve int64 `json:"reserve,omitempty,string"`

	Size int64 `json:"size,omitempty,string"`
}

type Revision struct {
	Commit *CommitInfo `json:"commit,omitempty"`

	Fetchs []*FetchConfiguration `json:"fetchs,omitempty"`

	GitRevision string `json:"gitRevision,omitempty"`

	PatchSet int64 `json:"patchSet,omitempty"`
}

type Target struct {
	BuildCommands []string `json:"buildCommands,omitempty"`

	BuildPlatform string `json:"buildPlatform,omitempty"`

	JavaVersion string `json:"javaVersion,omitempty"`

	Name string `json:"name,omitempty"`

	Signing *TargetSigningConfig `json:"signing,omitempty"`

	SubmitQueue *TargetSubmitQueueTargetConfig `json:"submitQueue,omitempty"`

	Target string `json:"target,omitempty"`
}

type TargetListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	Targets []*Target `json:"targets,omitempty"`
}

type TargetSigningConfig struct {
	Apks []*TargetSigningConfigApk `json:"apks,omitempty"`

	DefaultApks []string `json:"defaultApks,omitempty"`

	Otas []*TargetSigningConfigLooseOTA `json:"otas,omitempty"`

	PackageType string `json:"packageType,omitempty"`
}

type TargetSigningConfigApk struct {
	AclName string `json:"aclName,omitempty"`

	Key string `json:"key,omitempty"`

	MicroApks []*TargetSigningConfigMicroApk `json:"microApks,omitempty"`

	Name string `json:"name,omitempty"`

	PackageName string `json:"packageName,omitempty"`
}

type TargetSigningConfigLooseOTA struct {
	AclName string `json:"aclName,omitempty"`

	Key string `json:"key,omitempty"`

	Name string `json:"name,omitempty"`
}

type TargetSigningConfigMicroApk struct {
	Key string `json:"key,omitempty"`

	Name string `json:"name,omitempty"`
}

type TargetSubmitQueueTargetConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	Weight int64 `json:"weight,omitempty"`

	Whitelists []string `json:"whitelists,omitempty"`
}

type TestResult struct {
	Id int64 `json:"id,omitempty,string"`

	PostedToGerrit bool `json:"postedToGerrit,omitempty"`

	Revision string `json:"revision,omitempty"`

	Status string `json:"status,omitempty"`

	Summary string `json:"summary,omitempty"`
}

type TestResultListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	TestResults []*TestResult `json:"testResults,omitempty"`
}

type UrlManifestLocation struct {
	Url string `json:"url,omitempty"`
}

type User struct {
	Email string `json:"email,omitempty"`

	Name string `json:"name,omitempty"`
}

// method id "androidbuildinternal.branch.get":

type BranchGetCall struct {
	s          *Service
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *BranchService) Get(resourceId string) *BranchGetCall {
	c := &BranchGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BranchGetCall) Fields(s ...googleapi.Field) *BranchGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BranchGetCall) Do() (*BranchConfig, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BranchConfig
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.branch.get",
	//   "parameterOrder": [
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "branches/{resourceId}",
	//   "response": {
	//     "$ref": "BranchConfig"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.branch.list":

type BranchListCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// List:
func (r *BranchService) List() *BranchListCall {
	c := &BranchListCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// BuildPrefix sets the optional parameter "buildPrefix":
func (c *BranchListCall) BuildPrefix(buildPrefix string) *BranchListCall {
	c.opt_["buildPrefix"] = buildPrefix
	return c
}

// FlashstationEnabled sets the optional parameter
// "flashstationEnabled":
func (c *BranchListCall) FlashstationEnabled(flashstationEnabled bool) *BranchListCall {
	c.opt_["flashstationEnabled"] = flashstationEnabled
	return c
}

// FlashstationProduct sets the optional parameter
// "flashstationProduct":
func (c *BranchListCall) FlashstationProduct(flashstationProduct string) *BranchListCall {
	c.opt_["flashstationProduct"] = flashstationProduct
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BranchListCall) MaxResults(maxResults int64) *BranchListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BranchListCall) PageToken(pageToken string) *BranchListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BranchListCall) Fields(s ...googleapi.Field) *BranchListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BranchListCall) Do() (*BranchListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["buildPrefix"]; ok {
		params.Set("buildPrefix", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["flashstationEnabled"]; ok {
		params.Set("flashstationEnabled", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["flashstationProduct"]; ok {
		params.Set("flashstationProduct", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BranchListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.branch.list",
	//   "parameters": {
	//     "buildPrefix": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "flashstationEnabled": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "flashstationProduct": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "branches",
	//   "response": {
	//     "$ref": "BranchListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.bughash.get":

type BughashGetCall struct {
	s          *Service
	namespace  string
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *BughashService) Get(namespace string, resourceId string) *BughashGetCall {
	c := &BughashGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.namespace = namespace
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashGetCall) Fields(s ...googleapi.Field) *BughashGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BughashGetCall) Do() (*BugHash, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BugHash
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.bughash.get",
	//   "parameterOrder": [
	//     "namespace",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "bugHashes/{namespace}/{resourceId}",
	//   "response": {
	//     "$ref": "BugHash"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.bughash.list":

type BughashListCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// List:
func (r *BughashService) List() *BughashListCall {
	c := &BughashListCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// BugId sets the optional parameter "bugId":
func (c *BughashListCall) BugId(bugId int64) *BughashListCall {
	c.opt_["bugId"] = bugId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BughashListCall) MaxResults(maxResults int64) *BughashListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// Namespace sets the optional parameter "namespace":
func (c *BughashListCall) Namespace(namespace string) *BughashListCall {
	c.opt_["namespace"] = namespace
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BughashListCall) PageToken(pageToken string) *BughashListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashListCall) Fields(s ...googleapi.Field) *BughashListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BughashListCall) Do() (*BugHashListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["bugId"]; ok {
		params.Set("bugId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["namespace"]; ok {
		params.Set("namespace", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BugHashListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.bughash.list",
	//   "parameters": {
	//     "bugId": {
	//       "format": "int64",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "namespace": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "bugHashes",
	//   "response": {
	//     "$ref": "BugHashListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.bughash.patch":

type BughashPatchCall struct {
	s          *Service
	namespace  string
	resourceId string
	bughash    *BugHash
	opt_       map[string]interface{}
}

// Patch:
func (r *BughashService) Patch(namespace string, resourceId string, bughash *BugHash) *BughashPatchCall {
	c := &BughashPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.namespace = namespace
	c.resourceId = resourceId
	c.bughash = bughash
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashPatchCall) Fields(s ...googleapi.Field) *BughashPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BughashPatchCall) Do() (*BugHash, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.bughash)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BugHash
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.bughash.patch",
	//   "parameterOrder": [
	//     "namespace",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "bugHashes/{namespace}/{resourceId}",
	//   "request": {
	//     "$ref": "BugHash"
	//   },
	//   "response": {
	//     "$ref": "BugHash"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.bughash.update":

type BughashUpdateCall struct {
	s          *Service
	namespace  string
	resourceId string
	bughash    *BugHash
	opt_       map[string]interface{}
}

// Update:
func (r *BughashService) Update(namespace string, resourceId string, bughash *BugHash) *BughashUpdateCall {
	c := &BughashUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.namespace = namespace
	c.resourceId = resourceId
	c.bughash = bughash
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashUpdateCall) Fields(s ...googleapi.Field) *BughashUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BughashUpdateCall) Do() (*BugHash, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.bughash)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BugHash
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.bughash.update",
	//   "parameterOrder": [
	//     "namespace",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "bugHashes/{namespace}/{resourceId}",
	//   "request": {
	//     "$ref": "BugHash"
	//   },
	//   "response": {
	//     "$ref": "BugHash"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.get":

type BuildGetCall struct {
	s       *Service
	buildId string
	target  string
	opt_    map[string]interface{}
}

// Get:
func (r *BuildService) Get(buildId string, target string) *BuildGetCall {
	c := &BuildGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	return c
}

// ExtraFields sets the optional parameter "extraFields":
func (c *BuildGetCall) ExtraFields(extraFields string) *BuildGetCall {
	c.opt_["extraFields"] = extraFields
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildGetCall) ResourceId(resourceId string) *BuildGetCall {
	c.opt_["resourceId"] = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildGetCall) Fields(s ...googleapi.Field) *BuildGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildGetCall) Do() (*Build, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["extraFields"]; ok {
		params.Set("extraFields", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["resourceId"]; ok {
		params.Set("resourceId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Build
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.build.get",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "extraFields": {
	//       "enum": [
	//         "all",
	//         "changeInfo"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}",
	//   "response": {
	//     "$ref": "Build"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.insert":

type BuildInsertCall struct {
	s     *Service
	build *Build
	opt_  map[string]interface{}
}

// Insert:
func (r *BuildService) Insert(build *Build) *BuildInsertCall {
	c := &BuildInsertCall{s: r.s, opt_: make(map[string]interface{})}
	c.build = build
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildInsertCall) Fields(s ...googleapi.Field) *BuildInsertCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildInsertCall) Do() (*Build, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Build
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.build.insert",
	//   "path": "builds",
	//   "request": {
	//     "$ref": "Build"
	//   },
	//   "response": {
	//     "$ref": "Build"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.list":

type BuildListCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// List:
func (r *BuildService) List() *BuildListCall {
	c := &BuildListCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// Branch sets the optional parameter "branch":
func (c *BuildListCall) Branch(branch string) *BuildListCall {
	c.opt_["branch"] = branch
	return c
}

// BuildAttemptStatus sets the optional parameter "buildAttemptStatus":
func (c *BuildListCall) BuildAttemptStatus(buildAttemptStatus string) *BuildListCall {
	c.opt_["buildAttemptStatus"] = buildAttemptStatus
	return c
}

// BuildId sets the optional parameter "buildId":
func (c *BuildListCall) BuildId(buildId string) *BuildListCall {
	c.opt_["buildId"] = buildId
	return c
}

// BuildType sets the optional parameter "buildType":
func (c *BuildListCall) BuildType(buildType string) *BuildListCall {
	c.opt_["buildType"] = buildType
	return c
}

// EndBuildId sets the optional parameter "endBuildId":
func (c *BuildListCall) EndBuildId(endBuildId string) *BuildListCall {
	c.opt_["endBuildId"] = endBuildId
	return c
}

// ExtraFields sets the optional parameter "extraFields":
func (c *BuildListCall) ExtraFields(extraFields string) *BuildListCall {
	c.opt_["extraFields"] = extraFields
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildListCall) MaxResults(maxResults int64) *BuildListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildListCall) PageToken(pageToken string) *BuildListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// ReleaseCandidateName sets the optional parameter
// "releaseCandidateName":
func (c *BuildListCall) ReleaseCandidateName(releaseCandidateName string) *BuildListCall {
	c.opt_["releaseCandidateName"] = releaseCandidateName
	return c
}

// Signed sets the optional parameter "signed":
func (c *BuildListCall) Signed(signed bool) *BuildListCall {
	c.opt_["signed"] = signed
	return c
}

// StartBuildId sets the optional parameter "startBuildId":
func (c *BuildListCall) StartBuildId(startBuildId string) *BuildListCall {
	c.opt_["startBuildId"] = startBuildId
	return c
}

// Successful sets the optional parameter "successful":
func (c *BuildListCall) Successful(successful bool) *BuildListCall {
	c.opt_["successful"] = successful
	return c
}

// Target sets the optional parameter "target":
func (c *BuildListCall) Target(target string) *BuildListCall {
	c.opt_["target"] = target
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildListCall) Fields(s ...googleapi.Field) *BuildListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildListCall) Do() (*BuildListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["branch"]; ok {
		params.Set("branch", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["buildAttemptStatus"]; ok {
		params.Set("buildAttemptStatus", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["buildId"]; ok {
		params.Set("buildId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["buildType"]; ok {
		params.Set("buildType", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["endBuildId"]; ok {
		params.Set("endBuildId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["extraFields"]; ok {
		params.Set("extraFields", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["releaseCandidateName"]; ok {
		params.Set("releaseCandidateName", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["signed"]; ok {
		params.Set("signed", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["startBuildId"]; ok {
		params.Set("startBuildId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["successful"]; ok {
		params.Set("successful", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["target"]; ok {
		params.Set("target", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.build.list",
	//   "parameters": {
	//     "branch": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "buildAttemptStatus": {
	//       "enum": [
	//         "building",
	//         "built",
	//         "complete",
	//         "error",
	//         "pending",
	//         "pendingGerritUpload",
	//         "synced",
	//         "syncing",
	//         "testing"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         "",
	//         "",
	//         "",
	//         "",
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "buildType": {
	//       "enum": [
	//         "external",
	//         "pending",
	//         "submitted"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "endBuildId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "extraFields": {
	//       "enum": [
	//         "all",
	//         "changeInfo"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "releaseCandidateName": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "signed": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "startBuildId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "successful": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "target": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds",
	//   "response": {
	//     "$ref": "BuildListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.patch":

type BuildPatchCall struct {
	s       *Service
	buildId string
	target  string
	build   *Build
	opt_    map[string]interface{}
}

// Patch:
func (r *BuildService) Patch(buildId string, target string, build *Build) *BuildPatchCall {
	c := &BuildPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.build = build
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildPatchCall) ResourceId(resourceId string) *BuildPatchCall {
	c.opt_["resourceId"] = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildPatchCall) Fields(s ...googleapi.Field) *BuildPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildPatchCall) Do() (*Build, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["resourceId"]; ok {
		params.Set("resourceId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Build
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.build.patch",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}",
	//   "request": {
	//     "$ref": "Build"
	//   },
	//   "response": {
	//     "$ref": "Build"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.pop":

type BuildPopCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// Pop:
func (r *BuildService) Pop() *BuildPopCall {
	c := &BuildPopCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// BuildType sets the optional parameter "buildType":
func (c *BuildPopCall) BuildType(buildType string) *BuildPopCall {
	c.opt_["buildType"] = buildType
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildPopCall) Fields(s ...googleapi.Field) *BuildPopCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildPopCall) Do() (*Build, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["buildType"]; ok {
		params.Set("buildType", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/pop")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Build
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.build.pop",
	//   "parameters": {
	//     "buildType": {
	//       "enum": [
	//         "external",
	//         "pending",
	//         "submitted"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/pop",
	//   "response": {
	//     "$ref": "Build"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.sign":

type BuildSignCall struct {
	s       *Service
	buildId string
	target  string
	opt_    map[string]interface{}
}

// Sign:
func (r *BuildService) Sign(buildId string, target string) *BuildSignCall {
	c := &BuildSignCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	return c
}

// Apks sets the optional parameter "apks":
func (c *BuildSignCall) Apks(apks string) *BuildSignCall {
	c.opt_["apks"] = apks
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildSignCall) Fields(s ...googleapi.Field) *BuildSignCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildSignCall) Do() (*BuildSignResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["apks"]; ok {
		params.Set("apks", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/sign")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildSignResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.build.sign",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "apks": {
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/sign",
	//   "response": {
	//     "$ref": "BuildSignResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.build.update":

type BuildUpdateCall struct {
	s       *Service
	buildId string
	target  string
	build   *Build
	opt_    map[string]interface{}
}

// Update:
func (r *BuildService) Update(buildId string, target string, build *Build) *BuildUpdateCall {
	c := &BuildUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.build = build
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildUpdateCall) ResourceId(resourceId string) *BuildUpdateCall {
	c.opt_["resourceId"] = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildUpdateCall) Fields(s ...googleapi.Field) *BuildUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildUpdateCall) Do() (*Build, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["resourceId"]; ok {
		params.Set("resourceId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Build
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.build.update",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}",
	//   "request": {
	//     "$ref": "Build"
	//   },
	//   "response": {
	//     "$ref": "Build"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildartifact.delete":

type BuildartifactDeleteCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	resourceId string
	opt_       map[string]interface{}
}

// Delete:
func (r *BuildartifactService) Delete(buildId string, target string, attemptId string, resourceId string) *BuildartifactDeleteCall {
	c := &BuildartifactDeleteCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactDeleteCall) Fields(s ...googleapi.Field) *BuildartifactDeleteCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildartifactDeleteCall) Do() error {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return err
	}
	return nil
	// {
	//   "httpMethod": "DELETE",
	//   "id": "androidbuildinternal.buildartifact.delete",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}",
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildartifact.get":

type BuildartifactGetCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *BuildartifactService) Get(buildId string, target string, attemptId string, resourceId string) *BuildartifactGetCall {
	c := &BuildartifactGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactGetCall) Fields(s ...googleapi.Field) *BuildartifactGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildartifactGetCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildartifact.get",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}",
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ],
	//   "supportsMediaDownload": true
	// }

}

// method id "androidbuildinternal.buildartifact.list":

type BuildartifactListCall struct {
	s         *Service
	buildId   string
	target    string
	attemptId string
	opt_      map[string]interface{}
}

// List:
func (r *BuildartifactService) List(buildId string, target string, attemptId string) *BuildartifactListCall {
	c := &BuildartifactListCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildartifactListCall) MaxResults(maxResults int64) *BuildartifactListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildartifactListCall) PageToken(pageToken string) *BuildartifactListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactListCall) Fields(s ...googleapi.Field) *BuildartifactListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildartifactListCall) Do() (*BuildArtifactListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildArtifactListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildartifact.list",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts",
	//   "response": {
	//     "$ref": "BuildArtifactListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildartifact.patch":

type BuildartifactPatchCall struct {
	s                     *Service
	buildId               string
	target                string
	attemptId             string
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	opt_                  map[string]interface{}
}

// Patch:
func (r *BuildartifactService) Patch(buildId string, target string, attemptId string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *BuildartifactPatchCall {
	c := &BuildartifactPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactPatchCall) Fields(s ...googleapi.Field) *BuildartifactPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildartifactPatchCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.buildartifact.patch",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}",
	//   "request": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildartifact.update":

type BuildartifactUpdateCall struct {
	s                     *Service
	buildId               string
	target                string
	attemptId             string
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	opt_                  map[string]interface{}
	media_                io.Reader
	resumable_            googleapi.SizeReaderAt
	mediaType_            string
	ctx_                  context.Context
	protocol_             string
}

// Update:
func (r *BuildartifactService) Update(buildId string, target string, attemptId string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *BuildartifactUpdateCall {
	c := &BuildartifactUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Media specifies the media to upload in a single chunk.
// At most one of Media and ResumableMedia may be set.
func (c *BuildartifactUpdateCall) Media(r io.Reader) *BuildartifactUpdateCall {
	c.media_ = r
	c.protocol_ = "multipart"
	return c
}

// ResumableMedia specifies the media to upload in chunks and can be cancelled with ctx.
// At most one of Media and ResumableMedia may be set.
// mediaType identifies the MIME media type of the upload, such as "image/png".
// If mediaType is "", it will be auto-detected.
func (c *BuildartifactUpdateCall) ResumableMedia(ctx context.Context, r io.ReaderAt, size int64, mediaType string) *BuildartifactUpdateCall {
	c.ctx_ = ctx
	c.resumable_ = io.NewSectionReader(r, 0, size)
	c.mediaType_ = mediaType
	c.protocol_ = "resumable"
	return c
}

// ProgressUpdater provides a callback function that will be called after every chunk.
// It should be a low-latency function in order to not slow down the upload operation.
// This should only be called when using ResumableMedia (as opposed to Media).
func (c *BuildartifactUpdateCall) ProgressUpdater(pu googleapi.ProgressUpdater) *BuildartifactUpdateCall {
	c.opt_["progressUpdater"] = pu
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactUpdateCall) Fields(s ...googleapi.Field) *BuildartifactUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildartifactUpdateCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	var progressUpdater_ googleapi.ProgressUpdater
	if v, ok := c.opt_["progressUpdater"]; ok {
		if pu, ok := v.(googleapi.ProgressUpdater); ok {
			progressUpdater_ = pu
		}
	}
	if c.media_ != nil || c.resumable_ != nil {
		urls = strings.Replace(urls, "https://www.googleapis.com/", "https://www.googleapis.com/upload/", 1)
		params.Set("uploadType", c.protocol_)
	}
	urls += "?" + params.Encode()
	if c.protocol_ != "resumable" {
		var cancel func()
		cancel, _ = googleapi.ConditionallyIncludeMedia(c.media_, &body, &ctype)
		if cancel != nil {
			defer cancel()
		}
	}
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	if c.protocol_ == "resumable" {
		req.ContentLength = 0
		if c.mediaType_ == "" {
			c.mediaType_ = googleapi.DetectMediaType(c.resumable_)
		}
		req.Header.Set("X-Upload-Content-Type", c.mediaType_)
		req.Body = nil
	} else {
		req.Header.Set("Content-Type", ctype)
	}
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	if c.protocol_ == "resumable" {
		loc := res.Header.Get("Location")
		rx := &googleapi.ResumableUpload{
			Client:        c.s.client,
			URI:           loc,
			Media:         c.resumable_,
			MediaType:     c.mediaType_,
			ContentLength: c.resumable_.Size(),
			Callback:      progressUpdater_,
		}
		res, err = rx.Upload(c.ctx_)
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.buildartifact.update",
	//   "mediaUpload": {
	//     "accept": [
	//       "*/*"
	//     ],
	//     "maxSize": "2GB",
	//     "protocols": {
	//       "resumable": {
	//         "multipart": true,
	//         "path": "/resumable/upload/android/internal/build/v2beta1/builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}"
	//       },
	//       "simple": {
	//         "multipart": true,
	//         "path": "/upload/android/internal/build/v2beta1/builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}"
	//       }
	//     }
	//   },
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}",
	//   "request": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ],
	//   "supportsMediaUpload": true
	// }

}

// method id "androidbuildinternal.buildattempt.get":

type BuildattemptGetCall struct {
	s          *Service
	buildId    string
	target     string
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *BuildattemptService) Get(buildId string, target string, resourceId string) *BuildattemptGetCall {
	c := &BuildattemptGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.resourceId = resourceId
	return c
}

// ExtraFields sets the optional parameter "extraFields":
func (c *BuildattemptGetCall) ExtraFields(extraFields string) *BuildattemptGetCall {
	c.opt_["extraFields"] = extraFields
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptGetCall) Fields(s ...googleapi.Field) *BuildattemptGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildattemptGetCall) Do() (*BuildAttempt, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["extraFields"]; ok {
		params.Set("extraFields", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildAttempt
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildattempt.get",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "extraFields": {
	//       "enum": [
	//         "all",
	//         "buildProp",
	//         "repoConfig"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{resourceId}",
	//   "response": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildattempt.insert":

type BuildattemptInsertCall struct {
	s            *Service
	buildId      string
	target       string
	buildattempt *BuildAttempt
	opt_         map[string]interface{}
}

// Insert:
func (r *BuildattemptService) Insert(buildId string, target string, buildattempt *BuildAttempt) *BuildattemptInsertCall {
	c := &BuildattemptInsertCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.buildattempt = buildattempt
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptInsertCall) Fields(s ...googleapi.Field) *BuildattemptInsertCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildattemptInsertCall) Do() (*BuildAttempt, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildAttempt
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.buildattempt.insert",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts",
	//   "request": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "response": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildattempt.list":

type BuildattemptListCall struct {
	s       *Service
	buildId string
	target  string
	opt_    map[string]interface{}
}

// List:
func (r *BuildattemptService) List(buildId string, target string) *BuildattemptListCall {
	c := &BuildattemptListCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	return c
}

// ExtraFields sets the optional parameter "extraFields":
func (c *BuildattemptListCall) ExtraFields(extraFields string) *BuildattemptListCall {
	c.opt_["extraFields"] = extraFields
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildattemptListCall) MaxResults(maxResults int64) *BuildattemptListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildattemptListCall) PageToken(pageToken string) *BuildattemptListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptListCall) Fields(s ...googleapi.Field) *BuildattemptListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildattemptListCall) Do() (*BuildAttemptListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["extraFields"]; ok {
		params.Set("extraFields", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildAttemptListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildattempt.list",
	//   "parameterOrder": [
	//     "buildId",
	//     "target"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "extraFields": {
	//       "enum": [
	//         "all",
	//         "buildProp",
	//         "repoConfig"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts",
	//   "response": {
	//     "$ref": "BuildAttemptListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildattempt.patch":

type BuildattemptPatchCall struct {
	s            *Service
	target       string
	resourceId   string
	buildId      string
	buildattempt *BuildAttempt
	opt_         map[string]interface{}
}

// Patch:
func (r *BuildattemptService) Patch(target string, resourceId string, buildId string, buildattempt *BuildAttempt) *BuildattemptPatchCall {
	c := &BuildattemptPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.target = target
	c.resourceId = resourceId
	c.buildId = buildId
	c.buildattempt = buildattempt
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptPatchCall) Fields(s ...googleapi.Field) *BuildattemptPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildattemptPatchCall) Do() (*BuildAttempt, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	params.Set("buildId", fmt.Sprintf("%v", c.buildId))
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{target}/attempts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildAttempt
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.buildattempt.patch",
	//   "parameterOrder": [
	//     "target",
	//     "resourceId",
	//     "buildId"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "query",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{target}/attempts/{resourceId}",
	//   "request": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "response": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildattempt.update":

type BuildattemptUpdateCall struct {
	s            *Service
	target       string
	resourceId   string
	buildattempt *BuildAttempt
	opt_         map[string]interface{}
}

// Update:
func (r *BuildattemptService) Update(target string, resourceId string, buildattempt *BuildAttempt) *BuildattemptUpdateCall {
	c := &BuildattemptUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.target = target
	c.resourceId = resourceId
	c.buildattempt = buildattempt
	return c
}

// BuildId sets the optional parameter "buildId":
func (c *BuildattemptUpdateCall) BuildId(buildId string) *BuildattemptUpdateCall {
	c.opt_["buildId"] = buildId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptUpdateCall) Fields(s ...googleapi.Field) *BuildattemptUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildattemptUpdateCall) Do() (*BuildAttempt, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["buildId"]; ok {
		params.Set("buildId", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{target}/attempts/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildAttempt
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.buildattempt.update",
	//   "parameterOrder": [
	//     "target",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "buildId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{target}/attempts/{resourceId}",
	//   "request": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "response": {
	//     "$ref": "BuildAttempt"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildid.list":

type BuildidListCall struct {
	s      *Service
	branch string
	opt_   map[string]interface{}
}

// List:
func (r *BuildidService) List(branch string) *BuildidListCall {
	c := &BuildidListCall{s: r.s, opt_: make(map[string]interface{})}
	c.branch = branch
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildidListCall) MaxResults(maxResults int64) *BuildidListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildidListCall) PageToken(pageToken string) *BuildidListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildidListCall) Fields(s ...googleapi.Field) *BuildidListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildidListCall) Do() (*BuildIdListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildids/{branch}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch": c.branch,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildIdListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildid.list",
	//   "parameterOrder": [
	//     "branch"
	//   ],
	//   "parameters": {
	//     "branch": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "buildids/{branch}",
	//   "response": {
	//     "$ref": "BuildIdListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildrequest.get":

type BuildrequestGetCall struct {
	s          *Service
	resourceId int64
	opt_       map[string]interface{}
}

// Get:
func (r *BuildrequestService) Get(resourceId int64) *BuildrequestGetCall {
	c := &BuildrequestGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestGetCall) Fields(s ...googleapi.Field) *BuildrequestGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildrequestGetCall) Do() (*BuildRequest, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildRequest
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildrequest.get",
	//   "parameterOrder": [
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "buildRequests/{resourceId}",
	//   "response": {
	//     "$ref": "BuildRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildrequest.insert":

type BuildrequestInsertCall struct {
	s            *Service
	buildrequest *BuildRequest
	opt_         map[string]interface{}
}

// Insert:
func (r *BuildrequestService) Insert(buildrequest *BuildRequest) *BuildrequestInsertCall {
	c := &BuildrequestInsertCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestInsertCall) Fields(s ...googleapi.Field) *BuildrequestInsertCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildrequestInsertCall) Do() (*BuildRequest, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildRequest
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.buildrequest.insert",
	//   "path": "buildRequests",
	//   "request": {
	//     "$ref": "BuildRequest"
	//   },
	//   "response": {
	//     "$ref": "BuildRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildrequest.list":

type BuildrequestListCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// List:
func (r *BuildrequestService) List() *BuildrequestListCall {
	c := &BuildrequestListCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// Branch sets the optional parameter "branch":
func (c *BuildrequestListCall) Branch(branch string) *BuildrequestListCall {
	c.opt_["branch"] = branch
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildrequestListCall) MaxResults(maxResults int64) *BuildrequestListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildrequestListCall) PageToken(pageToken string) *BuildrequestListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Status sets the optional parameter "status":
func (c *BuildrequestListCall) Status(status string) *BuildrequestListCall {
	c.opt_["status"] = status
	return c
}

// Type sets the optional parameter "type":
func (c *BuildrequestListCall) Type(type_ string) *BuildrequestListCall {
	c.opt_["type"] = type_
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestListCall) Fields(s ...googleapi.Field) *BuildrequestListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildrequestListCall) Do() (*BuildRequestListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["branch"]; ok {
		params.Set("branch", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["status"]; ok {
		params.Set("status", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["type"]; ok {
		params.Set("type", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildRequestListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.buildrequest.list",
	//   "parameters": {
	//     "branch": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "status": {
	//       "enum": [
	//         "complete",
	//         "failed",
	//         "inProgress",
	//         "pending"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "type": {
	//       "enum": [
	//         "rollup"
	//       ],
	//       "enumDescriptions": [
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "buildRequests",
	//   "response": {
	//     "$ref": "BuildRequestListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildrequest.patch":

type BuildrequestPatchCall struct {
	s            *Service
	resourceId   int64
	buildrequest *BuildRequest
	opt_         map[string]interface{}
}

// Patch:
func (r *BuildrequestService) Patch(resourceId int64, buildrequest *BuildRequest) *BuildrequestPatchCall {
	c := &BuildrequestPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.resourceId = resourceId
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestPatchCall) Fields(s ...googleapi.Field) *BuildrequestPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildrequestPatchCall) Do() (*BuildRequest, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildRequest
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.buildrequest.patch",
	//   "parameterOrder": [
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "buildRequests/{resourceId}",
	//   "request": {
	//     "$ref": "BuildRequest"
	//   },
	//   "response": {
	//     "$ref": "BuildRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.buildrequest.update":

type BuildrequestUpdateCall struct {
	s            *Service
	resourceId   int64
	buildrequest *BuildRequest
	opt_         map[string]interface{}
}

// Update:
func (r *BuildrequestService) Update(resourceId int64, buildrequest *BuildRequest) *BuildrequestUpdateCall {
	c := &BuildrequestUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.resourceId = resourceId
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestUpdateCall) Fields(s ...googleapi.Field) *BuildrequestUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *BuildrequestUpdateCall) Do() (*BuildRequest, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildRequest
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.buildrequest.update",
	//   "parameterOrder": [
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "buildRequests/{resourceId}",
	//   "request": {
	//     "$ref": "BuildRequest"
	//   },
	//   "response": {
	//     "$ref": "BuildRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.deviceblob.get":

type DeviceblobGetCall struct {
	s          *Service
	deviceName string
	binaryType string
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *DeviceblobService) Get(deviceName string, binaryType string, resourceId string) *DeviceblobGetCall {
	c := &DeviceblobGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobGetCall) Fields(s ...googleapi.Field) *DeviceblobGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *DeviceblobGetCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.deviceblob.get",
	//   "parameterOrder": [
	//     "deviceName",
	//     "binaryType",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "binaryType": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "deviceName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "deviceBlobs/{deviceName}/{binaryType}/{resourceId}",
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ],
	//   "supportsMediaDownload": true
	// }

}

// method id "androidbuildinternal.deviceblob.list":

type DeviceblobListCall struct {
	s          *Service
	deviceName string
	opt_       map[string]interface{}
}

// List:
func (r *DeviceblobService) List(deviceName string) *DeviceblobListCall {
	c := &DeviceblobListCall{s: r.s, opt_: make(map[string]interface{})}
	c.deviceName = deviceName
	return c
}

// BinaryType sets the optional parameter "binaryType":
func (c *DeviceblobListCall) BinaryType(binaryType string) *DeviceblobListCall {
	c.opt_["binaryType"] = binaryType
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *DeviceblobListCall) MaxResults(maxResults int64) *DeviceblobListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *DeviceblobListCall) PageToken(pageToken string) *DeviceblobListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Version sets the optional parameter "version":
func (c *DeviceblobListCall) Version(version string) *DeviceblobListCall {
	c.opt_["version"] = version
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobListCall) Fields(s ...googleapi.Field) *DeviceblobListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *DeviceblobListCall) Do() (*DeviceBlobListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["binaryType"]; ok {
		params.Set("binaryType", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["version"]; ok {
		params.Set("version", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *DeviceBlobListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.deviceblob.list",
	//   "parameterOrder": [
	//     "deviceName"
	//   ],
	//   "parameters": {
	//     "binaryType": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "deviceName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "version": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "deviceBlobs/{deviceName}",
	//   "response": {
	//     "$ref": "DeviceBlobListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.deviceblob.patch":

type DeviceblobPatchCall struct {
	s                     *Service
	deviceName            string
	binaryType            string
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	opt_                  map[string]interface{}
}

// Patch:
func (r *DeviceblobService) Patch(deviceName string, binaryType string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *DeviceblobPatchCall {
	c := &DeviceblobPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobPatchCall) Fields(s ...googleapi.Field) *DeviceblobPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *DeviceblobPatchCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.deviceblob.patch",
	//   "parameterOrder": [
	//     "deviceName",
	//     "binaryType",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "binaryType": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "deviceName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "deviceBlobs/{deviceName}/{binaryType}/{resourceId}",
	//   "request": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.deviceblob.update":

type DeviceblobUpdateCall struct {
	s                     *Service
	deviceName            string
	binaryType            string
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	opt_                  map[string]interface{}
	media_                io.Reader
	resumable_            googleapi.SizeReaderAt
	mediaType_            string
	ctx_                  context.Context
	protocol_             string
}

// Update:
func (r *DeviceblobService) Update(deviceName string, binaryType string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *DeviceblobUpdateCall {
	c := &DeviceblobUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Media specifies the media to upload in a single chunk.
// At most one of Media and ResumableMedia may be set.
func (c *DeviceblobUpdateCall) Media(r io.Reader) *DeviceblobUpdateCall {
	c.media_ = r
	c.protocol_ = "multipart"
	return c
}

// ResumableMedia specifies the media to upload in chunks and can be cancelled with ctx.
// At most one of Media and ResumableMedia may be set.
// mediaType identifies the MIME media type of the upload, such as "image/png".
// If mediaType is "", it will be auto-detected.
func (c *DeviceblobUpdateCall) ResumableMedia(ctx context.Context, r io.ReaderAt, size int64, mediaType string) *DeviceblobUpdateCall {
	c.ctx_ = ctx
	c.resumable_ = io.NewSectionReader(r, 0, size)
	c.mediaType_ = mediaType
	c.protocol_ = "resumable"
	return c
}

// ProgressUpdater provides a callback function that will be called after every chunk.
// It should be a low-latency function in order to not slow down the upload operation.
// This should only be called when using ResumableMedia (as opposed to Media).
func (c *DeviceblobUpdateCall) ProgressUpdater(pu googleapi.ProgressUpdater) *DeviceblobUpdateCall {
	c.opt_["progressUpdater"] = pu
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobUpdateCall) Fields(s ...googleapi.Field) *DeviceblobUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *DeviceblobUpdateCall) Do() (*BuildArtifactMetadata, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	var progressUpdater_ googleapi.ProgressUpdater
	if v, ok := c.opt_["progressUpdater"]; ok {
		if pu, ok := v.(googleapi.ProgressUpdater); ok {
			progressUpdater_ = pu
		}
	}
	if c.media_ != nil || c.resumable_ != nil {
		urls = strings.Replace(urls, "https://www.googleapis.com/", "https://www.googleapis.com/upload/", 1)
		params.Set("uploadType", c.protocol_)
	}
	urls += "?" + params.Encode()
	if c.protocol_ != "resumable" {
		var cancel func()
		cancel, _ = googleapi.ConditionallyIncludeMedia(c.media_, &body, &ctype)
		if cancel != nil {
			defer cancel()
		}
	}
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	if c.protocol_ == "resumable" {
		req.ContentLength = 0
		if c.mediaType_ == "" {
			c.mediaType_ = googleapi.DetectMediaType(c.resumable_)
		}
		req.Header.Set("X-Upload-Content-Type", c.mediaType_)
		req.Body = nil
	} else {
		req.Header.Set("Content-Type", ctype)
	}
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	if c.protocol_ == "resumable" {
		loc := res.Header.Get("Location")
		rx := &googleapi.ResumableUpload{
			Client:        c.s.client,
			URI:           loc,
			Media:         c.resumable_,
			MediaType:     c.mediaType_,
			ContentLength: c.resumable_.Size(),
			Callback:      progressUpdater_,
		}
		res, err = rx.Upload(c.ctx_)
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
	}
	var ret *BuildArtifactMetadata
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.deviceblob.update",
	//   "mediaUpload": {
	//     "accept": [
	//       "*/*"
	//     ],
	//     "maxSize": "2GB",
	//     "protocols": {
	//       "resumable": {
	//         "multipart": true,
	//         "path": "/resumable/upload/android/internal/build/v2beta1/deviceBlobs/{deviceName}/{binaryType}/{resourceId}"
	//       },
	//       "simple": {
	//         "multipart": true,
	//         "path": "/upload/android/internal/build/v2beta1/deviceBlobs/{deviceName}/{binaryType}/{resourceId}"
	//       }
	//     }
	//   },
	//   "parameterOrder": [
	//     "deviceName",
	//     "binaryType",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "binaryType": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "deviceName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "deviceBlobs/{deviceName}/{binaryType}/{resourceId}",
	//   "request": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ],
	//   "supportsMediaUpload": true
	// }

}

// method id "androidbuildinternal.target.get":

type TargetGetCall struct {
	s          *Service
	branch     string
	resourceId string
	opt_       map[string]interface{}
}

// Get:
func (r *TargetService) Get(branch string, resourceId string) *TargetGetCall {
	c := &TargetGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.branch = branch
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TargetGetCall) Fields(s ...googleapi.Field) *TargetGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TargetGetCall) Do() (*Target, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{branch}/targets/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch":     c.branch,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *Target
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.target.get",
	//   "parameterOrder": [
	//     "branch",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "branch": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "branches/{branch}/targets/{resourceId}",
	//   "response": {
	//     "$ref": "Target"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.target.list":

type TargetListCall struct {
	s      *Service
	branch string
	opt_   map[string]interface{}
}

// List:
func (r *TargetService) List(branch string) *TargetListCall {
	c := &TargetListCall{s: r.s, opt_: make(map[string]interface{})}
	c.branch = branch
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *TargetListCall) MaxResults(maxResults int64) *TargetListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *TargetListCall) PageToken(pageToken string) *TargetListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TargetListCall) Fields(s ...googleapi.Field) *TargetListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TargetListCall) Do() (*TargetListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{branch}/targets")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch": c.branch,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TargetListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.target.list",
	//   "parameterOrder": [
	//     "branch"
	//   ],
	//   "parameters": {
	//     "branch": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "branches/{branch}/targets",
	//   "response": {
	//     "$ref": "TargetListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testresult.get":

type TestresultGetCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	resourceId int64
	opt_       map[string]interface{}
}

// Get:
func (r *TestresultService) Get(buildId string, target string, attemptId string, resourceId int64) *TestresultGetCall {
	c := &TestresultGetCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultGetCall) Fields(s ...googleapi.Field) *TestresultGetCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TestresultGetCall) Do() (*TestResult, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TestResult
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.testresult.get",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}",
	//   "response": {
	//     "$ref": "TestResult"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testresult.insert":

type TestresultInsertCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	testresult *TestResult
	opt_       map[string]interface{}
}

// Insert:
func (r *TestresultService) Insert(buildId string, target string, attemptId string, testresult *TestResult) *TestresultInsertCall {
	c := &TestresultInsertCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultInsertCall) Fields(s ...googleapi.Field) *TestresultInsertCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TestresultInsertCall) Do() (*TestResult, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TestResult
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.testresult.insert",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/tests",
	//   "request": {
	//     "$ref": "TestResult"
	//   },
	//   "response": {
	//     "$ref": "TestResult"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testresult.list":

type TestresultListCall struct {
	s         *Service
	buildId   string
	target    string
	attemptId string
	opt_      map[string]interface{}
}

// List:
func (r *TestresultService) List(buildId string, target string, attemptId string) *TestresultListCall {
	c := &TestresultListCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *TestresultListCall) MaxResults(maxResults int64) *TestresultListCall {
	c.opt_["maxResults"] = maxResults
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *TestresultListCall) PageToken(pageToken string) *TestresultListCall {
	c.opt_["pageToken"] = pageToken
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultListCall) Fields(s ...googleapi.Field) *TestresultListCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TestresultListCall) Do() (*TestResultListResponse, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["maxResults"]; ok {
		params.Set("maxResults", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["pageToken"]; ok {
		params.Set("pageToken", fmt.Sprintf("%v", v))
	}
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TestResultListResponse
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.testresult.list",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/tests",
	//   "response": {
	//     "$ref": "TestResultListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testresult.patch":

type TestresultPatchCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	resourceId int64
	testresult *TestResult
	opt_       map[string]interface{}
}

// Patch:
func (r *TestresultService) Patch(buildId string, target string, attemptId string, resourceId int64, testresult *TestResult) *TestresultPatchCall {
	c := &TestresultPatchCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultPatchCall) Fields(s ...googleapi.Field) *TestresultPatchCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TestresultPatchCall) Do() (*TestResult, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TestResult
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.testresult.patch",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}",
	//   "request": {
	//     "$ref": "TestResult"
	//   },
	//   "response": {
	//     "$ref": "TestResult"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testresult.update":

type TestresultUpdateCall struct {
	s          *Service
	buildId    string
	target     string
	attemptId  string
	resourceId int64
	testresult *TestResult
	opt_       map[string]interface{}
}

// Update:
func (r *TestresultService) Update(buildId string, target string, attemptId string, resourceId int64, testresult *TestResult) *TestresultUpdateCall {
	c := &TestresultUpdateCall{s: r.s, opt_: make(map[string]interface{})}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultUpdateCall) Fields(s ...googleapi.Field) *TestresultUpdateCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *TestresultUpdateCall) Do() (*TestResult, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *TestResult
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.testresult.update",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "resourceId"
	//   ],
	//   "parameters": {
	//     "attemptId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "buildId": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}",
	//   "request": {
	//     "$ref": "TestResult"
	//   },
	//   "response": {
	//     "$ref": "TestResult"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}
