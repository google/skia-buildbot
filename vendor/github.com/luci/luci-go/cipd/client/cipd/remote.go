// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cipd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/cipd/client/cipd/common"
)

// remoteMaxRetries is how many times to retry transient HTTP errors.
const remoteMaxRetries = 10

type packageInstanceMsg struct {
	PackageName  string `json:"package_name"`
	InstanceID   string `json:"instance_id"`
	RegisteredBy string `json:"registered_by"`
	RegisteredTs string `json:"registered_ts"`
}

// roleChangeMsg corresponds to RoleChange proto message on backend.
type roleChangeMsg struct {
	Action    string `json:"action"`
	Role      string `json:"role"`
	Principal string `json:"principal"`
}

// pendingProcessingError is returned by attachTags if package instance is not
// yet ready and the call should be retried later.
type pendingProcessingError struct {
	message string
}

func (e *pendingProcessingError) Error() string {
	return e.message
}

// remoteImpl implements remote on top of real HTTP calls.
type remoteImpl struct {
	serviceURL string
	userAgent  string
	client     *http.Client
}

func isTemporaryNetError(err error) bool {
	// TODO(vadimsh): Figure out how to recognize dial timeouts, read timeouts,
	// etc. For now all network related errors that end up here are considered
	// temporary.
	switch err {
	case context.Canceled:
		return false
	case context.DeadlineExceeded:
		return false
	}
	return true
}

// isTemporaryHTTPError returns true for HTTP status codes that indicate
// a temporary error that may go away if request is retried.
func isTemporaryHTTPError(statusCode int) bool {
	return statusCode >= 500 || statusCode == 408 || statusCode == 429
}

// makeRequest sends POST or GET REST JSON requests with retries.
func (r *remoteImpl) makeRequest(ctx context.Context, path, method string, request, response interface{}) error {
	var body []byte
	if request != nil {
		b, err := json.Marshal(request)
		if err != nil {
			return err
		}
		body = b
	}

	// Logs warning is context is not canceled yet.
	logWarning := func(msg string, args ...interface{}) {
		if err := ctx.Err(); err == nil {
			logging.Warningf(ctx, msg, args...)
		}
	}

	url := fmt.Sprintf("%s/_ah/api/%s", r.serviceURL, path)
	logging.Debugf(ctx, "cipd: %s %s", method, url)
	for attempt := 0; attempt < remoteMaxRetries; attempt++ {
		if attempt != 0 {
			logWarning("cipd: retrying request to %s", url)
			clock.Sleep(ctx, 2*time.Second)
		}

		// Context canceled?
		if err := ctx.Err(); err != nil {
			return err
		}

		// Prepare request.
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", r.userAgent)

		// Connect, read response.
		resp, err := ctxhttp.Do(ctx, r.client, req)
		if err != nil {
			if isTemporaryNetError(err) {
				logWarning("cipd: connectivity error (%s)", err)
				continue
			}
			return err
		}
		responseBody, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if isTemporaryNetError(err) {
				logWarning("cipd: temporary error when reading response (%s)", err)
				continue
			}
			return err
		}
		if isTemporaryHTTPError(resp.StatusCode) {
			continue
		}

		// Success?
		if resp.StatusCode < 300 {
			return json.Unmarshal(responseBody, response)
		}

		// Fatal error?
		if resp.StatusCode == 403 || resp.StatusCode == 401 {
			return ErrAccessDenined
		}
		return fmt.Errorf("unexpected reply (HTTP %d):\n%s", resp.StatusCode, string(responseBody))
	}

	return ErrBackendInaccessible
}

func (r *remoteImpl) initiateUpload(ctx context.Context, sha1 string) (s *UploadSession, err error) {
	var reply struct {
		Status          string `json:"status"`
		UploadSessionID string `json:"upload_session_id"`
		UploadURL       string `json:"upload_url"`
		ErrorMessage    string `json:"error_message"`
	}
	err = r.makeRequest(ctx, "cas/v1/upload/SHA1/"+sha1, "POST", nil, &reply)
	if err != nil {
		return
	}
	switch reply.Status {
	case "ALREADY_UPLOADED":
		return
	case "SUCCESS":
		s = &UploadSession{reply.UploadSessionID, reply.UploadURL}
	case "ERROR":
		err = fmt.Errorf("server replied with error: %s", reply.ErrorMessage)
	default:
		err = fmt.Errorf("unexpected status: %s", reply.Status)
	}
	return
}

func (r *remoteImpl) finalizeUpload(ctx context.Context, sessionID string) (finished bool, err error) {
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	err = r.makeRequest(ctx, "cas/v1/finalize/"+sessionID, "POST", nil, &reply)
	if err != nil {
		return
	}
	switch reply.Status {
	case "MISSING":
		err = ErrUploadSessionDied
	case "UPLOADING", "VERIFYING":
		finished = false
	case "PUBLISHED":
		finished = true
	case "ERROR":
		err = errors.New(reply.ErrorMessage)
	default:
		err = fmt.Errorf("unexpected upload session status: %s", reply.Status)
	}
	return
}

func (r *remoteImpl) resolveVersion(ctx context.Context, packageName, version string) (pin common.Pin, err error) {
	if err = common.ValidatePackageName(packageName); err != nil {
		return
	}
	if err = common.ValidateInstanceVersion(version); err != nil {
		return
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		InstanceID   string `json:"instance_id"`
	}
	params := url.Values{}
	params.Add("package_name", packageName)
	params.Add("version", version)
	err = r.makeRequest(ctx, "repo/v1/instance/resolve?"+params.Encode(), "GET", nil, &reply)
	if err != nil {
		return
	}
	switch reply.Status {
	case "SUCCESS":
		if common.ValidateInstanceID(reply.InstanceID) != nil {
			err = fmt.Errorf("backend returned invalid instance ID: %s", reply.InstanceID)
		} else {
			pin = common.Pin{PackageName: packageName, InstanceID: reply.InstanceID}
		}
	case "PACKAGE_NOT_FOUND":
		err = fmt.Errorf("package %q is not registered", packageName)
	case "INSTANCE_NOT_FOUND":
		err = fmt.Errorf("package %q doesn't have instance with version %q", packageName, version)
	case "AMBIGUOUS_VERSION":
		err = fmt.Errorf("more than one instance of package %q match version %q", packageName, version)
	case "ERROR":
		err = errors.New(reply.ErrorMessage)
	default:
		err = fmt.Errorf("unexpected backend response: %s", reply.Status)
	}
	return
}

func (r *remoteImpl) registerInstance(ctx context.Context, pin common.Pin) (*registerInstanceResponse, error) {
	endpoint, err := instanceEndpoint(pin)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status          string             `json:"status"`
		Instance        packageInstanceMsg `json:"instance"`
		UploadSessionID string             `json:"upload_session_id"`
		UploadURL       string             `json:"upload_url"`
		ErrorMessage    string             `json:"error_message"`
	}
	err = r.makeRequest(ctx, endpoint, "POST", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "REGISTERED", "ALREADY_REGISTERED":
		ts, err := convertTimestamp(reply.Instance.RegisteredTs)
		if err != nil {
			return nil, err
		}
		return &registerInstanceResponse{
			alreadyRegistered: reply.Status == "ALREADY_REGISTERED",
			registeredBy:      reply.Instance.RegisteredBy,
			registeredTs:      ts,
		}, nil
	case "UPLOAD_FIRST":
		if reply.UploadSessionID == "" {
			return nil, ErrNoUploadSessionID
		}
		return &registerInstanceResponse{
			uploadSession: &UploadSession{reply.UploadSessionID, reply.UploadURL},
		}, nil
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected register package status: %s", reply.Status)
}

func (r *remoteImpl) deletePackage(ctx context.Context, packageName string) error {
	endpoint, err := packageEndpoint(packageName)
	if err != nil {
		return err
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	if err = r.makeRequest(ctx, endpoint, "DELETE", nil, &reply); err != nil {
		return err
	}
	switch reply.Status {
	case "SUCCESS":
		return nil
	case "PACKAGE_NOT_FOUND":
		return ErrPackageNotFound
	case "ERROR":
		return errors.New(reply.ErrorMessage)
	}
	return fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) fetchInstance(ctx context.Context, pin common.Pin) (*fetchInstanceResponse, error) {
	endpoint, err := instanceEndpoint(pin)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status       string             `json:"status"`
		Instance     packageInstanceMsg `json:"instance"`
		FetchURL     string             `json:"fetch_url"`
		ErrorMessage string             `json:"error_message"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		ts, err := convertTimestamp(reply.Instance.RegisteredTs)
		if err != nil {
			return nil, err
		}
		return &fetchInstanceResponse{
			fetchURL:     reply.FetchURL,
			registeredBy: reply.Instance.RegisteredBy,
			registeredTs: ts,
		}, nil
	case "PACKAGE_NOT_FOUND":
		return nil, fmt.Errorf("package %q is not registered", pin.PackageName)
	case "INSTANCE_NOT_FOUND":
		return nil, fmt.Errorf("package %q doesn't have instance %q", pin.PackageName, pin.InstanceID)
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) fetchTags(ctx context.Context, pin common.Pin, tags []string) ([]TagInfo, error) {
	endpoint, err := tagsEndpoint(pin, tags)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Tags         []struct {
			Tag          string `json:"tag"`
			RegisteredBy string `json:"registered_by"`
			RegisteredTs string `json:"registered_ts"`
		} `json:"tags"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		out := make([]TagInfo, len(reply.Tags))
		for i, tag := range reply.Tags {
			ts, err := convertTimestamp(tag.RegisteredTs)
			if err != nil {
				logging.Warningf(ctx, "cipd: failed to parse timestamp %q: %s", tag.RegisteredTs, err)
			}
			out[i] = TagInfo{
				Tag:          tag.Tag,
				RegisteredBy: tag.RegisteredBy,
				RegisteredTs: UnixTime(ts),
			}
		}
		return out, nil
	case "PACKAGE_NOT_FOUND":
		return nil, fmt.Errorf("package %q is not registered", pin.PackageName)
	case "INSTANCE_NOT_FOUND":
		return nil, fmt.Errorf("package %q doesn't have instance %q", pin.PackageName, pin.InstanceID)
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) fetchRefs(ctx context.Context, pin common.Pin, refs []string) ([]RefInfo, error) {
	endpoint, err := refEndpoint(pin.PackageName, pin.InstanceID, refs)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Refs         []struct {
			Ref        string `json:"ref"`
			ModifiedBy string `json:"modified_by"`
			ModifiedTs string `json:"modified_ts"`
		} `json:"refs"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		out := make([]RefInfo, len(reply.Refs))
		for i, ref := range reply.Refs {
			ts, err := convertTimestamp(ref.ModifiedTs)
			if err != nil {
				logging.Warningf(ctx, "cipd: failed to parse timestamp %q: %s", ref.ModifiedTs, err)
			}
			out[i] = RefInfo{
				Ref:        ref.Ref,
				ModifiedBy: ref.ModifiedBy,
				ModifiedTs: UnixTime(ts),
			}
		}
		return out, nil
	case "PACKAGE_NOT_FOUND":
		return nil, fmt.Errorf("package %q is not registered", pin.PackageName)
	case "INSTANCE_NOT_FOUND":
		return nil, fmt.Errorf("package %q doesn't have instance %q", pin.PackageName, pin.InstanceID)
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) fetchACL(ctx context.Context, packagePath string) ([]PackageACL, error) {
	endpoint, err := aclEndpoint(packagePath)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Acls         struct {
			Acls []struct {
				PackagePath string   `json:"package_path"`
				Role        string   `json:"role"`
				Principals  []string `json:"principals"`
				ModifiedBy  string   `json:"modified_by"`
				ModifiedTs  string   `json:"modified_ts"`
			} `json:"acls"`
		} `json:"acls"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		out := []PackageACL{}
		for _, acl := range reply.Acls.Acls {
			ts, err := convertTimestamp(acl.ModifiedTs)
			if err != nil {
				return nil, err
			}
			out = append(out, PackageACL{
				PackagePath: acl.PackagePath,
				Role:        acl.Role,
				Principals:  acl.Principals,
				ModifiedBy:  acl.ModifiedBy,
				ModifiedTs:  UnixTime(ts),
			})
		}
		return out, nil
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) modifyACL(ctx context.Context, packagePath string, changes []PackageACLChange) error {
	endpoint, err := aclEndpoint(packagePath)
	if err != nil {
		return err
	}
	var request struct {
		Changes []roleChangeMsg `json:"changes"`
	}
	for _, c := range changes {
		action := ""
		if c.Action == GrantRole {
			action = "GRANT"
		} else if c.Action == RevokeRole {
			action = "REVOKE"
		} else {
			return fmt.Errorf("unexpected action: %s", action)
		}
		request.Changes = append(request.Changes, roleChangeMsg{
			Action:    action,
			Role:      c.Role,
			Principal: c.Principal,
		})
	}
	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	err = r.makeRequest(ctx, endpoint, "POST", &request, &reply)
	if err != nil {
		return err
	}
	switch reply.Status {
	case "SUCCESS":
		return nil
	case "ERROR":
		return errors.New(reply.ErrorMessage)
	}
	return fmt.Errorf("unexpected reply status: %s", reply.Status)
}

func (r *remoteImpl) setRef(ctx context.Context, ref string, pin common.Pin) error {
	if err := common.ValidatePin(pin); err != nil {
		return err
	}
	endpoint, err := refEndpoint(pin.PackageName, "", []string{ref})
	if err != nil {
		return err
	}

	var request struct {
		InstanceID string `json:"instance_id"`
	}
	request.InstanceID = pin.InstanceID

	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	if err = r.makeRequest(ctx, endpoint, "POST", &request, &reply); err != nil {
		return err
	}
	switch reply.Status {
	case "SUCCESS":
		return nil
	case "PROCESSING_NOT_FINISHED_YET":
		return &pendingProcessingError{reply.ErrorMessage}
	case "ERROR", "PROCESSING_FAILED":
		return errors.New(reply.ErrorMessage)
	}
	return fmt.Errorf("unexpected status when moving ref: %s", reply.Status)
}

func (r *remoteImpl) attachTags(ctx context.Context, pin common.Pin, tags []string) error {
	// Tags will be passed in the request body, not via URL.
	endpoint, err := tagsEndpoint(pin, nil)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		err = common.ValidateInstanceTag(tag)
		if err != nil {
			return err
		}
	}

	var request struct {
		Tags []string `json:"tags"`
	}
	request.Tags = tags

	var reply struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	err = r.makeRequest(ctx, endpoint, "POST", &request, &reply)
	if err != nil {
		return err
	}
	switch reply.Status {
	case "SUCCESS":
		return nil
	case "PROCESSING_NOT_FINISHED_YET":
		return &pendingProcessingError{reply.ErrorMessage}
	case "ERROR", "PROCESSING_FAILED":
		return errors.New(reply.ErrorMessage)
	}
	return fmt.Errorf("unexpected status when attaching tags: %s", reply.Status)
}

func (r *remoteImpl) listPackages(ctx context.Context, path string, recursive, showHidden bool) ([]string, []string, error) {
	endpoint, err := packageSearchEndpoint(path, recursive, showHidden)
	if err != nil {
		return nil, nil, err
	}
	var reply struct {
		Status       string   `json:"status"`
		ErrorMessage string   `json:"error_message"`
		Packages     []string `json:"packages"`
		Directories  []string `json:"directories"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		packages := reply.Packages
		directories := reply.Directories
		return packages, directories, nil
	case "ERROR":
		return nil, nil, errors.New(reply.ErrorMessage)
	}
	return nil, nil, fmt.Errorf("unexpected list packages status: %s", reply.Status)
}

func (r *remoteImpl) searchInstances(ctx context.Context, tag, packageName string) ([]common.Pin, error) {
	endpoint, err := packageSearchInstancesEndpoint(tag, packageName)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Status       string               `json:"status"`
		ErrorMessage string               `json:"error_message"`
		Instances    []packageInstanceMsg `json:"instances"`
	}
	err = r.makeRequest(ctx, endpoint, "GET", nil, &reply)
	if err != nil {
		return nil, err
	}
	switch reply.Status {
	case "SUCCESS":
		pins := make([]common.Pin, len(reply.Instances))
		for i, instance := range reply.Instances {
			pins[i] = common.Pin{instance.PackageName, instance.InstanceID}
		}
		return pins, nil
	case "ERROR":
		return nil, errors.New(reply.ErrorMessage)
	}
	return nil, fmt.Errorf("unexpected searchInstances status: %s", reply.Status)
}

////////////////////////////////////////////////////////////////////////////////

func packageEndpoint(packageName string) (string, error) {
	if err := common.ValidatePackageName(packageName); err != nil {
		return "", err
	}
	params := url.Values{}
	params.Add("package_name", packageName)
	return "repo/v1/package?" + params.Encode(), nil
}

func instanceEndpoint(pin common.Pin) (string, error) {
	if err := common.ValidatePin(pin); err != nil {
		return "", err
	}
	params := url.Values{}
	params.Add("package_name", pin.PackageName)
	params.Add("instance_id", pin.InstanceID)
	return "repo/v1/instance?" + params.Encode(), nil
}

func aclEndpoint(packagePath string) (string, error) {
	if err := common.ValidatePackageName(packagePath); err != nil {
		return "", err
	}
	params := url.Values{}
	params.Add("package_path", packagePath)
	return "repo/v1/acl?" + params.Encode(), nil
}

func refEndpoint(packageName, instanceID string, refs []string) (string, error) {
	if err := common.ValidatePackageName(packageName); err != nil {
		return "", err
	}
	if instanceID != "" {
		if err := common.ValidateInstanceID(instanceID); err != nil {
			return "", err
		}
	}
	for _, ref := range refs {
		if err := common.ValidatePackageRef(ref); err != nil {
			return "", err
		}
	}
	params := url.Values{}
	params.Add("package_name", packageName)
	if instanceID != "" {
		params.Add("instance_id", instanceID)
	}
	for _, ref := range refs {
		params.Add("ref", ref)
	}
	return "repo/v1/ref?" + params.Encode(), nil
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func packageSearchEndpoint(path string, recursive, showHidden bool) (string, error) {
	params := url.Values{}
	params.Add("path", path)
	params.Add("recursive", boolToString(recursive))
	params.Add("show_hidden", boolToString(showHidden))
	return "repo/v1/package/search?" + params.Encode(), nil
}

func packageSearchInstancesEndpoint(tag, packageName string) (string, error) {
	params := url.Values{}
	if packageName != "" {
		params.Add("package_name", packageName)
	}
	params.Add("tag", tag)
	return "repo/v1/instance/search?" + params.Encode(), nil
}

func tagsEndpoint(pin common.Pin, tags []string) (string, error) {
	if err := common.ValidatePin(pin); err != nil {
		return "", err
	}
	for _, tag := range tags {
		if err := common.ValidateInstanceTag(tag); err != nil {
			return "", err
		}
	}
	params := url.Values{}
	params.Add("package_name", pin.PackageName)
	params.Add("instance_id", pin.InstanceID)
	for _, tag := range tags {
		params.Add("tag", tag)
	}
	return "repo/v1/tags?" + params.Encode(), nil
}

// convertTimestamp coverts string with int64 timestamp in microseconds since
// to time.Time
func convertTimestamp(ts string) (time.Time, error) {
	i, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("unexpected timestamp value %q in the server response", ts)
	}
	return time.Unix(0, i*1000), nil
}
