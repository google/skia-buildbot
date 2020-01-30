package config

import (
	"io"

	"go.skia.org/infra/go/skerr"
	"gopkg.in/yaml.v2"
)

/*
	Task Scheduler / Docker configuration.
*/

type Image struct {
	Name   string `yaml:"name"`
	Sha256 string `yaml:"sha256"`
}

type DockerBuildOutput struct {
	Image string `yaml:"image"`
	// If not provided, build the default target only.
	Target string `yaml:"target,omitempty"`
}

type DockerBuild struct {
	File      string                        `yaml:"file"`
	Base      string                        `yaml:"base"`
	Outputs   map[string]*DockerBuildOutput `yaml:"outputs"`
	BuildArgs map[string]string             `yaml:"build-args"`
}

type Artifact struct {
	Image  string `yaml:"image"`
	Source string `yaml:"source"`
	Mount  string `yaml:"mount"`
}

type DockerRun struct {
	Image     string               `yaml:"image"`
	Artifacts map[string]*Artifact `yaml:"artifacts,omitempty"`
	Command   []string             `yaml:"command"`
}

type RawCmd struct {
	Command []string `yaml:"command"`
}

type Task struct {
	Dimensions map[string]string `yaml:"dimensions"`

	// Exactly one of the following must be set.
	DockerBuild *DockerBuild `yaml:"docker-build"`
	DockerRun   *DockerRun   `yaml:"docker-run"`
	RawCmd      *RawCmd      `yaml:"raw-cmd"`
}

type Job struct {
	Tasks   []string `yaml:"tasks"`
	Trigger string   `yaml:"trigger"`
}

type Config struct {
	Version string            `yaml:"version"`
	Project string            `yaml:"project"`
	Images  map[string]*Image `yaml:"images"`
	Tasks   map[string]*Task  `yaml:"tasks"`
	Jobs    map[string]*Job   `yaml:"jobs"`
}

func Parse(r io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	dec.SetStrict(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse config")
	}
	return &cfg, nil
}
