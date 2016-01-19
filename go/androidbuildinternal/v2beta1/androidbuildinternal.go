// Package androidbuildinternal provides access to the .
//
// Usage example:
//
//   import "go.skia.org/infra/go/androidbuildinternal/v2beta1"
//   ...
//   androidbuildinternalService, err := androidbuildinternal.New(oauthHttpClient)
package androidbuildinternal // import "go.skia.org/infra/go/androidbuildinternal/v2beta1"

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

	context "golang.org/x/net/context"
	ctxhttp "golang.org/x/net/context/ctxhttp"
	gensupport "google.golang.org/api/gensupport"
	googleapi "google.golang.org/api/googleapi"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = gensupport.MarshalJSON
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace
var _ = context.Canceled
var _ = ctxhttp.Do

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
	s.Changesetspec = NewChangesetspecService(s)
	s.Deviceblob = NewDeviceblobService(s)
	s.Imagerequest = NewImagerequestService(s)
	s.Label = NewLabelService(s)
	s.Target = NewTargetService(s)
	s.Testartifact = NewTestartifactService(s)
	s.Testresult = NewTestresultService(s)
	s.Worknode = NewWorknodeService(s)
	s.Workplan = NewWorkplanService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Branch *BranchService

	Bughash *BughashService

	Build *BuildService

	Buildartifact *BuildartifactService

	Buildattempt *BuildattemptService

	Buildid *BuildidService

	Buildrequest *BuildrequestService

	Changesetspec *ChangesetspecService

	Deviceblob *DeviceblobService

	Imagerequest *ImagerequestService

	Label *LabelService

	Target *TargetService

	Testartifact *TestartifactService

	Testresult *TestresultService

	Worknode *WorknodeService

	Workplan *WorkplanService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
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

func NewChangesetspecService(s *Service) *ChangesetspecService {
	rs := &ChangesetspecService{s: s}
	return rs
}

type ChangesetspecService struct {
	s *Service
}

func NewDeviceblobService(s *Service) *DeviceblobService {
	rs := &DeviceblobService{s: s}
	return rs
}

type DeviceblobService struct {
	s *Service
}

func NewImagerequestService(s *Service) *ImagerequestService {
	rs := &ImagerequestService{s: s}
	return rs
}

type ImagerequestService struct {
	s *Service
}

func NewLabelService(s *Service) *LabelService {
	rs := &LabelService{s: s}
	return rs
}

type LabelService struct {
	s *Service
}

func NewTargetService(s *Service) *TargetService {
	rs := &TargetService{s: s}
	return rs
}

type TargetService struct {
	s *Service
}

func NewTestartifactService(s *Service) *TestartifactService {
	rs := &TestartifactService{s: s}
	return rs
}

type TestartifactService struct {
	s *Service
}

func NewTestresultService(s *Service) *TestresultService {
	rs := &TestresultService{s: s}
	return rs
}

type TestresultService struct {
	s *Service
}

func NewWorknodeService(s *Service) *WorknodeService {
	rs := &WorknodeService{s: s}
	return rs
}

type WorknodeService struct {
	s *Service
}

func NewWorkplanService(s *Service) *WorkplanService {
	rs := &WorkplanService{s: s}
	return rs
}

type WorkplanService struct {
	s *Service
}

type ApkSignResult struct {
	Apk string `json:"apk,omitempty"`

	ErrorMessage string `json:"errorMessage,omitempty"`

	Path string `json:"path,omitempty"`

	SignedApkArtifactName string `json:"signedApkArtifactName,omitempty"`

	Success bool `json:"success,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Apk") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ApkSignResult) MarshalJSON() ([]byte, error) {
	type noMethod ApkSignResult
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfig struct {
	BuildPrefix string `json:"buildPrefix,omitempty"`

	BuildRequest *BranchConfigBuildRequestConfig `json:"buildRequest,omitempty"`

	BuildUpdateAcl string `json:"buildUpdateAcl,omitempty"`

	DevelopmentBranch string `json:"developmentBranch,omitempty"`

	Disabled bool `json:"disabled,omitempty"`

	DisplayName string `json:"displayName,omitempty"`

	External *BranchConfigExternalBuildConfig `json:"external,omitempty"`

	Flashstation *BranchConfigFlashStationConfig `json:"flashstation,omitempty"`

	Gitbuildkicker *BranchConfigGitbuildkickerConfig `json:"gitbuildkicker,omitempty"`

	Manifest *ManifestLocation `json:"manifest,omitempty"`

	Name string `json:"name,omitempty"`

	PdkReleaseBranch bool `json:"pdkReleaseBranch,omitempty"`

	ReleaseBranch bool `json:"releaseBranch,omitempty"`

	SigningAcl string `json:"signingAcl,omitempty"`

	SubmitQueue *BranchConfigSubmitQueueBranchConfig `json:"submitQueue,omitempty"`

	Submitted *BranchConfigSubmittedBuildConfig `json:"submitted,omitempty"`

	Targets []*Target `json:"targets,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BuildPrefix") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigBuildRequestConfig struct {
	AclName string `json:"aclName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AclName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigBuildRequestConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigBuildRequestConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigExternalBuildConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Enabled") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigExternalBuildConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigExternalBuildConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigFlashStationConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	Products []string `json:"products,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Enabled") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigFlashStationConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigFlashStationConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigGitbuildkickerConfig struct {
	Notifications []string `json:"notifications,omitempty"`

	Targets []string `json:"targets,omitempty"`

	VersionInfo *BranchConfigGitbuildkickerConfigVersionBumpingSpec `json:"versionInfo,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Notifications") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigGitbuildkickerConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigGitbuildkickerConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigGitbuildkickerConfigVersionBumpingSpec struct {
	BumpDevBranch bool `json:"bumpDevBranch,omitempty"`

	File string `json:"file,omitempty"`

	PaddingWidth int64 `json:"paddingWidth,omitempty"`

	Project string `json:"project,omitempty"`

	VersionBranch string `json:"versionBranch,omitempty"`

	VersionRegex string `json:"versionRegex,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BumpDevBranch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigGitbuildkickerConfigVersionBumpingSpec) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigGitbuildkickerConfigVersionBumpingSpec
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigSubmitQueueBranchConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	TreehuggerEnabled bool `json:"treehuggerEnabled,omitempty"`

	Weight int64 `json:"weight,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Enabled") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigSubmitQueueBranchConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigSubmitQueueBranchConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchConfigSubmittedBuildConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Enabled") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchConfigSubmittedBuildConfig) MarshalJSON() ([]byte, error) {
	type noMethod BranchConfigSubmittedBuildConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BranchListResponse struct {
	Branches []*BranchConfig `json:"branches,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Branches") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BranchListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BranchListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
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

	// ForceSendFields is a list of field names (e.g. "BugId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Bug) MarshalJSON() ([]byte, error) {
	type noMethod Bug
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BugBugHashLines struct {
	Lines googleapi.Int64s `json:"lines,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Lines") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BugBugHashLines) MarshalJSON() ([]byte, error) {
	type noMethod BugBugHashLines
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BugHash struct {
	Bugs []*Bug `json:"bugs,omitempty"`

	Hash string `json:"hash,omitempty"`

	Namespace string `json:"namespace,omitempty"`

	Revision string `json:"revision,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Bugs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BugHash) MarshalJSON() ([]byte, error) {
	type noMethod BugHash
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BugHashListResponse struct {
	BugHashes []*BugHash `json:"bug_hashes,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BugHashes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BugHashListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BugHashListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type Build struct {
	AppProps []*BuildApplicationPropEntry `json:"appProps,omitempty"`

	Branch string `json:"branch,omitempty"`

	BuildAttemptStatus string `json:"buildAttemptStatus,omitempty"`

	BuildId string `json:"buildId,omitempty"`

	Changes []*Change `json:"changes,omitempty"`

	CreationTimestamp int64 `json:"creationTimestamp,omitempty,string"`

	Rank int64 `json:"rank,omitempty"`

	ReferenceReleaseCandidateName string `json:"referenceReleaseCandidateName,omitempty"`

	ReleaseCandidateName string `json:"releaseCandidateName,omitempty"`

	Revision string `json:"revision,omitempty"`

	Signed bool `json:"signed,omitempty"`

	Successful bool `json:"successful,omitempty"`

	Target *Target `json:"target,omitempty"`

	WorknodeId string `json:"worknodeId,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "AppProps") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Build) MarshalJSON() ([]byte, error) {
	type noMethod Build
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildApplicationPropEntry struct {
	Application string `json:"application,omitempty"`

	Key string `json:"key,omitempty"`

	Value string `json:"value,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Application") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildApplicationPropEntry) MarshalJSON() ([]byte, error) {
	type noMethod BuildApplicationPropEntry
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildArtifactCopyToResponse struct {
	DestinationBucket string `json:"destinationBucket,omitempty"`

	DestinationPath string `json:"destinationPath,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DestinationBucket")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildArtifactCopyToResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildArtifactCopyToResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildArtifactListResponse struct {
	Artifacts []*BuildArtifactMetadata `json:"artifacts,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Artifacts") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildArtifactListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildArtifactListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildArtifactMetadata struct {
	ContentType string `json:"contentType,omitempty"`

	Crc32 int64 `json:"crc32,omitempty"`

	CreationTime int64 `json:"creationTime,omitempty,string"`

	LastModifiedTime int64 `json:"lastModifiedTime,omitempty,string"`

	Md5 string `json:"md5,omitempty"`

	Name string `json:"name,omitempty"`

	Revision string `json:"revision,omitempty"`

	Size int64 `json:"size,omitempty,string"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ContentType") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildArtifactMetadata) MarshalJSON() ([]byte, error) {
	type noMethod BuildArtifactMetadata
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
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

	SyncEndTimestamp int64 `json:"syncEndTimestamp,omitempty,string"`

	SyncStartTimestamp int64 `json:"syncStartTimestamp,omitempty,string"`

	TimestampEnd int64 `json:"timestampEnd,omitempty,string"`

	TimestampStart int64 `json:"timestampStart,omitempty,string"`

	UpdaterFile string `json:"updaterFile,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BuildProp") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildAttempt) MarshalJSON() ([]byte, error) {
	type noMethod BuildAttempt
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildAttemptListResponse struct {
	Attempts []*BuildAttempt `json:"attempts,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Attempts") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildAttemptListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildAttemptListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildId struct {
	BuildId string `json:"buildId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildId) MarshalJSON() ([]byte, error) {
	type noMethod BuildId
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildIdListResponse struct {
	BuildIds []*BuildId `json:"buildIds,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BuildIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildIdListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildIdListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildListResponse struct {
	Builds []*Build `json:"builds,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Builds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildRequest struct {
	Branch string `json:"branch,omitempty"`

	Id int64 `json:"id,omitempty,string"`

	Requester *Email `json:"requester,omitempty"`

	Revision string `json:"revision,omitempty"`

	Rollup *BuildRequestRollupConfig `json:"rollup,omitempty"`

	Status string `json:"status,omitempty"`

	Type string `json:"type,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildRequest) MarshalJSON() ([]byte, error) {
	type noMethod BuildRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildRequestListResponse struct {
	BuildRequests []*BuildRequest `json:"build_requests,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BuildRequests") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildRequestListResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildRequestListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildRequestRollupConfig struct {
	BuildId string `json:"buildId,omitempty"`

	CutBuildId string `json:"cutBuildId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildRequestRollupConfig) MarshalJSON() ([]byte, error) {
	type noMethod BuildRequestRollupConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type BuildSignResponse struct {
	Results []*ApkSignResult `json:"results,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Results") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *BuildSignResponse) MarshalJSON() ([]byte, error) {
	type noMethod BuildSignResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
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

	Topic string `json:"topic,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Change) MarshalJSON() ([]byte, error) {
	type noMethod Change
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ChangeSetSpec struct {
	ChangeSpecIds []string `json:"changeSpecIds,omitempty"`

	ChangeSpecs []*ChangeSetSpecChangeSpec `json:"changeSpecs,omitempty"`

	Id string `json:"id,omitempty"`

	Revision string `json:"revision,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ChangeSpecIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ChangeSetSpec) MarshalJSON() ([]byte, error) {
	type noMethod ChangeSetSpec
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ChangeSetSpecChangeSpec struct {
	DummySpecString string `json:"dummySpecString,omitempty"`

	GerritChange *GerritChangeSpec `json:"gerritChange,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DummySpecString") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ChangeSetSpecChangeSpec) MarshalJSON() ([]byte, error) {
	type noMethod ChangeSetSpecChangeSpec
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ChangeSetSpecListSupersetsRequest struct {
	ChangeSpecs []*ChangeSetSpecChangeSpec `json:"changeSpecs,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ChangeSpecs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ChangeSetSpecListSupersetsRequest) MarshalJSON() ([]byte, error) {
	type noMethod ChangeSetSpecListSupersetsRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ChangeSetSpecListSupersetsResponse struct {
	ChangeSetSpecs []*ChangeSetSpec `json:"changeSetSpecs,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ChangeSetSpecs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ChangeSetSpecListSupersetsResponse) MarshalJSON() ([]byte, error) {
	type noMethod ChangeSetSpecListSupersetsResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type CommitInfo struct {
	Author *User `json:"author,omitempty"`

	CommitId string `json:"commitId,omitempty"`

	CommitMessage string `json:"commitMessage,omitempty"`

	Committer *User `json:"committer,omitempty"`

	Parents []*CommitInfo `json:"parents,omitempty"`

	Subject string `json:"subject,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Author") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *CommitInfo) MarshalJSON() ([]byte, error) {
	type noMethod CommitInfo
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type DeviceBlobCopyToResponse struct {
	DestinationBucket string `json:"destinationBucket,omitempty"`

	DestinationPath string `json:"destinationPath,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DestinationBucket")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *DeviceBlobCopyToResponse) MarshalJSON() ([]byte, error) {
	type noMethod DeviceBlobCopyToResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type DeviceBlobListResponse struct {
	Blobs []*BuildArtifactMetadata `json:"blobs,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Blobs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *DeviceBlobListResponse) MarshalJSON() ([]byte, error) {
	type noMethod DeviceBlobListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type Email struct {
	Email string `json:"email,omitempty"`

	Id int64 `json:"id,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "Email") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Email) MarshalJSON() ([]byte, error) {
	type noMethod Email
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type FetchConfiguration struct {
	Method string `json:"method,omitempty"`

	Ref string `json:"ref,omitempty"`

	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Method") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *FetchConfiguration) MarshalJSON() ([]byte, error) {
	type noMethod FetchConfiguration
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type GerritChangeSpec struct {
	ChangeNumber int64 `json:"changeNumber,omitempty,string"`

	Hostname string `json:"hostname,omitempty"`

	Patchset int64 `json:"patchset,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ChangeNumber") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *GerritChangeSpec) MarshalJSON() ([]byte, error) {
	type noMethod GerritChangeSpec
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type GitManifestLocation struct {
	Branch string `json:"branch,omitempty"`

	FilePath string `json:"filePath,omitempty"`

	Host string `json:"host,omitempty"`

	RepoPath string `json:"repoPath,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *GitManifestLocation) MarshalJSON() ([]byte, error) {
	type noMethod GitManifestLocation
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequest struct {
	Build *ImageRequestBuildInfo `json:"build,omitempty"`

	Device string `json:"device,omitempty"`

	Email string `json:"email,omitempty"`

	Id string `json:"id,omitempty"`

	Incrementals []*ImageRequestBuildInfo `json:"incrementals,omitempty"`

	Params *ImageRequestParams `json:"params,omitempty"`

	ReleaseParams *ImageRequestReleaseImageParams `json:"releaseParams,omitempty"`

	Revision string `json:"revision,omitempty"`

	Signed bool `json:"signed,omitempty"`

	Status string `json:"status,omitempty"`

	Type string `json:"type,omitempty"`

	UserdebugParams *ImageRequestUserdebugImageParams `json:"userdebugParams,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Build") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequest) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequestBuildInfo struct {
	Branch string `json:"branch,omitempty"`

	BuildId string `json:"buildId,omitempty"`

	Target string `json:"target,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequestBuildInfo) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequestBuildInfo
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequestListResponse struct {
	ImageRequests []*ImageRequest `json:"image_requests,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ImageRequests") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequestListResponse) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequestListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequestParams struct {
	ArtifactNames []string `json:"artifactNames,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ArtifactNames") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequestParams) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequestParams
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequestReleaseImageParams struct {
	IncludeFullRadio bool `json:"includeFullRadio,omitempty"`

	OemVariants []string `json:"oemVariants,omitempty"`

	SignatureCheck bool `json:"signatureCheck,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IncludeFullRadio") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequestReleaseImageParams) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequestReleaseImageParams
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ImageRequestUserdebugImageParams struct {
	OemVariants []string `json:"oemVariants,omitempty"`

	// ForceSendFields is a list of field names (e.g. "OemVariants") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ImageRequestUserdebugImageParams) MarshalJSON() ([]byte, error) {
	type noMethod ImageRequestUserdebugImageParams
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type InputEdge struct {
	NeighborId string `json:"neighborId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "NeighborId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *InputEdge) MarshalJSON() ([]byte, error) {
	type noMethod InputEdge
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type Label struct {
	Builds []*LabelLabeledBuild `json:"builds,omitempty"`

	Description string `json:"description,omitempty"`

	LastUpdatedMillis int64 `json:"lastUpdatedMillis,omitempty,string"`

	Name string `json:"name,omitempty"`

	Namespace string `json:"namespace,omitempty"`

	Revision string `json:"revision,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Builds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Label) MarshalJSON() ([]byte, error) {
	type noMethod Label
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelAddBuildsRequest struct {
	Builds []*LabelLabeledBuild `json:"builds,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Builds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelAddBuildsRequest) MarshalJSON() ([]byte, error) {
	type noMethod LabelAddBuildsRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelAddBuildsResponse struct {
	Label *Label `json:"label,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Label") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelAddBuildsResponse) MarshalJSON() ([]byte, error) {
	type noMethod LabelAddBuildsResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelCloneResponse struct {
	Label *Label `json:"label,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Label") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelCloneResponse) MarshalJSON() ([]byte, error) {
	type noMethod LabelCloneResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelLabeledBuild struct {
	BuildId string `json:"buildId,omitempty"`

	TargetName string `json:"targetName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelLabeledBuild) MarshalJSON() ([]byte, error) {
	type noMethod LabelLabeledBuild
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelListResponse struct {
	Labels []*Label `json:"labels,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Labels") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelListResponse) MarshalJSON() ([]byte, error) {
	type noMethod LabelListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelRemoveBuildsRequest struct {
	Builds []*LabelLabeledBuild `json:"builds,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Builds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelRemoveBuildsRequest) MarshalJSON() ([]byte, error) {
	type noMethod LabelRemoveBuildsRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelRemoveBuildsResponse struct {
	Label *Label `json:"label,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Label") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelRemoveBuildsResponse) MarshalJSON() ([]byte, error) {
	type noMethod LabelRemoveBuildsResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LabelResetResponse struct {
	Label *Label `json:"label,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Label") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LabelResetResponse) MarshalJSON() ([]byte, error) {
	type noMethod LabelResetResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type ManifestLocation struct {
	Git *GitManifestLocation `json:"git,omitempty"`

	Url *UrlManifestLocation `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Git") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ManifestLocation) MarshalJSON() ([]byte, error) {
	type noMethod ManifestLocation
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type PartitionSize struct {
	Limit int64 `json:"limit,omitempty,string"`

	Reserve int64 `json:"reserve,omitempty,string"`

	Size int64 `json:"size,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "Limit") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *PartitionSize) MarshalJSON() ([]byte, error) {
	type noMethod PartitionSize
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type Revision struct {
	Commit *CommitInfo `json:"commit,omitempty"`

	Fetchs []*FetchConfiguration `json:"fetchs,omitempty"`

	GitRevision string `json:"gitRevision,omitempty"`

	PatchSet int64 `json:"patchSet,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Commit") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Revision) MarshalJSON() ([]byte, error) {
	type noMethod Revision
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type Target struct {
	BuildCommands []string `json:"buildCommands,omitempty"`

	BuildPlatform string `json:"buildPlatform,omitempty"`

	JavaVersion string `json:"javaVersion,omitempty"`

	LaunchcontrolName string `json:"launchcontrolName,omitempty"`

	Name string `json:"name,omitempty"`

	Product string `json:"product,omitempty"`

	Signing *TargetSigningConfig `json:"signing,omitempty"`

	SubmitQueue *TargetSubmitQueueTargetConfig `json:"submitQueue,omitempty"`

	Target string `json:"target,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BuildCommands") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *Target) MarshalJSON() ([]byte, error) {
	type noMethod Target
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	Targets []*Target `json:"targets,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetListResponse) MarshalJSON() ([]byte, error) {
	type noMethod TargetListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetSigningConfig struct {
	Apks []*TargetSigningConfigApk `json:"apks,omitempty"`

	DefaultApks []string `json:"defaultApks,omitempty"`

	Otas []*TargetSigningConfigLooseOTA `json:"otas,omitempty"`

	PackageType string `json:"packageType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Apks") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetSigningConfig) MarshalJSON() ([]byte, error) {
	type noMethod TargetSigningConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetSigningConfigApk struct {
	AclName string `json:"aclName,omitempty"`

	Key string `json:"key,omitempty"`

	MicroApks []*TargetSigningConfigMicroApk `json:"microApks,omitempty"`

	Name string `json:"name,omitempty"`

	PackageName string `json:"packageName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AclName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetSigningConfigApk) MarshalJSON() ([]byte, error) {
	type noMethod TargetSigningConfigApk
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetSigningConfigLooseOTA struct {
	AclName string `json:"aclName,omitempty"`

	Key string `json:"key,omitempty"`

	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AclName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetSigningConfigLooseOTA) MarshalJSON() ([]byte, error) {
	type noMethod TargetSigningConfigLooseOTA
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetSigningConfigMicroApk struct {
	Key string `json:"key,omitempty"`

	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Key") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetSigningConfigMicroApk) MarshalJSON() ([]byte, error) {
	type noMethod TargetSigningConfigMicroApk
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TargetSubmitQueueTargetConfig struct {
	Enabled bool `json:"enabled,omitempty"`

	TreehuggerEnabled bool `json:"treehuggerEnabled,omitempty"`

	Weight int64 `json:"weight,omitempty"`

	Whitelists []string `json:"whitelists,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Enabled") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TargetSubmitQueueTargetConfig) MarshalJSON() ([]byte, error) {
	type noMethod TargetSubmitQueueTargetConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TestArtifactCopyToResponse struct {
	DestinationBucket string `json:"destinationBucket,omitempty"`

	DestinationPath string `json:"destinationPath,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DestinationBucket")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TestArtifactCopyToResponse) MarshalJSON() ([]byte, error) {
	type noMethod TestArtifactCopyToResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TestArtifactListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	TestArtifacts []*BuildArtifactMetadata `json:"test_artifacts,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TestArtifactListResponse) MarshalJSON() ([]byte, error) {
	type noMethod TestArtifactListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TestResult struct {
	Id int64 `json:"id,omitempty,string"`

	PostedToGerrit bool `json:"postedToGerrit,omitempty"`

	Revision string `json:"revision,omitempty"`

	Status string `json:"status,omitempty"`

	Summary string `json:"summary,omitempty"`

	TestTag string `json:"testTag,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Id") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TestResult) MarshalJSON() ([]byte, error) {
	type noMethod TestResult
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type TestResultListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	TestResults []*TestResult `json:"testResults,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *TestResultListResponse) MarshalJSON() ([]byte, error) {
	type noMethod TestResultListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type UrlManifestLocation struct {
	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Url") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *UrlManifestLocation) MarshalJSON() ([]byte, error) {
	type noMethod UrlManifestLocation
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type User struct {
	Email string `json:"email,omitempty"`

	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Email") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *User) MarshalJSON() ([]byte, error) {
	type noMethod User
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNode struct {
	ChangeSetSpecIds []string `json:"changeSetSpecIds,omitempty"`

	ContainerId string `json:"containerId,omitempty"`

	CurrentAttempt *WorkNodeAttempt `json:"currentAttempt,omitempty"`

	HeartbeatTimeMillis int64 `json:"heartbeatTimeMillis,omitempty,string"`

	Id string `json:"id,omitempty"`

	InputEdges []*InputEdge `json:"inputEdges,omitempty"`

	IsFinal bool `json:"isFinal,omitempty"`

	LastUpdatedMillis int64 `json:"lastUpdatedMillis,omitempty,string"`

	PreviousAttempts []*WorkNodeAttempt `json:"previousAttempts,omitempty"`

	RetryStatus *WorkNodeRetry `json:"retryStatus,omitempty"`

	Revision string `json:"revision,omitempty"`

	Status string `json:"status,omitempty"`

	Tag string `json:"tag,omitempty"`

	WorkExecutorType string `json:"workExecutorType,omitempty"`

	WorkOutput *WorkProduct `json:"workOutput,omitempty"`

	WorkParameters *WorkParameters `json:"workParameters,omitempty"`

	WorkerId string `json:"workerId,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ChangeSetSpecIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNode) MarshalJSON() ([]byte, error) {
	type noMethod WorkNode
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeAttempt struct {
	ProgressMessages []*WorkNodeProgressMessage `json:"progressMessages,omitempty"`

	StartTimeMillis int64 `json:"startTimeMillis,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "ProgressMessages") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeAttempt) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeAttempt
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeCompleteRequest struct {
	Status string `json:"status,omitempty"`

	WorkNode *WorkNode `json:"workNode,omitempty"`

	WorkProduct *WorkProduct `json:"workProduct,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Status") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeCompleteRequest) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeCompleteRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeCompleteResponse struct {
	WorkNode *WorkNode `json:"workNode,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "WorkNode") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeCompleteResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeCompleteResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeFailRequest struct {
	WorkNode *WorkNode `json:"workNode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "WorkNode") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeFailRequest) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeFailRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeFailResponse struct {
	WorkNode *WorkNode `json:"workNode,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "WorkNode") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeFailResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeFailResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	WorkNodes []*WorkNode `json:"workNodes,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeListResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodePopRequest struct {
	HeartbeatTimeMillis int64 `json:"heartbeatTimeMillis,omitempty,string"`

	NodeId string `json:"nodeId,omitempty"`

	PoppedStatus string `json:"poppedStatus,omitempty"`

	WorkExecutorType string `json:"workExecutorType,omitempty"`

	WorkerId string `json:"workerId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "HeartbeatTimeMillis")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodePopRequest) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodePopRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodePopResponse struct {
	InputWorkNodes []*WorkNode `json:"inputWorkNodes,omitempty"`

	WorkNode *WorkNode `json:"workNode,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "InputWorkNodes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodePopResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodePopResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeProgressMessage struct {
	DisplayMessage string `json:"displayMessage,omitempty"`

	MessageString string `json:"messageString,omitempty"`

	TimeMillis int64 `json:"timeMillis,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "DisplayMessage") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeProgressMessage) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeProgressMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkNodeRetry struct {
	MaximumRetries int64 `json:"maximumRetries,omitempty"`

	RetryCount int64 `json:"retryCount,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MaximumRetries") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkNodeRetry) MarshalJSON() ([]byte, error) {
	type noMethod WorkNodeRetry
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkParameters struct {
	AtpTestParameters *WorkParametersAtpTestParameters `json:"atpTestParameters,omitempty"`

	ChangeFinished *WorkParametersPendingChangeFinishedParameters `json:"changeFinished,omitempty"`

	SubmitQueue *WorkParametersPendingChangeBuildParameters `json:"submitQueue,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AtpTestParameters")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkParameters) MarshalJSON() ([]byte, error) {
	type noMethod WorkParameters
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkParametersAtpTestParameters struct {
	Branch string `json:"branch,omitempty"`

	Target string `json:"target,omitempty"`

	TestName string `json:"testName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkParametersAtpTestParameters) MarshalJSON() ([]byte, error) {
	type noMethod WorkParametersAtpTestParameters
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkParametersPendingChangeBuildParameters struct {
	Branch string `json:"branch,omitempty"`

	Target string `json:"target,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Branch") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkParametersPendingChangeBuildParameters) MarshalJSON() ([]byte, error) {
	type noMethod WorkParametersPendingChangeBuildParameters
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkParametersPendingChangeFinishedParameters struct {
	DisplayMessage string `json:"displayMessage,omitempty"`

	LeaderChangeSpecs []*ChangeSetSpecChangeSpec `json:"leaderChangeSpecs,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DisplayMessage") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkParametersPendingChangeFinishedParameters) MarshalJSON() ([]byte, error) {
	type noMethod WorkParametersPendingChangeFinishedParameters
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlan struct {
	Id string `json:"id,omitempty"`

	LastUpdatedMillis int64 `json:"lastUpdatedMillis,omitempty,string"`

	Revision string `json:"revision,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Id") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlan) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlan
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlanAddNodesRequest struct {
	Resource *WorkPlan `json:"resource,omitempty"`

	WorkNodes []*WorkNode `json:"workNodes,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Resource") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlanAddNodesRequest) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlanAddNodesRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlanAddNodesResponse struct {
	NewWorkNodes []*WorkNode `json:"newWorkNodes,omitempty"`

	Resource *WorkPlan `json:"resource,omitempty"`

	WorkNodes []*WorkNode `json:"workNodes,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NewWorkNodes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlanAddNodesResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlanAddNodesResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlanCreateWithNodesRequest struct {
	Template *WorkPlan `json:"template,omitempty"`

	WorkNodes []*WorkNode `json:"workNodes,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Template") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlanCreateWithNodesRequest) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlanCreateWithNodesRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlanCreateWithNodesResponse struct {
	Resource *WorkPlan `json:"resource,omitempty"`

	WorkNodes []*WorkNode `json:"workNodes,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Resource") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlanCreateWithNodesResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlanCreateWithNodesResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkPlanListResponse struct {
	NextPageToken string `json:"nextPageToken,omitempty"`

	PreviousPageToken string `json:"previousPageToken,omitempty"`

	WorkPlans []*WorkPlan `json:"workPlans,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkPlanListResponse) MarshalJSON() ([]byte, error) {
	type noMethod WorkPlanListResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkProduct struct {
	BuildOutput *WorkProductBuildOutputProduct `json:"buildOutput,omitempty"`

	DisplayMessage string `json:"displayMessage,omitempty"`

	DummyOutput *WorkProductDummyOutputProduct `json:"dummyOutput,omitempty"`

	Success bool `json:"success,omitempty"`

	TestOutput *WorkProductTestOutputProduct `json:"testOutput,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildOutput") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkProduct) MarshalJSON() ([]byte, error) {
	type noMethod WorkProduct
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkProductBuildOutputProduct struct {
	BuildId string `json:"buildId,omitempty"`

	BuildType string `json:"buildType,omitempty"`

	Target string `json:"target,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkProductBuildOutputProduct) MarshalJSON() ([]byte, error) {
	type noMethod WorkProductBuildOutputProduct
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkProductDummyOutputProduct struct {
	DummyString string `json:"dummyString,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DummyString") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkProductDummyOutputProduct) MarshalJSON() ([]byte, error) {
	type noMethod WorkProductDummyOutputProduct
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type WorkProductTestOutputProduct struct {
	BuildId string `json:"buildId,omitempty"`

	Target string `json:"target,omitempty"`

	TestResultId string `json:"testResultId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BuildId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *WorkProductTestOutputProduct) MarshalJSON() ([]byte, error) {
	type noMethod WorkProductTestOutputProduct
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

// method id "androidbuildinternal.branch.get":

type BranchGetCall struct {
	s            *Service
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BranchService) Get(resourceId string) *BranchGetCall {
	c := &BranchGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BranchGetCall) Fields(s ...googleapi.Field) *BranchGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BranchGetCall) IfNoneMatch(entityTag string) *BranchGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BranchGetCall) Context(ctx context.Context) *BranchGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BranchGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.branch.get" call.
// Exactly one of *BranchConfig or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BranchConfig.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BranchGetCall) Do(opts ...googleapi.CallOption) (*BranchConfig, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BranchConfig{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BranchService) List() *BranchListCall {
	c := &BranchListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// BuildPrefix sets the optional parameter "buildPrefix":
func (c *BranchListCall) BuildPrefix(buildPrefix string) *BranchListCall {
	c.urlParams_.Set("buildPrefix", buildPrefix)
	return c
}

// Disabled sets the optional parameter "disabled":
func (c *BranchListCall) Disabled(disabled bool) *BranchListCall {
	c.urlParams_.Set("disabled", fmt.Sprint(disabled))
	return c
}

// ExcludeIfEmptyFields sets the optional parameter
// "excludeIfEmptyFields":
//
// Possible values:
//   "buildPrefix"
//   "buildUpdateAcl"
//   "flashstation"
func (c *BranchListCall) ExcludeIfEmptyFields(excludeIfEmptyFields ...string) *BranchListCall {
	c.urlParams_.SetMulti("excludeIfEmptyFields", append([]string{}, excludeIfEmptyFields...))
	return c
}

// FlashstationEnabled sets the optional parameter
// "flashstationEnabled":
func (c *BranchListCall) FlashstationEnabled(flashstationEnabled bool) *BranchListCall {
	c.urlParams_.Set("flashstationEnabled", fmt.Sprint(flashstationEnabled))
	return c
}

// FlashstationProduct sets the optional parameter
// "flashstationProduct":
func (c *BranchListCall) FlashstationProduct(flashstationProduct string) *BranchListCall {
	c.urlParams_.Set("flashstationProduct", flashstationProduct)
	return c
}

// IsExternal sets the optional parameter "isExternal":
func (c *BranchListCall) IsExternal(isExternal bool) *BranchListCall {
	c.urlParams_.Set("isExternal", fmt.Sprint(isExternal))
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BranchListCall) MaxResults(maxResults int64) *BranchListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BranchListCall) PageToken(pageToken string) *BranchListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BranchListCall) Fields(s ...googleapi.Field) *BranchListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BranchListCall) IfNoneMatch(entityTag string) *BranchListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BranchListCall) Context(ctx context.Context) *BranchListCall {
	c.ctx_ = ctx
	return c
}

func (c *BranchListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.branch.list" call.
// Exactly one of *BranchListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BranchListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BranchListCall) Do(opts ...googleapi.CallOption) (*BranchListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BranchListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	//     "disabled": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "excludeIfEmptyFields": {
	//       "enum": [
	//         "buildPrefix",
	//         "buildUpdateAcl",
	//         "flashstation"
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
	//     "flashstationEnabled": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "flashstationProduct": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "isExternal": {
	//       "location": "query",
	//       "type": "boolean"
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
	s            *Service
	namespace    string
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BughashService) Get(namespace string, resourceId string) *BughashGetCall {
	c := &BughashGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashGetCall) Fields(s ...googleapi.Field) *BughashGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BughashGetCall) IfNoneMatch(entityTag string) *BughashGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BughashGetCall) Context(ctx context.Context) *BughashGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BughashGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.bughash.get" call.
// Exactly one of *BugHash or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *BugHash.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BughashGetCall) Do(opts ...googleapi.CallOption) (*BugHash, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BugHash{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BughashService) List() *BughashListCall {
	c := &BughashListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// BugId sets the optional parameter "bugId":
func (c *BughashListCall) BugId(bugId int64) *BughashListCall {
	c.urlParams_.Set("bugId", fmt.Sprint(bugId))
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BughashListCall) MaxResults(maxResults int64) *BughashListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// Namespace sets the optional parameter "namespace":
func (c *BughashListCall) Namespace(namespace string) *BughashListCall {
	c.urlParams_.Set("namespace", namespace)
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BughashListCall) PageToken(pageToken string) *BughashListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashListCall) Fields(s ...googleapi.Field) *BughashListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BughashListCall) IfNoneMatch(entityTag string) *BughashListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BughashListCall) Context(ctx context.Context) *BughashListCall {
	c.ctx_ = ctx
	return c
}

func (c *BughashListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.bughash.list" call.
// Exactly one of *BugHashListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BugHashListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BughashListCall) Do(opts ...googleapi.CallOption) (*BugHashListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BugHashListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Patch:
func (r *BughashService) Patch(namespace string, resourceId string, bughash *BugHash) *BughashPatchCall {
	c := &BughashPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.resourceId = resourceId
	c.bughash = bughash
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashPatchCall) Fields(s ...googleapi.Field) *BughashPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BughashPatchCall) Context(ctx context.Context) *BughashPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *BughashPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.bughash)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.bughash.patch" call.
// Exactly one of *BugHash or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *BugHash.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BughashPatchCall) Do(opts ...googleapi.CallOption) (*BugHash, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BugHash{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Update:
func (r *BughashService) Update(namespace string, resourceId string, bughash *BugHash) *BughashUpdateCall {
	c := &BughashUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.resourceId = resourceId
	c.bughash = bughash
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BughashUpdateCall) Fields(s ...googleapi.Field) *BughashUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BughashUpdateCall) Context(ctx context.Context) *BughashUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *BughashUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.bughash)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "bugHashes/{namespace}/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":  c.namespace,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.bughash.update" call.
// Exactly one of *BugHash or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *BugHash.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BughashUpdateCall) Do(opts ...googleapi.CallOption) (*BugHash, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BugHash{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	buildId      string
	target       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BuildService) Get(buildId string, target string) *BuildGetCall {
	c := &BuildGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	return c
}

// ExtraFields sets the optional parameter "extraFields":
//
// Possible values:
//   "all"
//   "changeInfo"
func (c *BuildGetCall) ExtraFields(extraFields ...string) *BuildGetCall {
	c.urlParams_.SetMulti("extraFields", append([]string{}, extraFields...))
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildGetCall) ResourceId(resourceId string) *BuildGetCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildGetCall) Fields(s ...googleapi.Field) *BuildGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildGetCall) IfNoneMatch(entityTag string) *BuildGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildGetCall) Context(ctx context.Context) *BuildGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.get" call.
// Exactly one of *Build or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Build.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BuildGetCall) Do(opts ...googleapi.CallOption) (*Build, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Build{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s          *Service
	buildType  string
	build      *Build
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Insert:
func (r *BuildService) Insert(buildType string, build *Build) *BuildInsertCall {
	c := &BuildInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.build = build
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildInsertCall) Fields(s ...googleapi.Field) *BuildInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildInsertCall) Context(ctx context.Context) *BuildInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType": c.buildType,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.insert" call.
// Exactly one of *Build or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Build.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BuildInsertCall) Do(opts ...googleapi.CallOption) (*Build, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Build{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.build.insert",
	//   "parameterOrder": [
	//     "buildType"
	//   ],
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
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}",
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
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BuildService) List() *BuildListCall {
	c := &BuildListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Branch sets the optional parameter "branch":
func (c *BuildListCall) Branch(branch string) *BuildListCall {
	c.urlParams_.Set("branch", branch)
	return c
}

// BuildAttemptStatus sets the optional parameter "buildAttemptStatus":
//
// Possible values:
//   "building"
//   "built"
//   "complete"
//   "error"
//   "pending"
//   "pendingGerritUpload"
//   "synced"
//   "syncing"
//   "testing"
func (c *BuildListCall) BuildAttemptStatus(buildAttemptStatus string) *BuildListCall {
	c.urlParams_.Set("buildAttemptStatus", buildAttemptStatus)
	return c
}

// BuildId sets the optional parameter "buildId":
func (c *BuildListCall) BuildId(buildId string) *BuildListCall {
	c.urlParams_.Set("buildId", buildId)
	return c
}

// BuildType sets the optional parameter "buildType":
//
// Possible values:
//   "external"
//   "pending"
//   "submitted"
func (c *BuildListCall) BuildType(buildType string) *BuildListCall {
	c.urlParams_.Set("buildType", buildType)
	return c
}

// EndBuildId sets the optional parameter "endBuildId":
func (c *BuildListCall) EndBuildId(endBuildId string) *BuildListCall {
	c.urlParams_.Set("endBuildId", endBuildId)
	return c
}

// ExtraFields sets the optional parameter "extraFields":
//
// Possible values:
//   "all"
//   "changeInfo"
func (c *BuildListCall) ExtraFields(extraFields ...string) *BuildListCall {
	c.urlParams_.SetMulti("extraFields", append([]string{}, extraFields...))
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildListCall) MaxResults(maxResults int64) *BuildListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildListCall) PageToken(pageToken string) *BuildListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// ReleaseCandidateName sets the optional parameter
// "releaseCandidateName":
func (c *BuildListCall) ReleaseCandidateName(releaseCandidateName string) *BuildListCall {
	c.urlParams_.Set("releaseCandidateName", releaseCandidateName)
	return c
}

// Signed sets the optional parameter "signed":
func (c *BuildListCall) Signed(signed bool) *BuildListCall {
	c.urlParams_.Set("signed", fmt.Sprint(signed))
	return c
}

// StartBuildId sets the optional parameter "startBuildId":
func (c *BuildListCall) StartBuildId(startBuildId string) *BuildListCall {
	c.urlParams_.Set("startBuildId", startBuildId)
	return c
}

// Successful sets the optional parameter "successful":
func (c *BuildListCall) Successful(successful bool) *BuildListCall {
	c.urlParams_.Set("successful", fmt.Sprint(successful))
	return c
}

// Target sets the optional parameter "target":
func (c *BuildListCall) Target(target string) *BuildListCall {
	c.urlParams_.Set("target", target)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildListCall) Fields(s ...googleapi.Field) *BuildListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildListCall) IfNoneMatch(entityTag string) *BuildListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildListCall) Context(ctx context.Context) *BuildListCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.list" call.
// Exactly one of *BuildListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildListCall) Do(opts ...googleapi.CallOption) (*BuildListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s          *Service
	buildId    string
	target     string
	build      *Build
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Patch:
func (r *BuildService) Patch(buildId string, target string, build *Build) *BuildPatchCall {
	c := &BuildPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.build = build
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildPatchCall) ResourceId(resourceId string) *BuildPatchCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildPatchCall) Fields(s ...googleapi.Field) *BuildPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildPatchCall) Context(ctx context.Context) *BuildPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.patch" call.
// Exactly one of *Build or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Build.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BuildPatchCall) Do(opts ...googleapi.CallOption) (*Build, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Build{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s          *Service
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Pop:
func (r *BuildService) Pop() *BuildPopCall {
	c := &BuildPopCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// BuildType sets the optional parameter "buildType":
//
// Possible values:
//   "external"
//   "pending"
//   "submitted"
func (c *BuildPopCall) BuildType(buildType string) *BuildPopCall {
	c.urlParams_.Set("buildType", buildType)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildPopCall) Fields(s ...googleapi.Field) *BuildPopCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildPopCall) Context(ctx context.Context) *BuildPopCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildPopCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/pop")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.pop" call.
// Exactly one of *Build or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Build.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BuildPopCall) Do(opts ...googleapi.CallOption) (*Build, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Build{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s          *Service
	buildId    string
	target     string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Sign:
func (r *BuildService) Sign(buildId string, target string) *BuildSignCall {
	c := &BuildSignCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	return c
}

// Apks sets the optional parameter "apks":
func (c *BuildSignCall) Apks(apks ...string) *BuildSignCall {
	c.urlParams_.SetMulti("apks", append([]string{}, apks...))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildSignCall) Fields(s ...googleapi.Field) *BuildSignCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildSignCall) Context(ctx context.Context) *BuildSignCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildSignCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/sign")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.sign" call.
// Exactly one of *BuildSignResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildSignResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildSignCall) Do(opts ...googleapi.CallOption) (*BuildSignResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildSignResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s          *Service
	buildId    string
	target     string
	build      *Build
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Update:
func (r *BuildService) Update(buildId string, target string, build *Build) *BuildUpdateCall {
	c := &BuildUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.build = build
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *BuildUpdateCall) ResourceId(resourceId string) *BuildUpdateCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildUpdateCall) Fields(s ...googleapi.Field) *BuildUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildUpdateCall) Context(ctx context.Context) *BuildUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.build)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.build.update" call.
// Exactly one of *Build or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Build.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *BuildUpdateCall) Do(opts ...googleapi.CallOption) (*Build, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Build{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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

// method id "androidbuildinternal.buildartifact.copyTo":

type BuildartifactCopyToCall struct {
	s            *Service
	buildId      string
	target       string
	attemptId    string
	artifactName string
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// CopyTo:
func (r *BuildartifactService) CopyTo(buildId string, target string, attemptId string, artifactName string) *BuildartifactCopyToCall {
	c := &BuildartifactCopyToCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.artifactName = artifactName
	return c
}

// DestinationBucket sets the optional parameter "destinationBucket":
func (c *BuildartifactCopyToCall) DestinationBucket(destinationBucket string) *BuildartifactCopyToCall {
	c.urlParams_.Set("destinationBucket", destinationBucket)
	return c
}

// DestinationPath sets the optional parameter "destinationPath":
func (c *BuildartifactCopyToCall) DestinationPath(destinationPath string) *BuildartifactCopyToCall {
	c.urlParams_.Set("destinationPath", destinationPath)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactCopyToCall) Fields(s ...googleapi.Field) *BuildartifactCopyToCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildartifactCopyToCall) Context(ctx context.Context) *BuildartifactCopyToCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactCopyToCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{artifactName}/copyTo")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"artifactName": c.artifactName,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildartifact.copyTo" call.
// Exactly one of *BuildArtifactCopyToResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *BuildArtifactCopyToResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildartifactCopyToCall) Do(opts ...googleapi.CallOption) (*BuildArtifactCopyToResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactCopyToResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.buildartifact.copyTo",
	//   "parameterOrder": [
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "artifactName"
	//   ],
	//   "parameters": {
	//     "artifactName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
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
	//     "destinationBucket": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "destinationPath": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{artifactName}/copyTo",
	//   "response": {
	//     "$ref": "BuildArtifactCopyToResponse"
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Delete:
func (r *BuildartifactService) Delete(buildId string, target string, attemptId string, resourceId string) *BuildartifactDeleteCall {
	c := &BuildartifactDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// DeleteObject sets the optional parameter "deleteObject":
func (c *BuildartifactDeleteCall) DeleteObject(deleteObject bool) *BuildartifactDeleteCall {
	c.urlParams_.Set("deleteObject", fmt.Sprint(deleteObject))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactDeleteCall) Fields(s ...googleapi.Field) *BuildartifactDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildartifactDeleteCall) Context(ctx context.Context) *BuildartifactDeleteCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactDeleteCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildartifact.delete" call.
func (c *BuildartifactDeleteCall) Do(opts ...googleapi.CallOption) error {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
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
	//     "deleteObject": {
	//       "default": "true",
	//       "location": "query",
	//       "type": "boolean"
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
	s            *Service
	buildId      string
	target       string
	attemptId    string
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BuildartifactService) Get(buildId string, target string, attemptId string, resourceId string) *BuildartifactGetCall {
	c := &BuildartifactGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactGetCall) Fields(s ...googleapi.Field) *BuildartifactGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildartifactGetCall) IfNoneMatch(entityTag string) *BuildartifactGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do and Download
// methods. Any pending HTTP request will be aborted if the provided
// context is canceled.
func (c *BuildartifactGetCall) Context(ctx context.Context) *BuildartifactGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Download fetches the API endpoint's "media" value, instead of the normal
// API response value. If the returned error is nil, the Response is guaranteed to
// have a 2xx status code. Callers must close the Response.Body as usual.
func (c *BuildartifactGetCall) Download(opts ...googleapi.CallOption) (*http.Response, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("media")
	if err != nil {
		return nil, err
	}
	if err := googleapi.CheckMediaResponse(res); err != nil {
		res.Body.Close()
		return nil, err
	}
	return res, nil
}

// Do executes the "androidbuildinternal.buildartifact.get" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildartifactGetCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	//   "supportsMediaDownload": true,
	//   "useMediaDownloadService": true
	// }

}

// method id "androidbuildinternal.buildartifact.list":

type BuildartifactListCall struct {
	s            *Service
	buildId      string
	target       string
	attemptId    string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BuildartifactService) List(buildId string, target string, attemptId string) *BuildartifactListCall {
	c := &BuildartifactListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildartifactListCall) MaxResults(maxResults int64) *BuildartifactListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildartifactListCall) PageToken(pageToken string) *BuildartifactListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactListCall) Fields(s ...googleapi.Field) *BuildartifactListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildartifactListCall) IfNoneMatch(entityTag string) *BuildartifactListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildartifactListCall) Context(ctx context.Context) *BuildartifactListCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildartifact.list" call.
// Exactly one of *BuildArtifactListResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *BuildArtifactListResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildartifactListCall) Do(opts ...googleapi.CallOption) (*BuildArtifactListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_            gensupport.URLParams
	ctx_                  context.Context
}

// Patch:
func (r *BuildartifactService) Patch(buildId string, target string, attemptId string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *BuildartifactPatchCall {
	c := &BuildartifactPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactPatchCall) Fields(s ...googleapi.Field) *BuildartifactPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildartifactPatchCall) Context(ctx context.Context) *BuildartifactPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildartifact.patch" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildartifactPatchCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_            gensupport.URLParams
	media_                io.Reader
	resumableBuffer_      *gensupport.ResumableBuffer
	mediaType_            string
	mediaSize_            int64 // mediaSize, if known.  Used only for calls to progressUpdater_.
	progressUpdater_      googleapi.ProgressUpdater
	ctx_                  context.Context
}

// Update:
func (r *BuildartifactService) Update(buildId string, target string, attemptId string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *BuildartifactUpdateCall {
	c := &BuildartifactUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Media specifies the media to upload in a single chunk. At most one of
// Media and ResumableMedia may be set.
func (c *BuildartifactUpdateCall) Media(r io.Reader, options ...googleapi.MediaOption) *BuildartifactUpdateCall {
	opts := googleapi.ProcessMediaOptions(options)
	chunkSize := opts.ChunkSize
	r, c.mediaType_ = gensupport.DetermineContentType(r, opts.ContentType)
	c.media_, c.resumableBuffer_ = gensupport.PrepareUpload(r, chunkSize)
	return c
}

// ResumableMedia specifies the media to upload in chunks and can be
// canceled with ctx. ResumableMedia is deprecated in favour of Media.
// At most one of Media and ResumableMedia may be set. mediaType
// identifies the MIME media type of the upload, such as "image/png". If
// mediaType is "", it will be auto-detected. The provided ctx will
// supersede any context previously provided to the Context method.
func (c *BuildartifactUpdateCall) ResumableMedia(ctx context.Context, r io.ReaderAt, size int64, mediaType string) *BuildartifactUpdateCall {
	c.ctx_ = ctx
	rdr := gensupport.ReaderAtToReader(r, size)
	rdr, c.mediaType_ = gensupport.DetermineContentType(rdr, mediaType)
	c.resumableBuffer_ = gensupport.NewResumableBuffer(rdr, googleapi.DefaultUploadChunkSize)
	c.media_ = nil
	c.mediaSize_ = size
	return c
}

// ProgressUpdater provides a callback function that will be called
// after every chunk. It should be a low-latency function in order to
// not slow down the upload operation. This should only be called when
// using ResumableMedia (as opposed to Media).
func (c *BuildartifactUpdateCall) ProgressUpdater(pu googleapi.ProgressUpdater) *BuildartifactUpdateCall {
	c.progressUpdater_ = pu
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildartifactUpdateCall) Fields(s ...googleapi.Field) *BuildartifactUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
// This context will supersede any context previously provided to the
// ResumableMedia method.
func (c *BuildartifactUpdateCall) Context(ctx context.Context) *BuildartifactUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildartifactUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/artifacts/{resourceId}")
	if c.media_ != nil || c.resumableBuffer_ != nil {
		urls = strings.Replace(urls, "https://www.googleapis.com/", "https://www.googleapis.com/upload/", 1)
		protocol := "multipart"
		if c.resumableBuffer_ != nil {
			protocol = "resumable"
		}
		c.urlParams_.Set("uploadType", protocol)
	}
	urls += "?" + c.urlParams_.Encode()
	if c.media_ != nil {
		var combined io.ReadCloser
		combined, ctype = gensupport.CombineBodyMedia(body, ctype, c.media_, c.mediaType_)
		defer combined.Close()
		body = combined
	}
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": c.resourceId,
	})
	if c.resumableBuffer_ != nil {
		req.Header.Set("X-Upload-Content-Type", c.mediaType_)
	}
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildartifact.update" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildartifactUpdateCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	if c.resumableBuffer_ != nil {
		loc := res.Header.Get("Location")
		rx := &gensupport.ResumableUpload{
			Client:    c.s.client,
			UserAgent: c.s.userAgent(),
			URI:       loc,
			Media:     c.resumableBuffer_,
			MediaType: c.mediaType_,
			Callback: func(curr int64) {
				if c.progressUpdater_ != nil {
					c.progressUpdater_(curr, c.mediaSize_)
				}
			},
		}
		ctx := c.ctx_
		if ctx == nil {
			ctx = context.TODO()
		}
		res, err = rx.Upload(ctx)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	buildId      string
	target       string
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BuildattemptService) Get(buildId string, target string, resourceId string) *BuildattemptGetCall {
	c := &BuildattemptGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.resourceId = resourceId
	return c
}

// ExtraFields sets the optional parameter "extraFields":
//
// Possible values:
//   "all"
//   "buildProp"
//   "repoConfig"
func (c *BuildattemptGetCall) ExtraFields(extraFields ...string) *BuildattemptGetCall {
	c.urlParams_.SetMulti("extraFields", append([]string{}, extraFields...))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptGetCall) Fields(s ...googleapi.Field) *BuildattemptGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildattemptGetCall) IfNoneMatch(entityTag string) *BuildattemptGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildattemptGetCall) Context(ctx context.Context) *BuildattemptGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildattemptGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildattempt.get" call.
// Exactly one of *BuildAttempt or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildAttempt.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildattemptGetCall) Do(opts ...googleapi.CallOption) (*BuildAttempt, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildAttempt{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Insert:
func (r *BuildattemptService) Insert(buildId string, target string, buildattempt *BuildAttempt) *BuildattemptInsertCall {
	c := &BuildattemptInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.buildattempt = buildattempt
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptInsertCall) Fields(s ...googleapi.Field) *BuildattemptInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildattemptInsertCall) Context(ctx context.Context) *BuildattemptInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildattemptInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildattempt.insert" call.
// Exactly one of *BuildAttempt or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildAttempt.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildattemptInsertCall) Do(opts ...googleapi.CallOption) (*BuildAttempt, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildAttempt{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	buildId      string
	target       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BuildattemptService) List(buildId string, target string) *BuildattemptListCall {
	c := &BuildattemptListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	return c
}

// ExtraFields sets the optional parameter "extraFields":
//
// Possible values:
//   "all"
//   "buildProp"
//   "repoConfig"
func (c *BuildattemptListCall) ExtraFields(extraFields ...string) *BuildattemptListCall {
	c.urlParams_.SetMulti("extraFields", append([]string{}, extraFields...))
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildattemptListCall) MaxResults(maxResults int64) *BuildattemptListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildattemptListCall) PageToken(pageToken string) *BuildattemptListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptListCall) Fields(s ...googleapi.Field) *BuildattemptListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildattemptListCall) IfNoneMatch(entityTag string) *BuildattemptListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildattemptListCall) Context(ctx context.Context) *BuildattemptListCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildattemptListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId": c.buildId,
		"target":  c.target,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildattempt.list" call.
// Exactly one of *BuildAttemptListResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *BuildAttemptListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildattemptListCall) Do(opts ...googleapi.CallOption) (*BuildAttemptListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildAttemptListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	buildattempt *BuildAttempt
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Patch:
func (r *BuildattemptService) Patch(target string, resourceId string, buildId string, buildattempt *BuildAttempt) *BuildattemptPatchCall {
	c := &BuildattemptPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.target = target
	c.resourceId = resourceId
	c.urlParams_.Set("buildId", buildId)
	c.buildattempt = buildattempt
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptPatchCall) Fields(s ...googleapi.Field) *BuildattemptPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildattemptPatchCall) Context(ctx context.Context) *BuildattemptPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildattemptPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{target}/attempts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildattempt.patch" call.
// Exactly one of *BuildAttempt or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildAttempt.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildattemptPatchCall) Do(opts ...googleapi.CallOption) (*BuildAttempt, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildAttempt{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Update:
func (r *BuildattemptService) Update(target string, resourceId string, buildattempt *BuildAttempt) *BuildattemptUpdateCall {
	c := &BuildattemptUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.target = target
	c.resourceId = resourceId
	c.buildattempt = buildattempt
	return c
}

// BuildId sets the optional parameter "buildId":
func (c *BuildattemptUpdateCall) BuildId(buildId string) *BuildattemptUpdateCall {
	c.urlParams_.Set("buildId", buildId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildattemptUpdateCall) Fields(s ...googleapi.Field) *BuildattemptUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildattemptUpdateCall) Context(ctx context.Context) *BuildattemptUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildattemptUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildattempt)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{target}/attempts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"target":     c.target,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildattempt.update" call.
// Exactly one of *BuildAttempt or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildAttempt.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildattemptUpdateCall) Do(opts ...googleapi.CallOption) (*BuildAttempt, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildAttempt{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	branch       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BuildidService) List(branch string) *BuildidListCall {
	c := &BuildidListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.branch = branch
	return c
}

// BuildType sets the optional parameter "buildType":
//
// Possible values:
//   "external"
//   "pending"
//   "submitted"
func (c *BuildidListCall) BuildType(buildType string) *BuildidListCall {
	c.urlParams_.Set("buildType", buildType)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildidListCall) MaxResults(maxResults int64) *BuildidListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildidListCall) PageToken(pageToken string) *BuildidListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildidListCall) Fields(s ...googleapi.Field) *BuildidListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildidListCall) IfNoneMatch(entityTag string) *BuildidListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildidListCall) Context(ctx context.Context) *BuildidListCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildidListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildIds/{branch}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch": c.branch,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildid.list" call.
// Exactly one of *BuildIdListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildIdListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildidListCall) Do(opts ...googleapi.CallOption) (*BuildIdListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildIdListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	//   "path": "buildIds/{branch}",
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
	s            *Service
	resourceId   int64
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *BuildrequestService) Get(resourceId int64) *BuildrequestGetCall {
	c := &BuildrequestGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestGetCall) Fields(s ...googleapi.Field) *BuildrequestGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildrequestGetCall) IfNoneMatch(entityTag string) *BuildrequestGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildrequestGetCall) Context(ctx context.Context) *BuildrequestGetCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildrequestGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildrequest.get" call.
// Exactly one of *BuildRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildrequestGetCall) Do(opts ...googleapi.CallOption) (*BuildRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Insert:
func (r *BuildrequestService) Insert(buildrequest *BuildRequest) *BuildrequestInsertCall {
	c := &BuildrequestInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestInsertCall) Fields(s ...googleapi.Field) *BuildrequestInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildrequestInsertCall) Context(ctx context.Context) *BuildrequestInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildrequestInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildrequest.insert" call.
// Exactly one of *BuildRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildrequestInsertCall) Do(opts ...googleapi.CallOption) (*BuildRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *BuildrequestService) List() *BuildrequestListCall {
	c := &BuildrequestListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Branch sets the optional parameter "branch":
func (c *BuildrequestListCall) Branch(branch string) *BuildrequestListCall {
	c.urlParams_.Set("branch", branch)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *BuildrequestListCall) MaxResults(maxResults int64) *BuildrequestListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *BuildrequestListCall) PageToken(pageToken string) *BuildrequestListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Status sets the optional parameter "status":
//
// Possible values:
//   "complete"
//   "failed"
//   "inProgress"
//   "pending"
func (c *BuildrequestListCall) Status(status string) *BuildrequestListCall {
	c.urlParams_.Set("status", status)
	return c
}

// Type sets the optional parameter "type":
//
// Possible values:
//   "rollup"
func (c *BuildrequestListCall) Type(type_ string) *BuildrequestListCall {
	c.urlParams_.Set("type", type_)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestListCall) Fields(s ...googleapi.Field) *BuildrequestListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *BuildrequestListCall) IfNoneMatch(entityTag string) *BuildrequestListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildrequestListCall) Context(ctx context.Context) *BuildrequestListCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildrequestListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildrequest.list" call.
// Exactly one of *BuildRequestListResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *BuildRequestListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *BuildrequestListCall) Do(opts ...googleapi.CallOption) (*BuildRequestListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildRequestListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Patch:
func (r *BuildrequestService) Patch(resourceId int64, buildrequest *BuildRequest) *BuildrequestPatchCall {
	c := &BuildrequestPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestPatchCall) Fields(s ...googleapi.Field) *BuildrequestPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildrequestPatchCall) Context(ctx context.Context) *BuildrequestPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildrequestPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildrequest.patch" call.
// Exactly one of *BuildRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildrequestPatchCall) Do(opts ...googleapi.CallOption) (*BuildRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Update:
func (r *BuildrequestService) Update(resourceId int64, buildrequest *BuildRequest) *BuildrequestUpdateCall {
	c := &BuildrequestUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.buildrequest = buildrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *BuildrequestUpdateCall) Fields(s ...googleapi.Field) *BuildrequestUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *BuildrequestUpdateCall) Context(ctx context.Context) *BuildrequestUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *BuildrequestUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "buildRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.buildrequest.update" call.
// Exactly one of *BuildRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *BuildRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *BuildrequestUpdateCall) Do(opts ...googleapi.CallOption) (*BuildRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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

// method id "androidbuildinternal.changesetspec.get":

type ChangesetspecGetCall struct {
	s            *Service
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *ChangesetspecService) Get(resourceId string) *ChangesetspecGetCall {
	c := &ChangesetspecGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ChangesetspecGetCall) Fields(s ...googleapi.Field) *ChangesetspecGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ChangesetspecGetCall) IfNoneMatch(entityTag string) *ChangesetspecGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ChangesetspecGetCall) Context(ctx context.Context) *ChangesetspecGetCall {
	c.ctx_ = ctx
	return c
}

func (c *ChangesetspecGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "changeSetSpecs/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.changesetspec.get" call.
// Exactly one of *ChangeSetSpec or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ChangeSetSpec.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ChangesetspecGetCall) Do(opts ...googleapi.CallOption) (*ChangeSetSpec, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ChangeSetSpec{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.changesetspec.get",
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
	//   "path": "changeSetSpecs/{resourceId}",
	//   "response": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.changesetspec.insert":

type ChangesetspecInsertCall struct {
	s             *Service
	changesetspec *ChangeSetSpec
	urlParams_    gensupport.URLParams
	ctx_          context.Context
}

// Insert:
func (r *ChangesetspecService) Insert(changesetspec *ChangeSetSpec) *ChangesetspecInsertCall {
	c := &ChangesetspecInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.changesetspec = changesetspec
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ChangesetspecInsertCall) Fields(s ...googleapi.Field) *ChangesetspecInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ChangesetspecInsertCall) Context(ctx context.Context) *ChangesetspecInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *ChangesetspecInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.changesetspec)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "changeSetSpecs")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.changesetspec.insert" call.
// Exactly one of *ChangeSetSpec or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ChangeSetSpec.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ChangesetspecInsertCall) Do(opts ...googleapi.CallOption) (*ChangeSetSpec, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ChangeSetSpec{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.changesetspec.insert",
	//   "path": "changeSetSpecs",
	//   "request": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "response": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.changesetspec.listsupersets":

type ChangesetspecListsupersetsCall struct {
	s                                 *Service
	changesetspeclistsupersetsrequest *ChangeSetSpecListSupersetsRequest
	urlParams_                        gensupport.URLParams
	ctx_                              context.Context
}

// Listsupersets:
func (r *ChangesetspecService) Listsupersets(changesetspeclistsupersetsrequest *ChangeSetSpecListSupersetsRequest) *ChangesetspecListsupersetsCall {
	c := &ChangesetspecListsupersetsCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.changesetspeclistsupersetsrequest = changesetspeclistsupersetsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ChangesetspecListsupersetsCall) Fields(s ...googleapi.Field) *ChangesetspecListsupersetsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ChangesetspecListsupersetsCall) Context(ctx context.Context) *ChangesetspecListsupersetsCall {
	c.ctx_ = ctx
	return c
}

func (c *ChangesetspecListsupersetsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.changesetspeclistsupersetsrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "changeSetSpecs/listSupersets")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.changesetspec.listsupersets" call.
// Exactly one of *ChangeSetSpecListSupersetsResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *ChangeSetSpecListSupersetsResponse.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ChangesetspecListsupersetsCall) Do(opts ...googleapi.CallOption) (*ChangeSetSpecListSupersetsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ChangeSetSpecListSupersetsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.changesetspec.listsupersets",
	//   "path": "changeSetSpecs/listSupersets",
	//   "request": {
	//     "$ref": "ChangeSetSpecListSupersetsRequest"
	//   },
	//   "response": {
	//     "$ref": "ChangeSetSpecListSupersetsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.changesetspec.patch":

type ChangesetspecPatchCall struct {
	s             *Service
	resourceId    string
	changesetspec *ChangeSetSpec
	urlParams_    gensupport.URLParams
	ctx_          context.Context
}

// Patch:
func (r *ChangesetspecService) Patch(resourceId string, changesetspec *ChangeSetSpec) *ChangesetspecPatchCall {
	c := &ChangesetspecPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.changesetspec = changesetspec
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ChangesetspecPatchCall) Fields(s ...googleapi.Field) *ChangesetspecPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ChangesetspecPatchCall) Context(ctx context.Context) *ChangesetspecPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *ChangesetspecPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.changesetspec)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "changeSetSpecs/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.changesetspec.patch" call.
// Exactly one of *ChangeSetSpec or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ChangeSetSpec.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ChangesetspecPatchCall) Do(opts ...googleapi.CallOption) (*ChangeSetSpec, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ChangeSetSpec{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.changesetspec.patch",
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
	//   "path": "changeSetSpecs/{resourceId}",
	//   "request": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "response": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.changesetspec.update":

type ChangesetspecUpdateCall struct {
	s             *Service
	resourceId    string
	changesetspec *ChangeSetSpec
	urlParams_    gensupport.URLParams
	ctx_          context.Context
}

// Update:
func (r *ChangesetspecService) Update(resourceId string, changesetspec *ChangeSetSpec) *ChangesetspecUpdateCall {
	c := &ChangesetspecUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.changesetspec = changesetspec
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ChangesetspecUpdateCall) Fields(s ...googleapi.Field) *ChangesetspecUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ChangesetspecUpdateCall) Context(ctx context.Context) *ChangesetspecUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *ChangesetspecUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.changesetspec)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "changeSetSpecs/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.changesetspec.update" call.
// Exactly one of *ChangeSetSpec or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ChangeSetSpec.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ChangesetspecUpdateCall) Do(opts ...googleapi.CallOption) (*ChangeSetSpec, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ChangeSetSpec{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.changesetspec.update",
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
	//   "path": "changeSetSpecs/{resourceId}",
	//   "request": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "response": {
	//     "$ref": "ChangeSetSpec"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.deviceblob.copyTo":

type DeviceblobCopyToCall struct {
	s          *Service
	deviceName string
	binaryType string
	version    string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// CopyTo:
func (r *DeviceblobService) CopyTo(deviceName string, binaryType string, version string) *DeviceblobCopyToCall {
	c := &DeviceblobCopyToCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.version = version
	return c
}

// DestinationBucket sets the optional parameter "destinationBucket":
func (c *DeviceblobCopyToCall) DestinationBucket(destinationBucket string) *DeviceblobCopyToCall {
	c.urlParams_.Set("destinationBucket", destinationBucket)
	return c
}

// DestinationPath sets the optional parameter "destinationPath":
func (c *DeviceblobCopyToCall) DestinationPath(destinationPath string) *DeviceblobCopyToCall {
	c.urlParams_.Set("destinationPath", destinationPath)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobCopyToCall) Fields(s ...googleapi.Field) *DeviceblobCopyToCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *DeviceblobCopyToCall) Context(ctx context.Context) *DeviceblobCopyToCall {
	c.ctx_ = ctx
	return c
}

func (c *DeviceblobCopyToCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{version}/copyTo")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"version":    c.version,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.deviceblob.copyTo" call.
// Exactly one of *DeviceBlobCopyToResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *DeviceBlobCopyToResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *DeviceblobCopyToCall) Do(opts ...googleapi.CallOption) (*DeviceBlobCopyToResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &DeviceBlobCopyToResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.deviceblob.copyTo",
	//   "parameterOrder": [
	//     "deviceName",
	//     "binaryType",
	//     "version"
	//   ],
	//   "parameters": {
	//     "binaryType": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "destinationBucket": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "destinationPath": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "deviceName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "version": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "deviceBlobs/{deviceName}/{binaryType}/{version}/copyTo",
	//   "response": {
	//     "$ref": "DeviceBlobCopyToResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.deviceblob.get":

type DeviceblobGetCall struct {
	s            *Service
	deviceName   string
	binaryType   string
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *DeviceblobService) Get(deviceName string, binaryType string, resourceId string) *DeviceblobGetCall {
	c := &DeviceblobGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobGetCall) Fields(s ...googleapi.Field) *DeviceblobGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *DeviceblobGetCall) IfNoneMatch(entityTag string) *DeviceblobGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do and Download
// methods. Any pending HTTP request will be aborted if the provided
// context is canceled.
func (c *DeviceblobGetCall) Context(ctx context.Context) *DeviceblobGetCall {
	c.ctx_ = ctx
	return c
}

func (c *DeviceblobGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Download fetches the API endpoint's "media" value, instead of the normal
// API response value. If the returned error is nil, the Response is guaranteed to
// have a 2xx status code. Callers must close the Response.Body as usual.
func (c *DeviceblobGetCall) Download(opts ...googleapi.CallOption) (*http.Response, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("media")
	if err != nil {
		return nil, err
	}
	if err := googleapi.CheckMediaResponse(res); err != nil {
		res.Body.Close()
		return nil, err
	}
	return res, nil
}

// Do executes the "androidbuildinternal.deviceblob.get" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *DeviceblobGetCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	//   "supportsMediaDownload": true,
	//   "useMediaDownloadService": true
	// }

}

// method id "androidbuildinternal.deviceblob.list":

type DeviceblobListCall struct {
	s            *Service
	deviceName   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *DeviceblobService) List(deviceName string) *DeviceblobListCall {
	c := &DeviceblobListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.deviceName = deviceName
	return c
}

// BinaryType sets the optional parameter "binaryType":
func (c *DeviceblobListCall) BinaryType(binaryType string) *DeviceblobListCall {
	c.urlParams_.Set("binaryType", binaryType)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *DeviceblobListCall) MaxResults(maxResults int64) *DeviceblobListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *DeviceblobListCall) PageToken(pageToken string) *DeviceblobListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Version sets the optional parameter "version":
func (c *DeviceblobListCall) Version(version string) *DeviceblobListCall {
	c.urlParams_.Set("version", version)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobListCall) Fields(s ...googleapi.Field) *DeviceblobListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *DeviceblobListCall) IfNoneMatch(entityTag string) *DeviceblobListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *DeviceblobListCall) Context(ctx context.Context) *DeviceblobListCall {
	c.ctx_ = ctx
	return c
}

func (c *DeviceblobListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.deviceblob.list" call.
// Exactly one of *DeviceBlobListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *DeviceBlobListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *DeviceblobListCall) Do(opts ...googleapi.CallOption) (*DeviceBlobListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &DeviceBlobListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_            gensupport.URLParams
	ctx_                  context.Context
}

// Patch:
func (r *DeviceblobService) Patch(deviceName string, binaryType string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *DeviceblobPatchCall {
	c := &DeviceblobPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobPatchCall) Fields(s ...googleapi.Field) *DeviceblobPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *DeviceblobPatchCall) Context(ctx context.Context) *DeviceblobPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *DeviceblobPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.deviceblob.patch" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *DeviceblobPatchCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_            gensupport.URLParams
	media_                io.Reader
	resumableBuffer_      *gensupport.ResumableBuffer
	mediaType_            string
	mediaSize_            int64 // mediaSize, if known.  Used only for calls to progressUpdater_.
	progressUpdater_      googleapi.ProgressUpdater
	ctx_                  context.Context
}

// Update:
func (r *DeviceblobService) Update(deviceName string, binaryType string, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *DeviceblobUpdateCall {
	c := &DeviceblobUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.deviceName = deviceName
	c.binaryType = binaryType
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Media specifies the media to upload in a single chunk. At most one of
// Media and ResumableMedia may be set.
func (c *DeviceblobUpdateCall) Media(r io.Reader, options ...googleapi.MediaOption) *DeviceblobUpdateCall {
	opts := googleapi.ProcessMediaOptions(options)
	chunkSize := opts.ChunkSize
	r, c.mediaType_ = gensupport.DetermineContentType(r, opts.ContentType)
	c.media_, c.resumableBuffer_ = gensupport.PrepareUpload(r, chunkSize)
	return c
}

// ResumableMedia specifies the media to upload in chunks and can be
// canceled with ctx. ResumableMedia is deprecated in favour of Media.
// At most one of Media and ResumableMedia may be set. mediaType
// identifies the MIME media type of the upload, such as "image/png". If
// mediaType is "", it will be auto-detected. The provided ctx will
// supersede any context previously provided to the Context method.
func (c *DeviceblobUpdateCall) ResumableMedia(ctx context.Context, r io.ReaderAt, size int64, mediaType string) *DeviceblobUpdateCall {
	c.ctx_ = ctx
	rdr := gensupport.ReaderAtToReader(r, size)
	rdr, c.mediaType_ = gensupport.DetermineContentType(rdr, mediaType)
	c.resumableBuffer_ = gensupport.NewResumableBuffer(rdr, googleapi.DefaultUploadChunkSize)
	c.media_ = nil
	c.mediaSize_ = size
	return c
}

// ProgressUpdater provides a callback function that will be called
// after every chunk. It should be a low-latency function in order to
// not slow down the upload operation. This should only be called when
// using ResumableMedia (as opposed to Media).
func (c *DeviceblobUpdateCall) ProgressUpdater(pu googleapi.ProgressUpdater) *DeviceblobUpdateCall {
	c.progressUpdater_ = pu
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *DeviceblobUpdateCall) Fields(s ...googleapi.Field) *DeviceblobUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
// This context will supersede any context previously provided to the
// ResumableMedia method.
func (c *DeviceblobUpdateCall) Context(ctx context.Context) *DeviceblobUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *DeviceblobUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "deviceBlobs/{deviceName}/{binaryType}/{resourceId}")
	if c.media_ != nil || c.resumableBuffer_ != nil {
		urls = strings.Replace(urls, "https://www.googleapis.com/", "https://www.googleapis.com/upload/", 1)
		protocol := "multipart"
		if c.resumableBuffer_ != nil {
			protocol = "resumable"
		}
		c.urlParams_.Set("uploadType", protocol)
	}
	urls += "?" + c.urlParams_.Encode()
	if c.media_ != nil {
		var combined io.ReadCloser
		combined, ctype = gensupport.CombineBodyMedia(body, ctype, c.media_, c.mediaType_)
		defer combined.Close()
		body = combined
	}
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"deviceName": c.deviceName,
		"binaryType": c.binaryType,
		"resourceId": c.resourceId,
	})
	if c.resumableBuffer_ != nil {
		req.Header.Set("X-Upload-Content-Type", c.mediaType_)
	}
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.deviceblob.update" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *DeviceblobUpdateCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	if c.resumableBuffer_ != nil {
		loc := res.Header.Get("Location")
		rx := &gensupport.ResumableUpload{
			Client:    c.s.client,
			UserAgent: c.s.userAgent(),
			URI:       loc,
			Media:     c.resumableBuffer_,
			MediaType: c.mediaType_,
			Callback: func(curr int64) {
				if c.progressUpdater_ != nil {
					c.progressUpdater_(curr, c.mediaSize_)
				}
			},
		}
		ctx := c.ctx_
		if ctx == nil {
			ctx = context.TODO()
		}
		res, err = rx.Upload(ctx)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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

// method id "androidbuildinternal.imagerequest.get":

type ImagerequestGetCall struct {
	s            *Service
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *ImagerequestService) Get(resourceId string) *ImagerequestGetCall {
	c := &ImagerequestGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagerequestGetCall) Fields(s ...googleapi.Field) *ImagerequestGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ImagerequestGetCall) IfNoneMatch(entityTag string) *ImagerequestGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagerequestGetCall) Context(ctx context.Context) *ImagerequestGetCall {
	c.ctx_ = ctx
	return c
}

func (c *ImagerequestGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "imageRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.imagerequest.get" call.
// Exactly one of *ImageRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ImageRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ImagerequestGetCall) Do(opts ...googleapi.CallOption) (*ImageRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ImageRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.imagerequest.get",
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
	//   "path": "imageRequests/{resourceId}",
	//   "response": {
	//     "$ref": "ImageRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.imagerequest.insert":

type ImagerequestInsertCall struct {
	s            *Service
	imagerequest *ImageRequest
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Insert:
func (r *ImagerequestService) Insert(imagerequest *ImageRequest) *ImagerequestInsertCall {
	c := &ImagerequestInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.imagerequest = imagerequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagerequestInsertCall) Fields(s ...googleapi.Field) *ImagerequestInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagerequestInsertCall) Context(ctx context.Context) *ImagerequestInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *ImagerequestInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.imagerequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "imageRequests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.imagerequest.insert" call.
// Exactly one of *ImageRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ImageRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ImagerequestInsertCall) Do(opts ...googleapi.CallOption) (*ImageRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ImageRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.imagerequest.insert",
	//   "path": "imageRequests",
	//   "request": {
	//     "$ref": "ImageRequest"
	//   },
	//   "response": {
	//     "$ref": "ImageRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.imagerequest.list":

type ImagerequestListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *ImagerequestService) List() *ImagerequestListCall {
	c := &ImagerequestListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Device sets the optional parameter "device":
func (c *ImagerequestListCall) Device(device string) *ImagerequestListCall {
	c.urlParams_.Set("device", device)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *ImagerequestListCall) MaxResults(maxResults int64) *ImagerequestListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *ImagerequestListCall) PageToken(pageToken string) *ImagerequestListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Status sets the optional parameter "status":
//
// Possible values:
//   "complete"
//   "failed"
//   "inProgress"
//   "pending"
func (c *ImagerequestListCall) Status(status string) *ImagerequestListCall {
	c.urlParams_.Set("status", status)
	return c
}

// Type sets the optional parameter "type":
//
// Possible values:
//   "gms"
//   "looseOta"
//   "release"
//   "userdebug"
func (c *ImagerequestListCall) Type(type_ string) *ImagerequestListCall {
	c.urlParams_.Set("type", type_)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagerequestListCall) Fields(s ...googleapi.Field) *ImagerequestListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ImagerequestListCall) IfNoneMatch(entityTag string) *ImagerequestListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagerequestListCall) Context(ctx context.Context) *ImagerequestListCall {
	c.ctx_ = ctx
	return c
}

func (c *ImagerequestListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "imageRequests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.imagerequest.list" call.
// Exactly one of *ImageRequestListResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *ImageRequestListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ImagerequestListCall) Do(opts ...googleapi.CallOption) (*ImageRequestListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ImageRequestListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.imagerequest.list",
	//   "parameters": {
	//     "device": {
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
	//         "gms",
	//         "looseOta",
	//         "release",
	//         "userdebug"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "imageRequests",
	//   "response": {
	//     "$ref": "ImageRequestListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.imagerequest.patch":

type ImagerequestPatchCall struct {
	s            *Service
	resourceId   string
	imagerequest *ImageRequest
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Patch:
func (r *ImagerequestService) Patch(resourceId string, imagerequest *ImageRequest) *ImagerequestPatchCall {
	c := &ImagerequestPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.imagerequest = imagerequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagerequestPatchCall) Fields(s ...googleapi.Field) *ImagerequestPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagerequestPatchCall) Context(ctx context.Context) *ImagerequestPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *ImagerequestPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.imagerequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "imageRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.imagerequest.patch" call.
// Exactly one of *ImageRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ImageRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ImagerequestPatchCall) Do(opts ...googleapi.CallOption) (*ImageRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ImageRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.imagerequest.patch",
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
	//   "path": "imageRequests/{resourceId}",
	//   "request": {
	//     "$ref": "ImageRequest"
	//   },
	//   "response": {
	//     "$ref": "ImageRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.imagerequest.update":

type ImagerequestUpdateCall struct {
	s            *Service
	resourceId   string
	imagerequest *ImageRequest
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Update:
func (r *ImagerequestService) Update(resourceId string, imagerequest *ImageRequest) *ImagerequestUpdateCall {
	c := &ImagerequestUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.imagerequest = imagerequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagerequestUpdateCall) Fields(s ...googleapi.Field) *ImagerequestUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagerequestUpdateCall) Context(ctx context.Context) *ImagerequestUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *ImagerequestUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.imagerequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "imageRequests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.imagerequest.update" call.
// Exactly one of *ImageRequest or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *ImageRequest.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ImagerequestUpdateCall) Do(opts ...googleapi.CallOption) (*ImageRequest, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ImageRequest{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.imagerequest.update",
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
	//   "path": "imageRequests/{resourceId}",
	//   "request": {
	//     "$ref": "ImageRequest"
	//   },
	//   "response": {
	//     "$ref": "ImageRequest"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.addbuilds":

type LabelAddbuildsCall struct {
	s                     *Service
	namespace             string
	name                  string
	labeladdbuildsrequest *LabelAddBuildsRequest
	urlParams_            gensupport.URLParams
	ctx_                  context.Context
}

// Addbuilds:
func (r *LabelService) Addbuilds(namespace string, name string, labeladdbuildsrequest *LabelAddBuildsRequest) *LabelAddbuildsCall {
	c := &LabelAddbuildsCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	c.labeladdbuildsrequest = labeladdbuildsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelAddbuildsCall) Fields(s ...googleapi.Field) *LabelAddbuildsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelAddbuildsCall) Context(ctx context.Context) *LabelAddbuildsCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelAddbuildsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.labeladdbuildsrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}/addBuilds")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
		"name":      c.name,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.addbuilds" call.
// Exactly one of *LabelAddBuildsResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *LabelAddBuildsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *LabelAddbuildsCall) Do(opts ...googleapi.CallOption) (*LabelAddBuildsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &LabelAddBuildsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.label.addbuilds",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}/addBuilds",
	//   "request": {
	//     "$ref": "LabelAddBuildsRequest"
	//   },
	//   "response": {
	//     "$ref": "LabelAddBuildsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.clone":

type LabelCloneCall struct {
	s               *Service
	namespace       string
	name            string
	destinationName string
	urlParams_      gensupport.URLParams
	ctx_            context.Context
}

// Clone:
func (r *LabelService) Clone(namespace string, name string, destinationName string) *LabelCloneCall {
	c := &LabelCloneCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	c.destinationName = destinationName
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelCloneCall) Fields(s ...googleapi.Field) *LabelCloneCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelCloneCall) Context(ctx context.Context) *LabelCloneCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelCloneCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}/reset/{destinationName}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace":       c.namespace,
		"name":            c.name,
		"destinationName": c.destinationName,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.clone" call.
// Exactly one of *LabelCloneResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *LabelCloneResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *LabelCloneCall) Do(opts ...googleapi.CallOption) (*LabelCloneResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &LabelCloneResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.label.clone",
	//   "parameterOrder": [
	//     "namespace",
	//     "name",
	//     "destinationName"
	//   ],
	//   "parameters": {
	//     "destinationName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}/reset/{destinationName}",
	//   "response": {
	//     "$ref": "LabelCloneResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.delete":

type LabelDeleteCall struct {
	s          *Service
	namespace  string
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Delete:
func (r *LabelService) Delete(namespace string, name string) *LabelDeleteCall {
	c := &LabelDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *LabelDeleteCall) ResourceId(resourceId string) *LabelDeleteCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelDeleteCall) Fields(s ...googleapi.Field) *LabelDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelDeleteCall) Context(ctx context.Context) *LabelDeleteCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelDeleteCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
		"name":      c.name,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.delete" call.
func (c *LabelDeleteCall) Do(opts ...googleapi.CallOption) error {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
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
	//   "id": "androidbuildinternal.label.delete",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}",
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.get":

type LabelGetCall struct {
	s            *Service
	namespace    string
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *LabelService) Get(namespace string, name string) *LabelGetCall {
	c := &LabelGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *LabelGetCall) ResourceId(resourceId string) *LabelGetCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelGetCall) Fields(s ...googleapi.Field) *LabelGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *LabelGetCall) IfNoneMatch(entityTag string) *LabelGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelGetCall) Context(ctx context.Context) *LabelGetCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
		"name":      c.name,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.get" call.
// Exactly one of *Label or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Label.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *LabelGetCall) Do(opts ...googleapi.CallOption) (*Label, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Label{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.label.get",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}",
	//   "response": {
	//     "$ref": "Label"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.list":

type LabelListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *LabelService) List() *LabelListCall {
	c := &LabelListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// BuildId sets the optional parameter "buildId":
func (c *LabelListCall) BuildId(buildId string) *LabelListCall {
	c.urlParams_.Set("buildId", buildId)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *LabelListCall) MaxResults(maxResults int64) *LabelListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// Name sets the optional parameter "name":
func (c *LabelListCall) Name(name string) *LabelListCall {
	c.urlParams_.Set("name", name)
	return c
}

// Namespace sets the optional parameter "namespace":
func (c *LabelListCall) Namespace(namespace string) *LabelListCall {
	c.urlParams_.Set("namespace", namespace)
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *LabelListCall) PageToken(pageToken string) *LabelListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelListCall) Fields(s ...googleapi.Field) *LabelListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *LabelListCall) IfNoneMatch(entityTag string) *LabelListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelListCall) Context(ctx context.Context) *LabelListCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.list" call.
// Exactly one of *LabelListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *LabelListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *LabelListCall) Do(opts ...googleapi.CallOption) (*LabelListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &LabelListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.label.list",
	//   "parameters": {
	//     "buildId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "maxResults": {
	//       "format": "uint32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "name": {
	//       "location": "query",
	//       "type": "string"
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
	//   "path": "labels",
	//   "response": {
	//     "$ref": "LabelListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.patch":

type LabelPatchCall struct {
	s          *Service
	namespace  string
	label      *Label
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Patch:
func (r *LabelService) Patch(namespace string, name string, label *Label) *LabelPatchCall {
	c := &LabelPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.urlParams_.Set("name", name)
	c.label = label
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *LabelPatchCall) ResourceId(resourceId string) *LabelPatchCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelPatchCall) Fields(s ...googleapi.Field) *LabelPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelPatchCall) Context(ctx context.Context) *LabelPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.label)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.patch" call.
// Exactly one of *Label or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Label.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *LabelPatchCall) Do(opts ...googleapi.CallOption) (*Label, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Label{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.label.patch",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "query",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}",
	//   "request": {
	//     "$ref": "Label"
	//   },
	//   "response": {
	//     "$ref": "Label"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.removebuilds":

type LabelRemovebuildsCall struct {
	s                        *Service
	namespace                string
	name                     string
	labelremovebuildsrequest *LabelRemoveBuildsRequest
	urlParams_               gensupport.URLParams
	ctx_                     context.Context
}

// Removebuilds:
func (r *LabelService) Removebuilds(namespace string, name string, labelremovebuildsrequest *LabelRemoveBuildsRequest) *LabelRemovebuildsCall {
	c := &LabelRemovebuildsCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	c.labelremovebuildsrequest = labelremovebuildsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelRemovebuildsCall) Fields(s ...googleapi.Field) *LabelRemovebuildsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelRemovebuildsCall) Context(ctx context.Context) *LabelRemovebuildsCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelRemovebuildsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.labelremovebuildsrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}/removeBuilds")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
		"name":      c.name,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.removebuilds" call.
// Exactly one of *LabelRemoveBuildsResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *LabelRemoveBuildsResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *LabelRemovebuildsCall) Do(opts ...googleapi.CallOption) (*LabelRemoveBuildsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &LabelRemoveBuildsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.label.removebuilds",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}/removeBuilds",
	//   "request": {
	//     "$ref": "LabelRemoveBuildsRequest"
	//   },
	//   "response": {
	//     "$ref": "LabelRemoveBuildsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.reset":

type LabelResetCall struct {
	s          *Service
	namespace  string
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Reset:
func (r *LabelService) Reset(namespace string, name string) *LabelResetCall {
	c := &LabelResetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelResetCall) Fields(s ...googleapi.Field) *LabelResetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelResetCall) Context(ctx context.Context) *LabelResetCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelResetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}/{name}/reset")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
		"name":      c.name,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.reset" call.
// Exactly one of *LabelResetResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *LabelResetResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *LabelResetCall) Do(opts ...googleapi.CallOption) (*LabelResetResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &LabelResetResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.label.reset",
	//   "parameterOrder": [
	//     "namespace",
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}/{name}/reset",
	//   "response": {
	//     "$ref": "LabelResetResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.label.update":

type LabelUpdateCall struct {
	s          *Service
	namespace  string
	label      *Label
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Update:
func (r *LabelService) Update(namespace string, label *Label) *LabelUpdateCall {
	c := &LabelUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.namespace = namespace
	c.label = label
	return c
}

// Name sets the optional parameter "name":
func (c *LabelUpdateCall) Name(name string) *LabelUpdateCall {
	c.urlParams_.Set("name", name)
	return c
}

// ResourceId sets the optional parameter "resourceId":
func (c *LabelUpdateCall) ResourceId(resourceId string) *LabelUpdateCall {
	c.urlParams_.Set("resourceId", resourceId)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *LabelUpdateCall) Fields(s ...googleapi.Field) *LabelUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *LabelUpdateCall) Context(ctx context.Context) *LabelUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *LabelUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.label)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "labels/{namespace}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"namespace": c.namespace,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.label.update" call.
// Exactly one of *Label or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Label.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *LabelUpdateCall) Do(opts ...googleapi.CallOption) (*Label, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Label{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.label.update",
	//   "parameterOrder": [
	//     "namespace"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "namespace": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "resourceId": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "labels/{namespace}",
	//   "request": {
	//     "$ref": "Label"
	//   },
	//   "response": {
	//     "$ref": "Label"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.target.get":

type TargetGetCall struct {
	s            *Service
	branch       string
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *TargetService) Get(branch string, resourceId string) *TargetGetCall {
	c := &TargetGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.branch = branch
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TargetGetCall) Fields(s ...googleapi.Field) *TargetGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TargetGetCall) IfNoneMatch(entityTag string) *TargetGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TargetGetCall) Context(ctx context.Context) *TargetGetCall {
	c.ctx_ = ctx
	return c
}

func (c *TargetGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{branch}/targets/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch":     c.branch,
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.target.get" call.
// Exactly one of *Target or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Target.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *TargetGetCall) Do(opts ...googleapi.CallOption) (*Target, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Target{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	branch       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *TargetService) List(branch string) *TargetListCall {
	c := &TargetListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.branch = branch
	return c
}

// LaunchcontrolName sets the optional parameter "launchcontrolName":
func (c *TargetListCall) LaunchcontrolName(launchcontrolName string) *TargetListCall {
	c.urlParams_.Set("launchcontrolName", launchcontrolName)
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *TargetListCall) MaxResults(maxResults int64) *TargetListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *TargetListCall) PageToken(pageToken string) *TargetListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Product sets the optional parameter "product":
func (c *TargetListCall) Product(product string) *TargetListCall {
	c.urlParams_.Set("product", product)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TargetListCall) Fields(s ...googleapi.Field) *TargetListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TargetListCall) IfNoneMatch(entityTag string) *TargetListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TargetListCall) Context(ctx context.Context) *TargetListCall {
	c.ctx_ = ctx
	return c
}

func (c *TargetListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "branches/{branch}/targets")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"branch": c.branch,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.target.list" call.
// Exactly one of *TargetListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *TargetListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TargetListCall) Do(opts ...googleapi.CallOption) (*TargetListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TargetListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	//     "launchcontrolName": {
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
	//     "product": {
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

// method id "androidbuildinternal.testartifact.copyTo":

type TestartifactCopyToCall struct {
	s            *Service
	buildType    string
	buildId      string
	target       string
	attemptId    string
	testResultId int64
	artifactName string
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// CopyTo:
func (r *TestartifactService) CopyTo(buildType string, buildId string, target string, attemptId string, testResultId int64, artifactName string) *TestartifactCopyToCall {
	c := &TestartifactCopyToCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	c.artifactName = artifactName
	return c
}

// DestinationBucket sets the optional parameter "destinationBucket":
func (c *TestartifactCopyToCall) DestinationBucket(destinationBucket string) *TestartifactCopyToCall {
	c.urlParams_.Set("destinationBucket", destinationBucket)
	return c
}

// DestinationPath sets the optional parameter "destinationPath":
func (c *TestartifactCopyToCall) DestinationPath(destinationPath string) *TestartifactCopyToCall {
	c.urlParams_.Set("destinationPath", destinationPath)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactCopyToCall) Fields(s ...googleapi.Field) *TestartifactCopyToCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestartifactCopyToCall) Context(ctx context.Context) *TestartifactCopyToCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactCopyToCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{artifactName}/copyTo")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
		"artifactName": c.artifactName,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testartifact.copyTo" call.
// Exactly one of *TestArtifactCopyToResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *TestArtifactCopyToResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestartifactCopyToCall) Do(opts ...googleapi.CallOption) (*TestArtifactCopyToResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestArtifactCopyToResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.testartifact.copyTo",
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId",
	//     "artifactName"
	//   ],
	//   "parameters": {
	//     "artifactName": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
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
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "destinationBucket": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "destinationPath": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "target": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{artifactName}/copyTo",
	//   "response": {
	//     "$ref": "TestArtifactCopyToResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testartifact.delete":

type TestartifactDeleteCall struct {
	s            *Service
	buildType    string
	buildId      string
	target       string
	attemptId    string
	testResultId int64
	resourceId   string
	urlParams_   gensupport.URLParams
	ctx_         context.Context
}

// Delete:
func (r *TestartifactService) Delete(buildType string, buildId string, target string, attemptId string, testResultId int64, resourceId string) *TestartifactDeleteCall {
	c := &TestartifactDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	c.resourceId = resourceId
	return c
}

// DeleteObject sets the optional parameter "deleteObject":
func (c *TestartifactDeleteCall) DeleteObject(deleteObject bool) *TestartifactDeleteCall {
	c.urlParams_.Set("deleteObject", fmt.Sprint(deleteObject))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactDeleteCall) Fields(s ...googleapi.Field) *TestartifactDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestartifactDeleteCall) Context(ctx context.Context) *TestartifactDeleteCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactDeleteCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
		"resourceId":   c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testartifact.delete" call.
func (c *TestartifactDeleteCall) Do(opts ...googleapi.CallOption) error {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
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
	//   "id": "androidbuildinternal.testartifact.delete",
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId",
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
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "deleteObject": {
	//       "default": "true",
	//       "location": "query",
	//       "type": "boolean"
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
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}",
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testartifact.get":

type TestartifactGetCall struct {
	s            *Service
	buildType    string
	buildId      string
	target       string
	attemptId    string
	testResultId int64
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *TestartifactService) Get(buildType string, buildId string, target string, attemptId string, testResultId int64, resourceId string) *TestartifactGetCall {
	c := &TestartifactGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactGetCall) Fields(s ...googleapi.Field) *TestartifactGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TestartifactGetCall) IfNoneMatch(entityTag string) *TestartifactGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do and Download
// methods. Any pending HTTP request will be aborted if the provided
// context is canceled.
func (c *TestartifactGetCall) Context(ctx context.Context) *TestartifactGetCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
		"resourceId":   c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Download fetches the API endpoint's "media" value, instead of the normal
// API response value. If the returned error is nil, the Response is guaranteed to
// have a 2xx status code. Callers must close the Response.Body as usual.
func (c *TestartifactGetCall) Download(opts ...googleapi.CallOption) (*http.Response, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("media")
	if err != nil {
		return nil, err
	}
	if err := googleapi.CheckMediaResponse(res); err != nil {
		res.Body.Close()
		return nil, err
	}
	return res, nil
}

// Do executes the "androidbuildinternal.testartifact.get" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestartifactGetCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.testartifact.get",
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId",
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
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}",
	//   "response": {
	//     "$ref": "BuildArtifactMetadata"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ],
	//   "supportsMediaDownload": true,
	//   "useMediaDownloadService": true
	// }

}

// method id "androidbuildinternal.testartifact.list":

type TestartifactListCall struct {
	s            *Service
	buildType    string
	buildId      string
	target       string
	attemptId    string
	testResultId int64
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *TestartifactService) List(buildType string, buildId string, target string, attemptId string, testResultId int64) *TestartifactListCall {
	c := &TestartifactListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *TestartifactListCall) MaxResults(maxResults int64) *TestartifactListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *TestartifactListCall) PageToken(pageToken string) *TestartifactListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactListCall) Fields(s ...googleapi.Field) *TestartifactListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TestartifactListCall) IfNoneMatch(entityTag string) *TestartifactListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestartifactListCall) Context(ctx context.Context) *TestartifactListCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testartifact.list" call.
// Exactly one of *TestArtifactListResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *TestArtifactListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestartifactListCall) Do(opts ...googleapi.CallOption) (*TestArtifactListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestArtifactListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.testartifact.list",
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId"
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
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts",
	//   "response": {
	//     "$ref": "TestArtifactListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.testartifact.patch":

type TestartifactPatchCall struct {
	s                     *Service
	buildType             string
	buildId               string
	target                string
	attemptId             string
	testResultId          int64
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	urlParams_            gensupport.URLParams
	ctx_                  context.Context
}

// Patch:
func (r *TestartifactService) Patch(buildType string, buildId string, target string, attemptId string, testResultId int64, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *TestartifactPatchCall {
	c := &TestartifactPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactPatchCall) Fields(s ...googleapi.Field) *TestartifactPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestartifactPatchCall) Context(ctx context.Context) *TestartifactPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
		"resourceId":   c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testartifact.patch" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestartifactPatchCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.testartifact.patch",
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId",
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
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}",
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

// method id "androidbuildinternal.testartifact.update":

type TestartifactUpdateCall struct {
	s                     *Service
	buildType             string
	buildId               string
	target                string
	attemptId             string
	testResultId          int64
	resourceId            string
	buildartifactmetadata *BuildArtifactMetadata
	urlParams_            gensupport.URLParams
	media_                io.Reader
	resumableBuffer_      *gensupport.ResumableBuffer
	mediaType_            string
	mediaSize_            int64 // mediaSize, if known.  Used only for calls to progressUpdater_.
	progressUpdater_      googleapi.ProgressUpdater
	ctx_                  context.Context
}

// Update:
func (r *TestartifactService) Update(buildType string, buildId string, target string, attemptId string, testResultId int64, resourceId string, buildartifactmetadata *BuildArtifactMetadata) *TestartifactUpdateCall {
	c := &TestartifactUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildType = buildType
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testResultId = testResultId
	c.resourceId = resourceId
	c.buildartifactmetadata = buildartifactmetadata
	return c
}

// Media specifies the media to upload in a single chunk. At most one of
// Media and ResumableMedia may be set.
func (c *TestartifactUpdateCall) Media(r io.Reader, options ...googleapi.MediaOption) *TestartifactUpdateCall {
	opts := googleapi.ProcessMediaOptions(options)
	chunkSize := opts.ChunkSize
	r, c.mediaType_ = gensupport.DetermineContentType(r, opts.ContentType)
	c.media_, c.resumableBuffer_ = gensupport.PrepareUpload(r, chunkSize)
	return c
}

// ResumableMedia specifies the media to upload in chunks and can be
// canceled with ctx. ResumableMedia is deprecated in favour of Media.
// At most one of Media and ResumableMedia may be set. mediaType
// identifies the MIME media type of the upload, such as "image/png". If
// mediaType is "", it will be auto-detected. The provided ctx will
// supersede any context previously provided to the Context method.
func (c *TestartifactUpdateCall) ResumableMedia(ctx context.Context, r io.ReaderAt, size int64, mediaType string) *TestartifactUpdateCall {
	c.ctx_ = ctx
	rdr := gensupport.ReaderAtToReader(r, size)
	rdr, c.mediaType_ = gensupport.DetermineContentType(rdr, mediaType)
	c.resumableBuffer_ = gensupport.NewResumableBuffer(rdr, googleapi.DefaultUploadChunkSize)
	c.media_ = nil
	c.mediaSize_ = size
	return c
}

// ProgressUpdater provides a callback function that will be called
// after every chunk. It should be a low-latency function in order to
// not slow down the upload operation. This should only be called when
// using ResumableMedia (as opposed to Media).
func (c *TestartifactUpdateCall) ProgressUpdater(pu googleapi.ProgressUpdater) *TestartifactUpdateCall {
	c.progressUpdater_ = pu
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestartifactUpdateCall) Fields(s ...googleapi.Field) *TestartifactUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
// This context will supersede any context previously provided to the
// ResumableMedia method.
func (c *TestartifactUpdateCall) Context(ctx context.Context) *TestartifactUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *TestartifactUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.buildartifactmetadata)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}")
	if c.media_ != nil || c.resumableBuffer_ != nil {
		urls = strings.Replace(urls, "https://www.googleapis.com/", "https://www.googleapis.com/upload/", 1)
		protocol := "multipart"
		if c.resumableBuffer_ != nil {
			protocol = "resumable"
		}
		c.urlParams_.Set("uploadType", protocol)
	}
	urls += "?" + c.urlParams_.Encode()
	if c.media_ != nil {
		var combined io.ReadCloser
		combined, ctype = gensupport.CombineBodyMedia(body, ctype, c.media_, c.mediaType_)
		defer combined.Close()
		body = combined
	}
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildType":    c.buildType,
		"buildId":      c.buildId,
		"target":       c.target,
		"attemptId":    c.attemptId,
		"testResultId": strconv.FormatInt(c.testResultId, 10),
		"resourceId":   c.resourceId,
	})
	if c.resumableBuffer_ != nil {
		req.Header.Set("X-Upload-Content-Type", c.mediaType_)
	}
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testartifact.update" call.
// Exactly one of *BuildArtifactMetadata or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *BuildArtifactMetadata.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestartifactUpdateCall) Do(opts ...googleapi.CallOption) (*BuildArtifactMetadata, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	if c.resumableBuffer_ != nil {
		loc := res.Header.Get("Location")
		rx := &gensupport.ResumableUpload{
			Client:    c.s.client,
			UserAgent: c.s.userAgent(),
			URI:       loc,
			Media:     c.resumableBuffer_,
			MediaType: c.mediaType_,
			Callback: func(curr int64) {
				if c.progressUpdater_ != nil {
					c.progressUpdater_(curr, c.mediaSize_)
				}
			},
		}
		ctx := c.ctx_
		if ctx == nil {
			ctx = context.TODO()
		}
		res, err = rx.Upload(ctx)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
	}
	ret := &BuildArtifactMetadata{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.testartifact.update",
	//   "mediaUpload": {
	//     "accept": [
	//       "*/*"
	//     ],
	//     "maxSize": "2GB",
	//     "protocols": {
	//       "resumable": {
	//         "multipart": true,
	//         "path": "/resumable/upload/android/internal/build/v2beta1/builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}"
	//       },
	//       "simple": {
	//         "multipart": true,
	//         "path": "/upload/android/internal/build/v2beta1/builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}"
	//       }
	//     }
	//   },
	//   "parameterOrder": [
	//     "buildType",
	//     "buildId",
	//     "target",
	//     "attemptId",
	//     "testResultId",
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
	//     },
	//     "testResultId": {
	//       "format": "int64",
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "builds/{buildType}/{buildId}/{target}/attempts/{attemptId}/tests/{testResultId}/artifacts/{resourceId}",
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

// method id "androidbuildinternal.testresult.get":

type TestresultGetCall struct {
	s            *Service
	buildId      string
	target       string
	attemptId    string
	resourceId   int64
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *TestresultService) Get(buildId string, target string, attemptId string, resourceId int64) *TestresultGetCall {
	c := &TestresultGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultGetCall) Fields(s ...googleapi.Field) *TestresultGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TestresultGetCall) IfNoneMatch(entityTag string) *TestresultGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestresultGetCall) Context(ctx context.Context) *TestresultGetCall {
	c.ctx_ = ctx
	return c
}

func (c *TestresultGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testresult.get" call.
// Exactly one of *TestResult or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *TestResult.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *TestresultGetCall) Do(opts ...googleapi.CallOption) (*TestResult, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestResult{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Insert:
func (r *TestresultService) Insert(buildId string, target string, attemptId string, testresult *TestResult) *TestresultInsertCall {
	c := &TestresultInsertCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultInsertCall) Fields(s ...googleapi.Field) *TestresultInsertCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestresultInsertCall) Context(ctx context.Context) *TestresultInsertCall {
	c.ctx_ = ctx
	return c
}

func (c *TestresultInsertCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testresult.insert" call.
// Exactly one of *TestResult or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *TestResult.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *TestresultInsertCall) Do(opts ...googleapi.CallOption) (*TestResult, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestResult{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	s            *Service
	buildId      string
	target       string
	attemptId    string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *TestresultService) List(buildId string, target string, attemptId string) *TestresultListCall {
	c := &TestresultListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *TestresultListCall) MaxResults(maxResults int64) *TestresultListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *TestresultListCall) PageToken(pageToken string) *TestresultListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultListCall) Fields(s ...googleapi.Field) *TestresultListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *TestresultListCall) IfNoneMatch(entityTag string) *TestresultListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestresultListCall) Context(ctx context.Context) *TestresultListCall {
	c.ctx_ = ctx
	return c
}

func (c *TestresultListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":   c.buildId,
		"target":    c.target,
		"attemptId": c.attemptId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testresult.list" call.
// Exactly one of *TestResultListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *TestResultListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *TestresultListCall) Do(opts ...googleapi.CallOption) (*TestResultListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestResultListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Patch:
func (r *TestresultService) Patch(buildId string, target string, attemptId string, resourceId int64, testresult *TestResult) *TestresultPatchCall {
	c := &TestresultPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultPatchCall) Fields(s ...googleapi.Field) *TestresultPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestresultPatchCall) Context(ctx context.Context) *TestresultPatchCall {
	c.ctx_ = ctx
	return c
}

func (c *TestresultPatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testresult.patch" call.
// Exactly one of *TestResult or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *TestResult.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *TestresultPatchCall) Do(opts ...googleapi.CallOption) (*TestResult, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestResult{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Update:
func (r *TestresultService) Update(buildId string, target string, attemptId string, resourceId int64, testresult *TestResult) *TestresultUpdateCall {
	c := &TestresultUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.buildId = buildId
	c.target = target
	c.attemptId = attemptId
	c.resourceId = resourceId
	c.testresult = testresult
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *TestresultUpdateCall) Fields(s ...googleapi.Field) *TestresultUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *TestresultUpdateCall) Context(ctx context.Context) *TestresultUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *TestresultUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testresult)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "builds/{buildId}/{target}/attempts/{attemptId}/tests/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"buildId":    c.buildId,
		"target":     c.target,
		"attemptId":  c.attemptId,
		"resourceId": strconv.FormatInt(c.resourceId, 10),
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.testresult.update" call.
// Exactly one of *TestResult or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *TestResult.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *TestresultUpdateCall) Do(opts ...googleapi.CallOption) (*TestResult, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &TestResult{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
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

// method id "androidbuildinternal.worknode.complete":

type WorknodeCompleteCall struct {
	s                       *Service
	worknodecompleterequest *WorkNodeCompleteRequest
	urlParams_              gensupport.URLParams
	ctx_                    context.Context
}

// Complete:
func (r *WorknodeService) Complete(worknodecompleterequest *WorkNodeCompleteRequest) *WorknodeCompleteCall {
	c := &WorknodeCompleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.worknodecompleterequest = worknodecompleterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodeCompleteCall) Fields(s ...googleapi.Field) *WorknodeCompleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodeCompleteCall) Context(ctx context.Context) *WorknodeCompleteCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodeCompleteCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.worknodecompleterequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/complete")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.complete" call.
// Exactly one of *WorkNodeCompleteResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *WorkNodeCompleteResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorknodeCompleteCall) Do(opts ...googleapi.CallOption) (*WorkNodeCompleteResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNodeCompleteResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.worknode.complete",
	//   "path": "workNodes/complete",
	//   "request": {
	//     "$ref": "WorkNodeCompleteRequest"
	//   },
	//   "response": {
	//     "$ref": "WorkNodeCompleteResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.fail":

type WorknodeFailCall struct {
	s                   *Service
	worknodefailrequest *WorkNodeFailRequest
	urlParams_          gensupport.URLParams
	ctx_                context.Context
}

// Fail:
func (r *WorknodeService) Fail(worknodefailrequest *WorkNodeFailRequest) *WorknodeFailCall {
	c := &WorknodeFailCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.worknodefailrequest = worknodefailrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodeFailCall) Fields(s ...googleapi.Field) *WorknodeFailCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodeFailCall) Context(ctx context.Context) *WorknodeFailCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodeFailCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.worknodefailrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/fail")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.fail" call.
// Exactly one of *WorkNodeFailResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *WorkNodeFailResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorknodeFailCall) Do(opts ...googleapi.CallOption) (*WorkNodeFailResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNodeFailResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.worknode.fail",
	//   "path": "workNodes/fail",
	//   "request": {
	//     "$ref": "WorkNodeFailRequest"
	//   },
	//   "response": {
	//     "$ref": "WorkNodeFailResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.get":

type WorknodeGetCall struct {
	s            *Service
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *WorknodeService) Get(resourceId string) *WorknodeGetCall {
	c := &WorknodeGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodeGetCall) Fields(s ...googleapi.Field) *WorknodeGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *WorknodeGetCall) IfNoneMatch(entityTag string) *WorknodeGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodeGetCall) Context(ctx context.Context) *WorknodeGetCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodeGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.get" call.
// Exactly one of *WorkNode or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *WorkNode.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *WorknodeGetCall) Do(opts ...googleapi.CallOption) (*WorkNode, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNode{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.worknode.get",
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
	//   "path": "workNodes/{resourceId}",
	//   "response": {
	//     "$ref": "WorkNode"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.list":

type WorknodeListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *WorknodeService) List() *WorknodeListCall {
	c := &WorknodeListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// ChangeSetSpecId sets the optional parameter "changeSetSpecId":
func (c *WorknodeListCall) ChangeSetSpecId(changeSetSpecId string) *WorknodeListCall {
	c.urlParams_.Set("changeSetSpecId", changeSetSpecId)
	return c
}

// IsFinal sets the optional parameter "isFinal":
func (c *WorknodeListCall) IsFinal(isFinal bool) *WorknodeListCall {
	c.urlParams_.Set("isFinal", fmt.Sprint(isFinal))
	return c
}

// IsTimedOut sets the optional parameter "isTimedOut":
func (c *WorknodeListCall) IsTimedOut(isTimedOut bool) *WorknodeListCall {
	c.urlParams_.Set("isTimedOut", fmt.Sprint(isTimedOut))
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *WorknodeListCall) MaxResults(maxResults int64) *WorknodeListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *WorknodeListCall) PageToken(pageToken string) *WorknodeListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Status sets the optional parameter "status":
//
// Possible values:
//   "cancelled"
//   "complete"
//   "created"
//   "failed"
//   "pending"
//   "running"
//   "scheduled"
//   "unknownWorkNodeStatus"
func (c *WorknodeListCall) Status(status ...string) *WorknodeListCall {
	c.urlParams_.SetMulti("status", append([]string{}, status...))
	return c
}

// WorkExecutorTypes sets the optional parameter "workExecutorTypes":
//
// Possible values:
//   "atpTest"
//   "dummyNode"
//   "pendingChangeBuild"
//   "pendingChangeFinished"
//   "unknownWorkExecutorType"
func (c *WorknodeListCall) WorkExecutorTypes(workExecutorTypes ...string) *WorknodeListCall {
	c.urlParams_.SetMulti("workExecutorTypes", append([]string{}, workExecutorTypes...))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodeListCall) Fields(s ...googleapi.Field) *WorknodeListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *WorknodeListCall) IfNoneMatch(entityTag string) *WorknodeListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodeListCall) Context(ctx context.Context) *WorknodeListCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodeListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.list" call.
// Exactly one of *WorkNodeListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *WorkNodeListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorknodeListCall) Do(opts ...googleapi.CallOption) (*WorkNodeListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNodeListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.worknode.list",
	//   "parameters": {
	//     "changeSetSpecId": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "isFinal": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "isTimedOut": {
	//       "location": "query",
	//       "type": "boolean"
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
	//         "cancelled",
	//         "complete",
	//         "created",
	//         "failed",
	//         "pending",
	//         "running",
	//         "scheduled",
	//         "unknownWorkNodeStatus"
	//       ],
	//       "enumDescriptions": [
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
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "workExecutorTypes": {
	//       "enum": [
	//         "atpTest",
	//         "dummyNode",
	//         "pendingChangeBuild",
	//         "pendingChangeFinished",
	//         "unknownWorkExecutorType"
	//       ],
	//       "enumDescriptions": [
	//         "",
	//         "",
	//         "",
	//         "",
	//         ""
	//       ],
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "workNodes",
	//   "response": {
	//     "$ref": "WorkNodeListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.patch":

type WorknodePatchCall struct {
	s          *Service
	resourceId string
	worknode   *WorkNode
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Patch:
func (r *WorknodeService) Patch(resourceId string, worknode *WorkNode) *WorknodePatchCall {
	c := &WorknodePatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.worknode = worknode
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodePatchCall) Fields(s ...googleapi.Field) *WorknodePatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodePatchCall) Context(ctx context.Context) *WorknodePatchCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodePatchCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.worknode)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.patch" call.
// Exactly one of *WorkNode or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *WorkNode.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *WorknodePatchCall) Do(opts ...googleapi.CallOption) (*WorkNode, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNode{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PATCH",
	//   "id": "androidbuildinternal.worknode.patch",
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
	//   "path": "workNodes/{resourceId}",
	//   "request": {
	//     "$ref": "WorkNode"
	//   },
	//   "response": {
	//     "$ref": "WorkNode"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.pop":

type WorknodePopCall struct {
	s                  *Service
	worknodepoprequest *WorkNodePopRequest
	urlParams_         gensupport.URLParams
	ctx_               context.Context
}

// Pop:
func (r *WorknodeService) Pop(worknodepoprequest *WorkNodePopRequest) *WorknodePopCall {
	c := &WorknodePopCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.worknodepoprequest = worknodepoprequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodePopCall) Fields(s ...googleapi.Field) *WorknodePopCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodePopCall) Context(ctx context.Context) *WorknodePopCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodePopCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.worknodepoprequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/pop")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.pop" call.
// Exactly one of *WorkNodePopResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *WorkNodePopResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorknodePopCall) Do(opts ...googleapi.CallOption) (*WorkNodePopResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNodePopResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.worknode.pop",
	//   "path": "workNodes/pop",
	//   "request": {
	//     "$ref": "WorkNodePopRequest"
	//   },
	//   "response": {
	//     "$ref": "WorkNodePopResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.worknode.update":

type WorknodeUpdateCall struct {
	s          *Service
	resourceId string
	worknode   *WorkNode
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Update:
func (r *WorknodeService) Update(resourceId string, worknode *WorkNode) *WorknodeUpdateCall {
	c := &WorknodeUpdateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	c.worknode = worknode
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorknodeUpdateCall) Fields(s ...googleapi.Field) *WorknodeUpdateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorknodeUpdateCall) Context(ctx context.Context) *WorknodeUpdateCall {
	c.ctx_ = ctx
	return c
}

func (c *WorknodeUpdateCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.worknode)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workNodes/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PUT", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.worknode.update" call.
// Exactly one of *WorkNode or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *WorkNode.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *WorknodeUpdateCall) Do(opts ...googleapi.CallOption) (*WorkNode, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkNode{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "PUT",
	//   "id": "androidbuildinternal.worknode.update",
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
	//   "path": "workNodes/{resourceId}",
	//   "request": {
	//     "$ref": "WorkNode"
	//   },
	//   "response": {
	//     "$ref": "WorkNode"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.workplan.addnodes":

type WorkplanAddnodesCall struct {
	s                       *Service
	workplanaddnodesrequest *WorkPlanAddNodesRequest
	urlParams_              gensupport.URLParams
	ctx_                    context.Context
}

// Addnodes:
func (r *WorkplanService) Addnodes(workplanaddnodesrequest *WorkPlanAddNodesRequest) *WorkplanAddnodesCall {
	c := &WorkplanAddnodesCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.workplanaddnodesrequest = workplanaddnodesrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorkplanAddnodesCall) Fields(s ...googleapi.Field) *WorkplanAddnodesCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorkplanAddnodesCall) Context(ctx context.Context) *WorkplanAddnodesCall {
	c.ctx_ = ctx
	return c
}

func (c *WorkplanAddnodesCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.workplanaddnodesrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workPlans/addNodes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.workplan.addnodes" call.
// Exactly one of *WorkPlanAddNodesResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *WorkPlanAddNodesResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorkplanAddnodesCall) Do(opts ...googleapi.CallOption) (*WorkPlanAddNodesResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkPlanAddNodesResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.workplan.addnodes",
	//   "path": "workPlans/addNodes",
	//   "request": {
	//     "$ref": "WorkPlanAddNodesRequest"
	//   },
	//   "response": {
	//     "$ref": "WorkPlanAddNodesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.workplan.createwithnodes":

type WorkplanCreatewithnodesCall struct {
	s                              *Service
	workplancreatewithnodesrequest *WorkPlanCreateWithNodesRequest
	urlParams_                     gensupport.URLParams
	ctx_                           context.Context
}

// Createwithnodes:
func (r *WorkplanService) Createwithnodes(workplancreatewithnodesrequest *WorkPlanCreateWithNodesRequest) *WorkplanCreatewithnodesCall {
	c := &WorkplanCreatewithnodesCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.workplancreatewithnodesrequest = workplancreatewithnodesrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorkplanCreatewithnodesCall) Fields(s ...googleapi.Field) *WorkplanCreatewithnodesCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorkplanCreatewithnodesCall) Context(ctx context.Context) *WorkplanCreatewithnodesCall {
	c.ctx_ = ctx
	return c
}

func (c *WorkplanCreatewithnodesCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.workplancreatewithnodesrequest)
	if err != nil {
		return nil, err
	}
	ctype := "application/json"
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workPlans/createWithNodes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.workplan.createwithnodes" call.
// Exactly one of *WorkPlanCreateWithNodesResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *WorkPlanCreateWithNodesResponse.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorkplanCreatewithnodesCall) Do(opts ...googleapi.CallOption) (*WorkPlanCreateWithNodesResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkPlanCreateWithNodesResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "androidbuildinternal.workplan.createwithnodes",
	//   "path": "workPlans/createWithNodes",
	//   "request": {
	//     "$ref": "WorkPlanCreateWithNodesRequest"
	//   },
	//   "response": {
	//     "$ref": "WorkPlanCreateWithNodesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.workplan.get":

type WorkplanGetCall struct {
	s            *Service
	resourceId   string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// Get:
func (r *WorkplanService) Get(resourceId string) *WorkplanGetCall {
	c := &WorkplanGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resourceId = resourceId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorkplanGetCall) Fields(s ...googleapi.Field) *WorkplanGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *WorkplanGetCall) IfNoneMatch(entityTag string) *WorkplanGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorkplanGetCall) Context(ctx context.Context) *WorkplanGetCall {
	c.ctx_ = ctx
	return c
}

func (c *WorkplanGetCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workPlans/{resourceId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"resourceId": c.resourceId,
	})
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.workplan.get" call.
// Exactly one of *WorkPlan or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *WorkPlan.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *WorkplanGetCall) Do(opts ...googleapi.CallOption) (*WorkPlan, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkPlan{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.workplan.get",
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
	//   "path": "workPlans/{resourceId}",
	//   "response": {
	//     "$ref": "WorkPlan"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}

// method id "androidbuildinternal.workplan.list":

type WorkplanListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// List:
func (r *WorkplanService) List() *WorkplanListCall {
	c := &WorkplanListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// MaxResults sets the optional parameter "maxResults":
func (c *WorkplanListCall) MaxResults(maxResults int64) *WorkplanListCall {
	c.urlParams_.Set("maxResults", fmt.Sprint(maxResults))
	return c
}

// PageToken sets the optional parameter "pageToken":
func (c *WorkplanListCall) PageToken(pageToken string) *WorkplanListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *WorkplanListCall) Fields(s ...googleapi.Field) *WorkplanListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *WorkplanListCall) IfNoneMatch(entityTag string) *WorkplanListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *WorkplanListCall) Context(ctx context.Context) *WorkplanListCall {
	c.ctx_ = ctx
	return c
}

func (c *WorkplanListCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "workPlans")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		req.Header.Set("If-None-Match", c.ifNoneMatch_)
	}
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "androidbuildinternal.workplan.list" call.
// Exactly one of *WorkPlanListResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *WorkPlanListResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *WorkplanListCall) Do(opts ...googleapi.CallOption) (*WorkPlanListResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &WorkPlanListResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "GET",
	//   "id": "androidbuildinternal.workplan.list",
	//   "parameters": {
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
	//   "path": "workPlans",
	//   "response": {
	//     "$ref": "WorkPlanListResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/androidbuild.internal"
	//   ]
	// }

}
