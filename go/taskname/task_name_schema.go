package taskname

import (
	"fmt"
	"strings"
)

//go:generate bazelisk run //:go -- run gen_schema.go

// TaskNameParser parses a builder/task name into its constituent parts
// See https://skia.googlesource.com/skia/+show/master/infra/bots/recipe_modules/builder_name_schema/builder_name_schema.json
type TaskNameParser interface {
	ParseTaskName(name string) (map[string]string, error)
}

// Schema is a sub-struct of taskNameParser.
type Schema struct {
	// Key names, in order, for this role.
	Keys []string `json:"keys"`

	// Optional key names, in order, for this role. These come after Keys
	// and RecurseRoles in task names.
	OptionalKeys []string `json:"optional_keys"`

	// Recursively apply one of these roll names to the schema (eg.
	// "Upload-Test-*").  The keys from the sub-role are applied to this
	// role.
	RecurseRoles []string `json:"recurse_roles"`
}

// taskNameParser fulfills the TaskNameParser interface. See gen_schema.go
// for more information on the two parts.
type taskNameParser struct {
	Schema map[string]*Schema `json:"builder_name_schema"`
	Sep    string             `json:"builder_name_sep"`
}

// DefaultTaskNameParser creates a TaskNameParser using the schema created by
// gen_schema.go.
func DefaultTaskNameParser() TaskNameParser {
	return &taskNameParser{
		Schema: SCHEMA_FROM_GIT,
		Sep:    SEPARATOR_FROM_GIT,
	}
}

// See TaskNameParser for more information
func (b *taskNameParser) ParseTaskName(name string) (map[string]string, error) {
	popFront := func(items []string) (string, []string, error) {
		if len(items) == 0 {
			return "", nil, fmt.Errorf("Invalid task name: %s (not enough parts)", name)
		}
		return items[0], items[1:], nil
	}

	result := map[string]string{}

	var parse func(int, string, []string) ([]string, error)
	parse = func(depth int, role string, parts []string) ([]string, error) {
		s, ok := b.Schema[role]
		if !ok {
			return nil, fmt.Errorf("Invalid task name; %q is not a valid role.", role)
		}
		if depth == 0 {
			result["role"] = role
		} else {
			result[fmt.Sprintf("sub-role-%d", depth)] = role
		}
		var err error
		for _, key := range s.Keys {
			var value string
			value, parts, err = popFront(parts)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		for _, subRole := range s.RecurseRoles {
			if len(parts) > 0 && parts[0] == subRole {
				parts, err = parse(depth+1, parts[0], parts[1:])
				if err != nil {
					return nil, err
				}
			}
		}
		for _, key := range s.OptionalKeys {
			if len(parts) > 0 {
				var value string
				value, parts, err = popFront(parts)
				if err != nil {
					return nil, err
				}
				result[key] = value
			}
		}
		if len(parts) > 0 {
			return nil, fmt.Errorf("Invalid task name: %s (too many parts)", name)
		}
		return parts, nil
	}

	split := strings.Split(name, b.Sep)
	if len(split) < 2 {
		return nil, fmt.Errorf("Invalid task name: %s (not enough parts)", name)
	}
	role := split[0]
	split = split[1:]
	_, err := parse(0, role, split)
	return result, err
}
