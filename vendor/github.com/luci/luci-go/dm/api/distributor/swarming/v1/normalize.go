// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarmingV1

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// DefaultSwarmingPriority is the priority used if
// Parameters.scheduling.priority == 0.
const DefaultSwarmingPriority = 100

// Normalize normalizes and checks for input violations.
func (p *Parameters) Normalize() (err error) {
	if err = p.Scheduling.Normalize(); err != nil {
		return
	}
	if err = p.Meta.Normalize(); err != nil {
		return
	}
	if err = p.Job.Normalize(); err != nil {
		return
	}
	return
}

// Normalize normalizes and checks for input violations.
func (s *Parameters_Scheduling) Normalize() (err error) {
	if s.Priority > 255 {
		return errors.New("scheduling.priority > 256")
	}
	// Priority == 0 means default.
	if s.Priority == 0 {
		s.Priority = DefaultSwarmingPriority
	}
	for k, v := range s.Dimensions {
		if k == "" {
			return errors.New("scheduling.dimensions: empty dimension key")
		}
		if v == "" {
			return fmt.Errorf("scheduling.dimensions: dimension key %q with empty value", k)
		}
	}
	if s.IoTimeout.Duration() < 0 {
		return errors.New("scheduling.io_timeout: negative timeout not allowed")
	}
	return
}

// Normalize normalizes and checks for input violations.
func (m *Parameters_Meta) Normalize() (err error) {
	return
}

// Normalize normalizes and checks for input violations.
func (j *Parameters_Job) Normalize() (err error) {
	if err = j.Inputs.Normalize(); err != nil {
		return
	}
	if len(j.Command) == 0 {
		return errors.New("job.command: command is required")
	}
	for k := range j.Env {
		if k == "" {
			return errors.New("job.env: environment key is empty")
		}
	}
	return
}

// Normalize normalizes and checks for input violations.
func (i *Parameters_Job_Inputs) Normalize() (err error) {
	if len(i.Packages) == 0 && len(i.Isolated) == 0 {
		return errors.New(
			"job.inputs: at least one of packages and isolated must be specified")
	}
	if len(i.Packages) > 0 {
		if err = schemaHostURLValidate(i.CipdServer); err != nil {
			return fmt.Errorf("job.inputs.cipd_server: %s", err)
		}
		for i, p := range i.Packages {
			if p.Name == "" {
				return fmt.Errorf("job.inputs.packages[%d]: missing name", i)
			}
			if p.Version == "" {
				return fmt.Errorf("job.inputs.packages[%d]: missing version", i)
			}
		}
	}
	return
}

func schemaHostURLValidate(uStr string) error {
	if uStr == "" {
		return errors.New("required")
	}
	if strings.IndexFunc(uStr, unicode.IsUpper) != -1 {
		return errors.New("must be lower-case")
	}
	u, err := url.Parse(uStr)
	if err != nil {
		return fmt.Errorf("invalid url: %s", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if fmt.Sprintf("%s://%s", u.Scheme, u.Host) != uStr {
		return fmt.Errorf("unsupported url: %s", uStr)
	}
	return nil
}

// Normalize normalizes and checks for input violations.
func (c *Config) Normalize() (err error) {
	if err = c.Swarming.Normalize(); err != nil {
		return
	}
	if err = c.Isolate.Normalize(); err != nil {
		return
	}
	return
}

// Normalize normalizes and checks for input violations.
func (s *Config_Swarming) Normalize() (err error) {
	if err = schemaHostURLValidate(s.Url); err != nil {
		return fmt.Errorf("config.swarming.host: %s", err)
	}
	return
}

// Normalize normalizes and checks for input violations.
func (i *Config_Isolate) Normalize() (err error) {
	if err = schemaHostURLValidate(i.Url); err != nil {
		return fmt.Errorf("config.isolate.host: %s", err)
	}
	return
}
