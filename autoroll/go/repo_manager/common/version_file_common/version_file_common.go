package version_file_common

import (
	"context"
	"path"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/pyl"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/bazel"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/readme_chromium"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func getUsingRegex(dep *config.VersionFileConfig_File, contents string) ([]string, []string, error) {
	re, err := regexp.Compile(dep.Regex)
	if err != nil {
		// We check the regex when we validate the config, so in theory we
		// shouldn't run into this.
		return nil, nil, skerr.Wrap(err)
	}
	matches := re.FindAllStringSubmatch(contents, -1)
	if len(matches) == 0 {
		return nil, nil, skerr.Fmt("no match found for regex `%s` in:\n%s", dep.Regex, contents)
	}
	var fullMatches []string
	var revisions []string
	for _, match := range matches {
		// We expect the regex to contain exactly one capture group, so the
		// match slice should contain two elements: entire matched text, and the
		// contents of the capture group.
		if len(match) != 2 {
			return nil, nil, skerr.Fmt("wrong number of matches found for regex `%s`; expected two; Contents:\n%s", dep.Regex, contents)
		}
		fullMatches = append(fullMatches, match[0])
		revisions = append(revisions, match[1])
	}
	return fullMatches, revisions, nil
}

// getPinnedRevInFile reads the given file contents to find the pinned revision.
func getPinnedRevInFile(id string, file *config.VersionFileConfig_File, contents string) (string, map[string]string, error) {
	if file.Regex != "" {
		_, revisions, err := getUsingRegex(file, contents)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return revisions[0], nil, nil
	} else if file.Path == deps_parser.DepsFileName {
		depsEntry, err := deps_parser.GetDep(contents, id)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return depsEntry.Version, nil, nil
	} else if path.Base(file.Path) == readme_chromium.FileName {
		files, err := readme_chromium.ParseMulti(contents)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		rcf, err := findMatchingReadmeChromiumFile(id, file.Path, files)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return rcf.Revision, nil, nil
	} else if strings.HasSuffix(file.Path, ".pyl") {
		rv, err := pyl.Get(contents, id)
		return rv, nil, skerr.Wrap(err)
	} else if bazel.IsBazelFile(file.Path) {
		entry, err := bazel.GetDep(contents, bazel.DependencyID(id))
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		var meta map[string]string
		if metaDep, ok := entry.(*bazel.MetaDependency); ok {
			meta = metaDep.SHA256
		}
		return entry.GetVersion(), meta, nil
	} else {
		return strings.TrimSpace(contents), nil, nil
	}
}

// GetPinnedRevs reads files using the given GetFileFunc to retrieve the given
// pinned revisions. File retrievals are cached for efficiency.
func GetPinnedRevs(ctx context.Context, deps []*config.VersionFileConfig, getFile GetFileFunc) (map[string]string, map[string]map[string]string, error) {
	rv := make(map[string]string, len(deps))
	rvMeta := make(map[string]map[string]string, len(deps))
	// Cache files in case multiple dependencies are versioned in
	// the same file, eg. DEPS.
	cache := map[string]string{}
	for _, dep := range deps {
		if len(dep.File) == 0 {
			return nil, nil, skerr.Fmt("no configured files to read from")
		}
		contents, ok := cache[dep.File[0].Path]
		if !ok {
			var err error
			contents, err = getFile(ctx, dep.File[0].Path)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			cache[dep.File[0].Path] = contents
		}
		version, data, err := getPinnedRevInFile(dep.Id, dep.File[0], contents)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		rv[dep.Id] = version
		rvMeta[dep.Id] = data
	}
	return rv, rvMeta, nil
}

// setPinnedRevInFile updates the given dependency pin in the given file, returning
// the new contents.
func setPinnedRevInFile(id string, dep *config.VersionFileConfig_File, newRev *revision.Revision, oldContents string) (string, error) {
	if dep.Regex != "" {
		fullMatches, oldVersions, err := getUsingRegex(dep, oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		newContents := oldContents
		for idx, fullMatch := range fullMatches {
			oldVersion := oldVersions[idx]
			// Replace the full string matched by the regex instead of just the
			// revision ID itself, in case the same string appears more than once in
			// the file.
			repl := strings.Replace(fullMatch, oldVersion, newRev.Id, 1)
			newContents = strings.Replace(newContents, fullMatch, repl, 1)
			if !dep.RegexReplaceAll {
				break
			}
		}
		return newContents, nil
	} else if dep.Path == deps_parser.DepsFileName {
		newContents, err := deps_parser.SetDep(oldContents, id, newRev.Id)
		return newContents, skerr.Wrap(err)
	} else if path.Base(dep.Path) == readme_chromium.FileName {
		files, err := readme_chromium.ParseMulti(oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		rcf, err := findMatchingReadmeChromiumFile(id, dep.Path, files)
		if rcf == nil {
			return "", skerr.Fmt("failed to find %s in %s", id, dep.Path)
		}
		rcf.Revision = newRev.Id
		rcf.Version = "N/A"
		if newRev.Release != "" {
			rcf.Version = newRev.Release
		}
		rcf.Date = ""
		if !util.TimeIsZero(newRev.Timestamp) {
			rcf.Date = newRev.Timestamp.Format("2006-01-02")
		}
		rcf.UpdateMechanism = readme_chromium.UpdateMechanism_Autoroll
		newContents, err := readme_chromium.WriteMulti(files)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return string(newContents), nil
	} else if strings.HasSuffix(dep.Path, ".pyl") {
		return pyl.Set(oldContents, id, newRev.Id)
	} else if bazel.IsBazelFile(dep.Path) {
		var dependency bazel.Dependency
		if len(newRev.Meta) > 0 {
			dependency = &bazel.MetaDependency{
				ID:      bazel.DependencyID(id),
				Version: newRev.Id,
				SHA256:  newRev.Meta,
			}
		} else {
			dependency = &bazel.SingleDependency{
				ID:      bazel.DependencyID(id),
				Version: newRev.Id,
				SHA256:  newRev.Checksum,
			}
		}
		newContents, err := bazel.SetDep(oldContents, dependency)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return newContents, nil
	} else {
		// Various tools expect a newline at the end of the file.
		// TODO(borenet): This should probably be configurable.
		return newRev.Id + "\n", nil
	}
}

// GetFileFunc is a function which retrieves the contents of a file.
type GetFileFunc func(ctx context.Context, path string) (string, error)

// updateSingleDep updates the dependency in the given file, writing the new
// contents into the changes map and returning the previous version.
func updateSingleDep(ctx context.Context, dep *config.VersionFileConfig, newRev *revision.Revision, changes map[string]string, getFile GetFileFunc) (string, error) {
	var primaryOldVersion string
	for idx, file := range dep.File {
		// Look up the path in our changes map to prevent overwriting
		// modifications we've already made.
		oldContents, ok := changes[file.Path]
		if !ok {
			var err error
			oldContents, err = getFile(ctx, file.Path)
			if err != nil {
				return "", skerr.Wrap(err)
			}
		}

		// Find the currently-pinned revision.
		oldVersion, _, err := getPinnedRevInFile(dep.Id, file, oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if idx == 0 {
			primaryOldVersion = oldVersion
		}

		// Create the new file content.
		if newRev.Id != oldVersion {
			newContents, err := setPinnedRevInFile(dep.Id, file, newRev, oldContents)
			if err != nil {
				return "", skerr.Wrap(err)
			}
			if newContents == oldContents {
				return "", skerr.Fmt("Failed to update dependency %s from %s to %s in %s; new contents identical to old contents:\n%s", dep.Id, oldVersion, newRev.Id, file.Path, oldContents)
			}
			changes[file.Path] = newContents

			// Update gitlink if it exists.
			if file.Path == deps_parser.DepsFileName {
				hasSubmodules, err := deps_parser.HasGitSubmodules(oldContents)
				if err != nil {
					return "", skerr.Wrap(err)
				}
				if hasSubmodules {
					depsEntry, err := deps_parser.GetDep(oldContents, dep.Id)
					if err != nil {
						return "", skerr.Wrap(err)
					}
					oldContents, err = getFile(ctx, depsEntry.Path)
					if err == nil {
						// Path found
						newContents = strings.ReplaceAll(oldContents, oldVersion, newRev.Id)
						if oldContents != newContents {
							changes[depsEntry.Path] = newContents
						}
					}
				}
			}

		}
	}
	return primaryOldVersion, nil
}

// UpdateDep updates the given dependency to the given revision, also updating
// any transitive dependencies to the revisions specified in the new revision of
// the primary dependency. Returns a map whose keys are file names to update and
// values are their updated contents.
func UpdateDep(ctx context.Context, primaryDep *config.DependencyConfig, rev *revision.Revision, getFile GetFileFunc) (map[string]string, error) {
	// Update the primary dependency.
	changes := make(map[string]string, 1+len(primaryDep.Transitive))
	oldRev, err := updateSingleDep(ctx, primaryDep.Primary, rev, changes, getFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	replacements := map[string]string{
		oldRev: rev.Id,
	}

	// Handle transitive dependencies.
	if len(primaryDep.Transitive) > 0 {
		for _, dep := range primaryDep.Transitive {
			// Find the new revision.
			newRev, ok := rev.Dependencies[dep.Child.Id]
			if !ok {
				return nil, skerr.Fmt("Could not find transitive dependency %q in %#v", dep.Child.Id, rev)
			}
			// Update.
			oldRev, err := updateSingleDep(ctx, dep.Parent, &revision.Revision{Id: newRev}, changes, getFile)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			replacements[oldRev] = newRev
		}
	}

	// Handle find-and-replace.
	sklog.Infof("find and replace count: \"%d\".", len(primaryDep.FindAndReplace))
	for _, f := range primaryDep.FindAndReplace {
		oldContents, ok := changes[f]
		if !ok {
			oldContents, err = getFile(ctx, f)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
		}
		newContents := oldContents
		for oldRev, newRev := range replacements {
			sklog.Infof("replacing \"%s\" with \"%s\" in %s.", oldRev, newRev, f)
			newContents = strings.ReplaceAll(newContents, oldRev, newRev)
		}
		if oldContents != newContents {
			changes[f] = newContents
		} else if _, ok := changes[f]; !ok {
			sklog.Warningf("find-and-replace made no changes to %s", f)
		}
	}

	return changes, nil
}

// findMatchingReadmeChromiumFile attempts to find the ReadmeChromiumFile
// matching the given dependency ID.
func findMatchingReadmeChromiumFile(id, path string, files []*readme_chromium.ReadmeChromiumFile) (*readme_chromium.ReadmeChromiumFile, error) {
	if len(files) == 0 {
		return nil, skerr.Fmt("found no README.chromium files")
	} else if len(files) == 1 {
		return files[0], nil
	}
	// Try to match the URL with the dependency ID using substring match.
	// The idea is that most of the URL fields are something like:
	// https://github.com/some-org/some-repo
	// but the dependency ID is:
	// https://chromium.googlesource.com/third_party/github.com/some-org/some-repo

	// TODO(borenet): This is very brittle; it would probably be better
	// to add a configuration option to match the name or URL
	// explicitly.
	for _, file := range files {
		trimmed := strings.Replace(file.URL, "https://", "", 1)
		if strings.Contains(id, trimmed) {
			return file, nil
		}
	}
	return nil, skerr.Fmt("failed to find %s in %s", id, path)
}
