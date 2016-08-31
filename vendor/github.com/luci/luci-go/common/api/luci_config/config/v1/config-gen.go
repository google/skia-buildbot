// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package config provides access to the Configuration Service.
//
// Usage example:
//
//   import "github.com/luci/luci-go/common/api/luci_config/config/v1"
//   ...
//   configService, err := config.New(oauthHttpClient)
package config // import "github.com/luci/luci-go/common/api/luci_config/config/v1"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	context "golang.org/x/net/context"
	ctxhttp "golang.org/x/net/context/ctxhttp"
	gensupport "google.golang.org/api/gensupport"
	googleapi "google.golang.org/api/googleapi"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

const apiId = "config:v1"
const apiName = "config"
const apiVersion = "v1"
const basePath = "http://localhost:8080/_ah/api/config/v1/"

// OAuth2 scopes used by this API.
const (
	// View your email address
	UserinfoEmailScope = "https://www.googleapis.com/auth/userinfo.email"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

type ComponentsConfigEndpointValidationMessage struct {
	// Possible values:
	//   "CRITICAL"
	//   "DEBUG"
	//   "ERROR"
	//   "INFO"
	//   "WARNING"
	Severity string `json:"severity,omitempty"`

	Text string `json:"text,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Severity") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *ComponentsConfigEndpointValidationMessage) MarshalJSON() ([]byte, error) {
	type noMethod ComponentsConfigEndpointValidationMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

// LuciConfigConfigSet: Describes a config set.
type LuciConfigConfigSet struct {
	ConfigSet string `json:"config_set,omitempty"`

	LastImportAttempt *LuciConfigConfigSetImportAttempt `json:"last_import_attempt,omitempty"`

	Location string `json:"location,omitempty"`

	Revision *LuciConfigRevision `json:"revision,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ConfigSet") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigConfigSet) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigConfigSet
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigConfigSetImportAttempt struct {
	Message string `json:"message,omitempty"`

	Revision *LuciConfigRevision `json:"revision,omitempty"`

	Success bool `json:"success,omitempty"`

	Timestamp int64 `json:"timestamp,omitempty,string"`

	ValidationMessages []*ComponentsConfigEndpointValidationMessage `json:"validation_messages,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Message") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigConfigSetImportAttempt) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigConfigSetImportAttempt
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetConfigByHashResponseMessage struct {
	Content string `json:"content,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Content") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetConfigByHashResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetConfigByHashResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetConfigMultiResponseMessage struct {
	Configs []*LuciConfigGetConfigMultiResponseMessageConfigEntry `json:"configs,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Configs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetConfigMultiResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetConfigMultiResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetConfigMultiResponseMessageConfigEntry struct {
	ConfigSet string `json:"config_set,omitempty"`

	Content string `json:"content,omitempty"`

	ContentHash string `json:"content_hash,omitempty"`

	Revision string `json:"revision,omitempty"`

	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ConfigSet") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetConfigMultiResponseMessageConfigEntry) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetConfigMultiResponseMessageConfigEntry
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetConfigResponseMessage struct {
	Content string `json:"content,omitempty"`

	ContentHash string `json:"content_hash,omitempty"`

	Revision string `json:"revision,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Content") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetConfigResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetConfigResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetConfigSetsResponseMessage struct {
	// ConfigSets: Describes a config set.
	ConfigSets []*LuciConfigConfigSet `json:"config_sets,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ConfigSets") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetConfigSetsResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetConfigSetsResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetMappingResponseMessage struct {
	Mappings []*LuciConfigGetMappingResponseMessageMapping `json:"mappings,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Mappings") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetMappingResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetMappingResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetMappingResponseMessageMapping struct {
	ConfigSet string `json:"config_set,omitempty"`

	Location string `json:"location,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ConfigSet") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetMappingResponseMessageMapping) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetMappingResponseMessageMapping
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetProjectsResponseMessage struct {
	Projects []*LuciConfigProject `json:"projects,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Projects") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetProjectsResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetProjectsResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetRefsResponseMessage struct {
	Refs []*LuciConfigGetRefsResponseMessageRef `json:"refs,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Refs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetRefsResponseMessage) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetRefsResponseMessage
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigGetRefsResponseMessageRef struct {
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigGetRefsResponseMessageRef) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigGetRefsResponseMessageRef
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigProject struct {
	Id string `json:"id,omitempty"`

	Name string `json:"name,omitempty"`

	// Possible values:
	//   "GITILES"
	RepoType string `json:"repo_type,omitempty"`

	RepoUrl string `json:"repo_url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Id") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigProject) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigProject
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

type LuciConfigRevision struct {
	CommitterEmail string `json:"committer_email,omitempty"`

	Id string `json:"id,omitempty"`

	Timestamp int64 `json:"timestamp,omitempty,string"`

	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CommitterEmail") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`
}

func (s *LuciConfigRevision) MarshalJSON() ([]byte, error) {
	type noMethod LuciConfigRevision
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields)
}

// method id "config.get_config":

type GetConfigCall struct {
	s            *Service
	configSet    string
	path         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetConfig: Gets a config file.
func (s *Service) GetConfig(configSet string, path string) *GetConfigCall {
	c := &GetConfigCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.configSet = configSet
	c.path = path
	return c
}

// HashOnly sets the optional parameter "hash_only":
func (c *GetConfigCall) HashOnly(hashOnly bool) *GetConfigCall {
	c.urlParams_.Set("hash_only", fmt.Sprint(hashOnly))
	return c
}

// Revision sets the optional parameter "revision":
func (c *GetConfigCall) Revision(revision string) *GetConfigCall {
	c.urlParams_.Set("revision", revision)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetConfigCall) Fields(s ...googleapi.Field) *GetConfigCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetConfigCall) IfNoneMatch(entityTag string) *GetConfigCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetConfigCall) Context(ctx context.Context) *GetConfigCall {
	c.ctx_ = ctx
	return c
}

func (c *GetConfigCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "config_sets/{config_set}/config/{path}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"config_set": c.configSet,
		"path":       c.path,
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

// Do executes the "config.get_config" call.
// Exactly one of *LuciConfigGetConfigResponseMessage or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *LuciConfigGetConfigResponseMessage.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *GetConfigCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetConfigResponseMessage, error) {
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
	ret := &LuciConfigGetConfigResponseMessage{
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
	//   "description": "Gets a config file.",
	//   "httpMethod": "GET",
	//   "id": "config.get_config",
	//   "parameterOrder": [
	//     "config_set",
	//     "path"
	//   ],
	//   "parameters": {
	//     "config_set": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "hash_only": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "path": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "revision": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "config_sets/{config_set}/config/{path}",
	//   "response": {
	//     "$ref": "LuciConfigGetConfigResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_config_by_hash":

type GetConfigByHashCall struct {
	s            *Service
	contentHash  string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetConfigByHash: Gets a config file by its hash.
func (s *Service) GetConfigByHash(contentHash string) *GetConfigByHashCall {
	c := &GetConfigByHashCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.contentHash = contentHash
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetConfigByHashCall) Fields(s ...googleapi.Field) *GetConfigByHashCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetConfigByHashCall) IfNoneMatch(entityTag string) *GetConfigByHashCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetConfigByHashCall) Context(ctx context.Context) *GetConfigByHashCall {
	c.ctx_ = ctx
	return c
}

func (c *GetConfigByHashCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "config/{content_hash}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"content_hash": c.contentHash,
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

// Do executes the "config.get_config_by_hash" call.
// Exactly one of *LuciConfigGetConfigByHashResponseMessage or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *LuciConfigGetConfigByHashResponseMessage.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *GetConfigByHashCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetConfigByHashResponseMessage, error) {
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
	ret := &LuciConfigGetConfigByHashResponseMessage{
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
	//   "description": "Gets a config file by its hash.",
	//   "httpMethod": "GET",
	//   "id": "config.get_config_by_hash",
	//   "parameterOrder": [
	//     "content_hash"
	//   ],
	//   "parameters": {
	//     "content_hash": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "config/{content_hash}",
	//   "response": {
	//     "$ref": "LuciConfigGetConfigByHashResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_config_sets":

type GetConfigSetsCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetConfigSets: Returns config sets.
func (s *Service) GetConfigSets() *GetConfigSetsCall {
	c := &GetConfigSetsCall{s: s, urlParams_: make(gensupport.URLParams)}
	return c
}

// ConfigSet sets the optional parameter "config_set":
func (c *GetConfigSetsCall) ConfigSet(configSet string) *GetConfigSetsCall {
	c.urlParams_.Set("config_set", configSet)
	return c
}

// IncludeLastImportAttempt sets the optional parameter
// "include_last_import_attempt":
func (c *GetConfigSetsCall) IncludeLastImportAttempt(includeLastImportAttempt bool) *GetConfigSetsCall {
	c.urlParams_.Set("include_last_import_attempt", fmt.Sprint(includeLastImportAttempt))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetConfigSetsCall) Fields(s ...googleapi.Field) *GetConfigSetsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetConfigSetsCall) IfNoneMatch(entityTag string) *GetConfigSetsCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetConfigSetsCall) Context(ctx context.Context) *GetConfigSetsCall {
	c.ctx_ = ctx
	return c
}

func (c *GetConfigSetsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "config-sets")
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

// Do executes the "config.get_config_sets" call.
// Exactly one of *LuciConfigGetConfigSetsResponseMessage or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *LuciConfigGetConfigSetsResponseMessage.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *GetConfigSetsCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetConfigSetsResponseMessage, error) {
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
	ret := &LuciConfigGetConfigSetsResponseMessage{
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
	//   "description": "Returns config sets.",
	//   "httpMethod": "GET",
	//   "id": "config.get_config_sets",
	//   "parameters": {
	//     "config_set": {
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "include_last_import_attempt": {
	//       "location": "query",
	//       "type": "boolean"
	//     }
	//   },
	//   "path": "config-sets",
	//   "response": {
	//     "$ref": "LuciConfigGetConfigSetsResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_mapping":

type GetMappingCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetMapping: DEPRECATED. Use get_config_sets.
func (s *Service) GetMapping() *GetMappingCall {
	c := &GetMappingCall{s: s, urlParams_: make(gensupport.URLParams)}
	return c
}

// ConfigSet sets the optional parameter "config_set":
func (c *GetMappingCall) ConfigSet(configSet string) *GetMappingCall {
	c.urlParams_.Set("config_set", configSet)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetMappingCall) Fields(s ...googleapi.Field) *GetMappingCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetMappingCall) IfNoneMatch(entityTag string) *GetMappingCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetMappingCall) Context(ctx context.Context) *GetMappingCall {
	c.ctx_ = ctx
	return c
}

func (c *GetMappingCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "mapping")
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

// Do executes the "config.get_mapping" call.
// Exactly one of *LuciConfigGetMappingResponseMessage or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *LuciConfigGetMappingResponseMessage.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *GetMappingCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetMappingResponseMessage, error) {
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
	ret := &LuciConfigGetMappingResponseMessage{
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
	//   "description": "DEPRECATED. Use get_config_sets.",
	//   "httpMethod": "GET",
	//   "id": "config.get_mapping",
	//   "parameters": {
	//     "config_set": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "mapping",
	//   "response": {
	//     "$ref": "LuciConfigGetMappingResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_project_configs":

type GetProjectConfigsCall struct {
	s            *Service
	path         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetProjectConfigs: Gets configs in all project config sets.
func (s *Service) GetProjectConfigs(path string) *GetProjectConfigsCall {
	c := &GetProjectConfigsCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.path = path
	return c
}

// HashesOnly sets the optional parameter "hashes_only":
func (c *GetProjectConfigsCall) HashesOnly(hashesOnly bool) *GetProjectConfigsCall {
	c.urlParams_.Set("hashes_only", fmt.Sprint(hashesOnly))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetProjectConfigsCall) Fields(s ...googleapi.Field) *GetProjectConfigsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetProjectConfigsCall) IfNoneMatch(entityTag string) *GetProjectConfigsCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetProjectConfigsCall) Context(ctx context.Context) *GetProjectConfigsCall {
	c.ctx_ = ctx
	return c
}

func (c *GetProjectConfigsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "configs/projects/{path}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"path": c.path,
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

// Do executes the "config.get_project_configs" call.
// Exactly one of *LuciConfigGetConfigMultiResponseMessage or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *LuciConfigGetConfigMultiResponseMessage.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *GetProjectConfigsCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetConfigMultiResponseMessage, error) {
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
	ret := &LuciConfigGetConfigMultiResponseMessage{
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
	//   "description": "Gets configs in all project config sets.",
	//   "httpMethod": "GET",
	//   "id": "config.get_project_configs",
	//   "parameterOrder": [
	//     "path"
	//   ],
	//   "parameters": {
	//     "hashes_only": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "path": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "configs/projects/{path}",
	//   "response": {
	//     "$ref": "LuciConfigGetConfigMultiResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_projects":

type GetProjectsCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetProjects: Gets list of registered projects. The project list is
// stored in services/luci-config:projects.cfg.
func (s *Service) GetProjects() *GetProjectsCall {
	c := &GetProjectsCall{s: s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetProjectsCall) Fields(s ...googleapi.Field) *GetProjectsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetProjectsCall) IfNoneMatch(entityTag string) *GetProjectsCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetProjectsCall) Context(ctx context.Context) *GetProjectsCall {
	c.ctx_ = ctx
	return c
}

func (c *GetProjectsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "projects")
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

// Do executes the "config.get_projects" call.
// Exactly one of *LuciConfigGetProjectsResponseMessage or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *LuciConfigGetProjectsResponseMessage.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *GetProjectsCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetProjectsResponseMessage, error) {
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
	ret := &LuciConfigGetProjectsResponseMessage{
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
	//   "description": "Gets list of registered projects. The project list is stored in services/luci-config:projects.cfg.",
	//   "httpMethod": "GET",
	//   "id": "config.get_projects",
	//   "path": "projects",
	//   "response": {
	//     "$ref": "LuciConfigGetProjectsResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_ref_configs":

type GetRefConfigsCall struct {
	s            *Service
	path         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetRefConfigs: Gets configs in all ref config sets.
func (s *Service) GetRefConfigs(path string) *GetRefConfigsCall {
	c := &GetRefConfigsCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.path = path
	return c
}

// HashesOnly sets the optional parameter "hashes_only":
func (c *GetRefConfigsCall) HashesOnly(hashesOnly bool) *GetRefConfigsCall {
	c.urlParams_.Set("hashes_only", fmt.Sprint(hashesOnly))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetRefConfigsCall) Fields(s ...googleapi.Field) *GetRefConfigsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetRefConfigsCall) IfNoneMatch(entityTag string) *GetRefConfigsCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetRefConfigsCall) Context(ctx context.Context) *GetRefConfigsCall {
	c.ctx_ = ctx
	return c
}

func (c *GetRefConfigsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "configs/refs/{path}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"path": c.path,
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

// Do executes the "config.get_ref_configs" call.
// Exactly one of *LuciConfigGetConfigMultiResponseMessage or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *LuciConfigGetConfigMultiResponseMessage.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *GetRefConfigsCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetConfigMultiResponseMessage, error) {
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
	ret := &LuciConfigGetConfigMultiResponseMessage{
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
	//   "description": "Gets configs in all ref config sets.",
	//   "httpMethod": "GET",
	//   "id": "config.get_ref_configs",
	//   "parameterOrder": [
	//     "path"
	//   ],
	//   "parameters": {
	//     "hashes_only": {
	//       "location": "query",
	//       "type": "boolean"
	//     },
	//     "path": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "configs/refs/{path}",
	//   "response": {
	//     "$ref": "LuciConfigGetConfigMultiResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.get_refs":

type GetRefsCall struct {
	s            *Service
	projectId    string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
}

// GetRefs: Gets list of refs of a project.
func (s *Service) GetRefs(projectId string) *GetRefsCall {
	c := &GetRefsCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.projectId = projectId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *GetRefsCall) Fields(s ...googleapi.Field) *GetRefsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *GetRefsCall) IfNoneMatch(entityTag string) *GetRefsCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *GetRefsCall) Context(ctx context.Context) *GetRefsCall {
	c.ctx_ = ctx
	return c
}

func (c *GetRefsCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "projects/{project_id}/refs")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"project_id": c.projectId,
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

// Do executes the "config.get_refs" call.
// Exactly one of *LuciConfigGetRefsResponseMessage or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *LuciConfigGetRefsResponseMessage.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *GetRefsCall) Do(opts ...googleapi.CallOption) (*LuciConfigGetRefsResponseMessage, error) {
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
	ret := &LuciConfigGetRefsResponseMessage{
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
	//   "description": "Gets list of refs of a project.",
	//   "httpMethod": "GET",
	//   "id": "config.get_refs",
	//   "parameterOrder": [
	//     "project_id"
	//   ],
	//   "parameters": {
	//     "project_id": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "projects/{project_id}/refs",
	//   "response": {
	//     "$ref": "LuciConfigGetRefsResponseMessage"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "config.reimport":

type ReimportCall struct {
	s          *Service
	urlParams_ gensupport.URLParams
	ctx_       context.Context
}

// Reimport: Reimports a config set.
func (s *Service) Reimport(configSet string) *ReimportCall {
	c := &ReimportCall{s: s, urlParams_: make(gensupport.URLParams)}
	c.urlParams_.Set("config_set", configSet)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ReimportCall) Fields(s ...googleapi.Field) *ReimportCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ReimportCall) Context(ctx context.Context) *ReimportCall {
	c.ctx_ = ctx
	return c
}

func (c *ReimportCall) doRequest(alt string) (*http.Response, error) {
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "reimport")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", c.s.userAgent())
	if c.ctx_ != nil {
		return ctxhttp.Do(c.ctx_, c.s.client, req)
	}
	return c.s.client.Do(req)
}

// Do executes the "config.reimport" call.
func (c *ReimportCall) Do(opts ...googleapi.CallOption) error {
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
	//   "description": "Reimports a config set.",
	//   "httpMethod": "POST",
	//   "id": "config.reimport",
	//   "parameterOrder": [
	//     "config_set"
	//   ],
	//   "parameters": {
	//     "config_set": {
	//       "location": "query",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "reimport",
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}
