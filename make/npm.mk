# Resolves to the repository root directory.
#
# Reference: https://stackoverflow.com/a/18137056.
#
# Since this variable will be visible to any Makefiles that include this file, we prefix it with
# "_npm_mk" to reduce the chances of name collisions.
_npm_mk_repo_root_dir := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))/..)

# Add this as a prerequisite to any target that depends on the //node_modules directory.
.PHONY: npm-ci
npm-ci: $(_npm_mk_repo_root_dir)/node_modules/lastupdate

$(_npm_mk_repo_root_dir)/node_modules/lastupdate: $(_npm_mk_repo_root_dir)/package-lock.json
	cd $(_npm_mk_repo_root_dir) && npm ci
	touch $(_npm_mk_repo_root_dir)/node_modules/lastupdate

$(_npm_mk_repo_root_dir)/package-lock.json: $(_npm_mk_repo_root_dir)/package.json
	cd $(_npm_mk_repo_root_dir) && npm install
	# If we change package.json and "npm install" leaves file package-lock.json intact, "make" will
	# always rebuild this target on subsequent invocations because it thinks that package-lock.json
	# is out of date. This can happen e.g. when we edit package.json without changing the
	# dependencies/devDependencies/etc. We prevent this by touching package-lock.json.
	touch $(_npm_mk_repo_root_dir)/package-lock.json
