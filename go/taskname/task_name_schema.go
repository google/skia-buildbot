package taskname

import (
	"fmt"
	"strings"
)

//go:generate go run gen_schema.go

// TaskNameParser parses a builder/task name into its constituant parts
// See https://skia.googlesource.com/skia/+/master/infra/bots/recipe_modules/builder_name_schema/builder_name_schema.json
type TaskNameParser interface {
	ParseTaskName(name string) (map[string]string, error)
}

// taskNameParser fulfills the TaskNameParser interface. See gen_schema.go
// for more information on the two parts.
type taskNameParser struct {
	Schema map[string][]string

	TaskNameSep string
}

// DefaultTaskNameParser creates a TaskNameParser using the schema created by
// gen_schema.go.
func DefaultTaskNameParser() (TaskNameParser, error) {
	return &taskNameParser{
		Schema:      SCHEMA_FROM_GIT,
		TaskNameSep: SEPERATOR_FROM_GIT,
	}, nil
}

// See TaskNameParser for more information
func (b *taskNameParser) ParseTaskName(name string) (map[string]string, error) {
	split := strings.Split(name, b.TaskNameSep)
	if len(split) < 2 {
		return nil, fmt.Errorf("Invalid task name: %q", name)
	}
	role := split[0]
	split = split[1:]
	keys, ok := b.Schema[role]
	if !ok {
		return nil, fmt.Errorf("Invalid task name; %q is not a valid role.", role)
	}
	extraConfig := ""
	if len(split) == len(keys)+1 {
		extraConfig = split[len(split)-1]
		split = split[:len(split)-1]
	}
	if len(split) != len(keys) {
		return nil, fmt.Errorf("Invalid task name: %q has incorrect number of parts.", name)
	}
	rv := make(map[string]string, len(keys)+2)
	rv["role"] = role
	if extraConfig != "" {
		rv["extra_config"] = extraConfig
	}
	for i, k := range keys {
		rv[k] = split[i]
	}
	return rv, nil
}
