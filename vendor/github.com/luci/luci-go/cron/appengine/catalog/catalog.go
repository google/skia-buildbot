// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package catalog implements a part that talks to luci-config service to fetch
// and parse job definitions. Catalog knows about all task types and can
// instantiate task.Manager's.
package catalog

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/schedule"
	"github.com/luci/luci-go/cron/appengine/task"
)

var (
	// jobIDRe is used to validate job ID field.
	jobIDRe = regexp.MustCompile(`^[0-9A-Za-z_\-]{1,100}$`)
)

// Catalog knows how to enumerate all cron job configs across all projects.
// Methods return errors.Transient on non-fatal errors. Any other error means
// that retry won't help.
type Catalog interface {
	// RegisterTaskManager registers a manager that knows how to deal with
	// a particular kind of tasks (as specified by its ProtoMessageType method).
	RegisterTaskManager(m task.Manager) error

	// GetTaskManager takes pointer to a proto message describing some task config
	// and returns corresponding TaskManager implementation (or nil).
	GetTaskManager(m proto.Message) task.Manager

	// UnmarshalTask takes serialized task proto (as in Definition.Task),
	// unmarshals and validates it and returns proto.Message that represent
	// the task to run. It can be passed to corresponding task.Manager.
	UnmarshalTask(task []byte) (proto.Message, error)

	// GetAllProjects returns a list of all known project ids.
	//
	// It assumes there's config.Interface implementation installed in
	// the context, will panic if it's not there.
	GetAllProjects(c context.Context) ([]string, error)

	// GetProjectJobs returns a list of cron jobs defined within a project or
	// empty list if no such project.
	//
	// It assumes there's config.Interface implementation installed in
	// the context, will panic if it's not there.
	GetProjectJobs(c context.Context, projectID string) ([]Definition, error)
}

// Definition wraps serialized definition of a cron job fetched from the config.
type Definition struct {
	// JobID is globally unique job identifier: "<ProjectID>/<JobName>".
	JobID string

	// Revision is config revision this definition was fetched from.
	Revision string

	// RevisionURL is URL to human readable page with config file.
	RevisionURL string

	// Schedule is cron job schedule in regular cron expression format.
	Schedule string

	// Task is serialized representation of cron job. It can be fed back to
	// Catalog.UnmarshalTask(...) to get proto.Message describing the task.
	Task []byte
}

// New returns implementation of Catalog.
//
// If configFileName is not "", it specifies name of *.cfg file to read cron job
// definition from. If it is "", <gae-app-id>.cfg will be used instead.
func New(configFileName string) Catalog {
	return &catalog{
		managers:       map[reflect.Type]task.Manager{},
		configFileName: configFileName,
	}
}

type catalog struct {
	managers       map[reflect.Type]task.Manager
	configFileName string
}

func (cat *catalog) RegisterTaskManager(m task.Manager) error {
	prototype := m.ProtoMessageType()
	typ := reflect.TypeOf(prototype)
	if typ == nil || typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("expecting pointer to a struct, got %T instead", prototype)
	}
	if _, ok := cat.managers[typ]; ok {
		return fmt.Errorf("task kind %T is already registered", prototype)
	}
	cat.managers[typ] = m
	return nil
}

func (cat *catalog) GetTaskManager(msg proto.Message) task.Manager {
	return cat.managers[reflect.TypeOf(msg)]
}

func (cat *catalog) UnmarshalTask(task []byte) (proto.Message, error) {
	msg := messages.Task{}
	if err := proto.Unmarshal(task, &msg); err != nil {
		return nil, err
	}
	return cat.extractTaskProto(&msg)
}

func (cat *catalog) GetAllProjects(c context.Context) ([]string, error) {
	// Enumerate all projects that have cron.cfg. Do not fetch actual configs yet.
	cfgs, err := config.GetProjectConfigs(c, cat.configFile(c), true)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		// ConfigSet must be "projects/<project-id>". Verify that.
		chunks := strings.Split(cfg.ConfigSet, "/")
		if len(chunks) != 2 || chunks[0] != "projects" {
			logging.Warningf(c, "Unexpected ConfigSet: %s", cfg.ConfigSet)
		} else {
			out = append(out, chunks[1])
		}
	}
	return out, nil
}

func (cat *catalog) GetProjectJobs(c context.Context, projectID string) ([]Definition, error) {
	configSetURL, err := config.GetConfigSetLocation(c, "projects/"+projectID)
	if err != nil {
		return nil, err
	}
	rawCfg, err := config.GetConfig(c, "projects/"+projectID, cat.configFile(c), false)
	if err == config.ErrNoConfig {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	revisionURL := getRevisionURL(configSetURL, rawCfg.Revision, rawCfg.Path)
	if revisionURL != "" {
		logging.Infof(c, "Importing %s", revisionURL)
	}
	cfg := messages.ProjectConfig{}
	if err = proto.UnmarshalText(rawCfg.Content, &cfg); err != nil {
		return nil, err
	}
	out := make([]Definition, 0, len(cfg.Job))
	for _, job := range cfg.Job {
		if job.GetDisabled() {
			continue
		}
		id := "(nil)"
		if job.Id != nil {
			id = *job.Id
		}
		if err = cat.validateJobProto(job); err != nil {
			logging.Errorf(c, "Invalid job definition %s/%s: %s", projectID, id, err)
			continue
		}
		packed, err := proto.Marshal(job.Task)
		if err != nil {
			logging.Errorf(c, "Failed to marshal the task: %s/%s: %s", projectID, id, err)
			continue
		}
		out = append(out, Definition{
			JobID:       fmt.Sprintf("%s/%s", projectID, *job.Id),
			Revision:    rawCfg.Revision,
			RevisionURL: revisionURL,
			Schedule:    *job.Schedule,
			Task:        packed,
		})
	}
	return out, nil
}

// configFile returns a name of *.cfg file (inside project's config set) with
// all cron job definitions for a project. This file contains text-encoded
// cron.ProjectConfig message.
func (cat *catalog) configFile(c context.Context) string {
	if cat.configFileName != "" {
		return cat.configFileName
	}
	return info.Get(c).AppID() + ".cfg"
}

// getRevisionURL derives URL to a config file revision, given URL of a config
// set (e.g. "https://host/repo/+/ref").
func getRevisionURL(configSetURL *url.URL, rev, path string) string {
	if path == "" || configSetURL == nil {
		return ""
	}
	// repoAndRef is something like /infra/experimental/+/refs/heads/infra/config
	repoAndRef := configSetURL.Path
	plus := strings.Index(repoAndRef, "/+/")
	if plus == -1 {
		return ""
	}
	// Replace ref in path with revision, add path.
	urlCpy := *configSetURL
	urlCpy.Path = fmt.Sprintf("%s/+/%s/%s", repoAndRef[:plus], rev, path)
	return urlCpy.String()
}

// validateJobProto verifies that messages.Job protobuf message makes sense.
func (cat *catalog) validateJobProto(j *messages.Job) error {
	if j == nil {
		return fmt.Errorf("job must be specified")
	}
	if j.Id == nil {
		return fmt.Errorf("missing 'id' field'")
	}
	if !jobIDRe.MatchString(*j.Id) {
		return fmt.Errorf("%q is not valid value for 'id' field", *j.Id)
	}
	if j.Schedule == nil {
		return fmt.Errorf("missing 'schedule' field")
	}
	if _, err := schedule.Parse(*j.Schedule, 0); err != nil {
		return fmt.Errorf("%s is not valid value for 'schedule' field - %s", *j.Schedule, err)
	}
	_, err := cat.extractTaskProto(j.Task)
	return err
}

// extractTaskProto verifies that messages.Task protobuf message makes sense. It
// ensures only one of its fields is set, validated that field and returns it.
func (cat *catalog) extractTaskProto(t *messages.Task) (proto.Message, error) {
	if t == nil {
		return nil, fmt.Errorf("missing 'task' field")
	}
	var taskDef *reflect.Value
	v := reflect.ValueOf(*t)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		// Skip unset fields (nil) and XXX_unrecognized (it has Slice kind).
		if field.IsNil() || field.Type().Kind() == reflect.Slice {
			continue
		}
		if taskDef != nil {
			return nil, fmt.Errorf(
				"only one field must be set, at least two are given (%T and %T)",
				taskDef.Interface(), field.Interface())
		}
		taskDef = &field
	}
	if taskDef == nil {
		return nil, fmt.Errorf("at least one field must be set")
	}
	taskMsg := taskDef.Interface().(proto.Message)
	taskMan := cat.GetTaskManager(taskMsg)
	if taskMan == nil {
		return nil, fmt.Errorf("unknown task type: %T", taskMsg)
	}
	if err := taskMan.ValidateProtoMessage(taskMsg); err != nil {
		return nil, err
	}
	return taskMsg, nil
}
