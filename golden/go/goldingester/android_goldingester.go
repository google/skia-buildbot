package goldingester

import (
	"fmt"

	"go.skia.org/infra/go/androidbuild"
)

var cachedGitHashes = map[string]string{}

// getAndroidGoldPreIngestHook returns a function is an instance of
// PreIngestionHook. The returned function accepts a DMResults instance
// and looks up the corresponding git hash from the androidbuildinternal
// service.
func getAndroidGoldPreIngestHook(gitHashInfo androidbuild.Info) PreIngestionHook {
	return func(dmResults *DMResults) error {
		branch := dmResults.Key["branch"]
		target := dmResults.Key["build_flavor"]
		buildID := dmResults.BuildNumber
		if branch == "" {
			return fmt.Errorf("Missing 'branch' field in keys of test results.")
		}

		if target == "" {
			return fmt.Errorf("Missing 'build_flavor' field in keys of test results.")
		}

		if buildID == "" {
			return fmt.Errorf("Missing value in 'build_number' field of test results.")
		}

		var gitHash string
		var ok bool

		// Cache it in short term memory since the lookup below tends to be slow.
		key := branch + ":" + target + ":" + buildID
		if gitHash, ok = cachedGitHashes[key]; !ok {
			// Look up the git hash.
			commit, err := gitHashInfo.Get(branch, target, buildID)
			if err != nil {
				return err
			}
			gitHash = commit.Hash
			cachedGitHashes[key] = commit.Hash
		}

		// gitHash now contains a valid value.
		dmResults.GitHash = gitHash
		return nil
	}
}
