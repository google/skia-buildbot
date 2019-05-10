package env

import (
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/dirs"
)

const (
	// PATH_PLACEHOLDER is a placeholder for any existing value of PATH,
	// used when merging environments to avoid overriding the PATH
	// altogether.
	PATH_PLACEHOLDER = "%(PATH)s"

	// PATH_VAR represents the PATH environment variable.
	PATH_VAR = "PATH"
)

type Env []string

// Merge the second Env into this one, returning a new Env with the original
// unchanged. Variables in the second Env override those in the first, except
// for PATH, which is merged. If PATH defined by the second Env contains
// %(PATH)s, then the result is the PATH from the second Env with PATH from
// the first Env inserted in place of %(PATH)s. Otherwise, the PATH from the
// second Env is appended to the PATH from the first Env.
func (e Env) Merge(other Env) Env {
	m := make(map[string]string, len(e))
	for _, kv := range e {
		split := strings.SplitN(kv, "=", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid env var: %s", kv)
			continue
		}
		m[split[0]] = split[1]
	}
	for _, kv := range other {
		split := strings.SplitN(kv, "=", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid env var: %s", kv)
			continue
		}
		k, v := split[0], split[1]
		if existing, ok := m[k]; ok && k == PATH_VAR {
			if strings.Contains(v, PATH_PLACEHOLDER) {
				v = strings.Replace(v, PATH_PLACEHOLDER, existing, -1)
			} else {
				v = existing + string(os.PathListSeparator) + v
			}
		}
		m[k] = v
	}
	rv := make([]string, 0, len(m))
	for k, v := range m {
		rv = append(rv, fmt.Sprintf("%s=%s", k, v))
	}
	return rv
}

func Base(workdir string) Env {
	return []string{
		"CHROME_HEADLESS=1",
		"GIT_USER_AGENT=git/1.9.1", // I don't think this version matters.
		fmt.Sprintf("SKIABOT_TEST_DEPOT_TOOLS=%s", dirs.DepotTools(workdir)),
		os.Getenv(PATH_VAR),
	}
}
