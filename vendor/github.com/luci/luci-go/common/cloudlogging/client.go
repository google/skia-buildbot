// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
	cloudlog "google.golang.org/api/logging/v1beta3"
)

// DefaultResourceType is used by NewClient if ClientOptions doesn't specify
// ResourceType.
const (
	// GCEService is service name for GCE instances.
	GCEService = "compute.googleapis.com"

	// DefaultResourceType is used as the ResourceType value if Options doesn't
	// specify a ResourceType.
	DefaultResourceType = "machine"

	// WriteScope is the cloud logging OAuth write scope.
	WriteScope = "https://www.googleapis.com/auth/logging.write"
)

// CloudLoggingScopes is the set of OAuth scopes that are required to
// publish cloud logs via this package.
var CloudLoggingScopes = []string{
	WriteScope,
}

// Labels is a set of key/value data that is transmitted alongside a log entry.
type Labels map[string]string

// ClientOptions is passed to NewClient.
type ClientOptions struct {
	// ProjectID is Cloud project to sends logs to. Must be set.
	ProjectID string
	// LogID identifies what sort of log this is. Must be set.
	LogID string

	// Zone is the service name that the log will be registered under. Default is
	// GCEService.
	ServiceName string
	// Zone is the zone to attribute the log to. This can be empty.
	Zone string
	// ResourceType identifies a kind of entity that produces this log (e.g.
	// 'machine', 'master'). Default is DefaultResourceType.
	ResourceType string
	// ResourceID identifies exact instance of provided resource type (e.g
	// 'vm12-m4', 'master.chromium.fyi'). Default is machine hostname.
	ResourceID string
	// Region is the region to attribute the log to. This can be empty.
	Region string
	// Region is the user ID to attribute the log to. This can be empty.
	UserID string

	// UserAgent is an optional string appended to User-Agent HTTP header.
	UserAgent string

	// CommonLabels is a list of key/value labels that will be included with each
	// log message that is sent.
	CommonLabels Labels

	// ErrorWriter, if not nil, will be called when an error is encountered in
	// the Client iself. This is used to report errors because the client may
	// send messages asynchronously, so they will not be returned immediately,
	// as is the go convention.
	ErrorWriter func(string)
}

// Populate fills the Config with default values derived from the environment.
// Missing values will not be set.
//
// This includes:
// - GCE metadata querying.
func (o *ClientOptions) Populate() {
	if metadata.OnGCE() {
		get := func(f func() (string, error), val *string) {
			if v, err := f(); err == nil {
				*val = v
			}
		}

		o.ServiceName = GCEService
		get(metadata.ProjectID, &o.ProjectID)
		get(metadata.InstanceName, &o.ResourceType)
		get(metadata.InstanceID, &o.ResourceID)
		get(metadata.Zone, &o.Zone)
	}
	if o.ErrorWriter == nil {
		o.ErrorWriter = func(s string) {
			fmt.Fprintln(os.Stderr, s)
		}
	}
}

// Validate evaluates a Config and returns an error if it is invalid or
// incomplete.
func (o *ClientOptions) Validate() error {
	if o.ProjectID == "" {
		return errors.New("cloudlogging: You must supply a project name")
	}
	if o.LogID == "" {
		return errors.New("cloudlogging: You must supply a logs ID")
	}
	if o.ResourceType == "" {
		return errors.New("cloudlogging: You must supply a resource type")
	}
	return nil
}

// AddFlags adds logging flags to the supplied FlagSet.
func (o *ClientOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.LogID, "cloud-logging-logs-id", o.LogID,
		"For cloud logging, the log stream ID.")
	fs.StringVar(&o.ServiceName, "cloud-logging-service", o.ServiceName,
		"For cloud logging, the service name.")
	fs.StringVar(&o.ProjectID, "cloud-logging-project-name", o.ProjectID,
		"For cloud logging, the project name.")
	fs.StringVar(&o.ResourceType, "cloud-logging-resource-type", o.ResourceType,
		"For cloud logging, the instance name.")
	fs.StringVar(&o.ResourceID, "cloud-logging-resource-id", o.ResourceID,
		"For cloud logging, the instance ID.")
	fs.StringVar(&o.Region, "cloud-logging-region", o.Region,
		"For cloud logging, the region.")
	fs.StringVar(&o.UserID, "cloud-logging-user", o.UserID,
		"For cloud logging, the user ID.")
	fs.StringVar(&o.Zone, "cloud-logging-zone", o.Zone,
		"For cloud logging, the zone.")
}

// Client knows how to send entries to Cloud Logging log.
type Client interface {
	// PushEntries sends entries to Cloud Logging. No retries.
	PushEntries(entries []*Entry) error
}

// NewClient returns new object that knows how to push log entries to a single
// log in Cloud Logging.
func NewClient(opts ClientOptions, client *http.Client) (Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if opts.ResourceType == "" {
		opts.ResourceType = DefaultResourceType
	}
	if opts.ResourceID == "" {
		var err error
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		opts.ResourceID = hostname
	}
	if opts.LogID == "" {
		return nil, errors.New("cloudlogging: no LogID is provided")
	}

	service, err := cloudlog.New(client)
	if err != nil {
		return nil, err
	}
	if opts.UserAgent != "" {
		service.UserAgent = opts.UserAgent
	}

	c := clientImpl{
		ClientOptions: &opts,
		service:       cloudlog.NewProjectsLogsEntriesService(service),
		commonLabels:  make(map[string]string, len(opts.CommonLabels)),
	}
	for k, v := range opts.CommonLabels {
		c.commonLabels[k] = v
	}
	if c.ResourceType != "" {
		c.commonLabels["compute.googleapis.com/resource_type"] = c.ResourceType
	}
	if c.ResourceID != "" {
		c.commonLabels["compute.googleapis.com/resource_id"] = c.ResourceID
	}
	return &c, nil
}

type clientImpl struct {
	*ClientOptions

	service *cloudlog.ProjectsLogsEntriesService
	// These are passed to Cloud Logging API as is.
	commonLabels map[string]string
}

func (c *clientImpl) PushEntries(entries []*Entry) error {
	req := cloudlog.WriteLogEntriesRequest{
		CommonLabels: c.commonLabels,
		Entries:      make([]*cloudlog.LogEntry, 0, len(entries)),
	}
	for i, e := range entries {
		entry, err := e.cloudLogEntry(c.ClientOptions)
		if err != nil {
			c.writeError("Failed to build log for entry #%d: %v", i, err)
			return err
		}
		req.Entries = append(req.Entries, entry)
	}

	_, err := c.service.Write(c.ProjectID, c.LogID, &req).Do()
	if err != nil {
		c.writeError(err.Error())
	}
	return err
}

func (c *clientImpl) writeError(f string, args ...interface{}) {
	if c.ErrorWriter != nil {
		c.ErrorWriter(fmt.Sprintf(f, args...))
	}
}
