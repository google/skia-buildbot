package parent

import "go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"

// GitCheckoutGithubFileConfig provides configuration for a Parent which uses a
// local Git checkout, uploads pull requests on Github, and pins dependencies
// using a file checked into the repo. The revision ID of the dependency makes
// up the full contents of the file, unless the file is "DEPS", which is a
// special case.
type GitCheckoutGithubFileConfig struct {
	GitCheckoutGithubConfig
	version_file_common.VersionFileConfig
}
