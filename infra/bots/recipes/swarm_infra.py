# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


# Recipe for Skia Infra.


import json
import re


PYTHON_VERSION_COMPATIBILITY = "PY3"


DEPS = [
  'recipe_engine/context',
  'recipe_engine/path',
  'recipe_engine/properties',
  'recipe_engine/raw_io',
  'recipe_engine/step',
]


INFRA_GIT_URL = 'https://skia.googlesource.com/buildbot'


def retry(api, attempts, *args, **kwargs):
  exc = None
  for _ in range(attempts):
    try:
      api.step(*args, **kwargs)
      return
    except api.step.StepFailure as e:
      exc = e
  else:  # pragma: nocover
    raise exc  # pylint:disable=raising-bad-type


def compute_go_root(api):
  # Starting with Go 1.18, the standard library includes "//go:embed"
  # directives that point to other files in the standard library. For security
  # reasons, the "embed" package does not support symbolic links (discussion at
  # https://github.com/golang/go/issues/35950#issuecomment-561725322), and it
  # produces "cannot embed irregular file" errors when it encounters one.
  #
  # To prevent the above error, we ensure our GOROOT environment variable
  # points to a path without symbolic links.
  #
  # For some reason api.path.realpath returns the path unchanged, so we invoke
  # realpath instead.
  go_root = api.path['start_dir'].join('go', 'go')
  symlink_version_file = go_root.join('VERSION') # Arbitrary symlink.
  step_result = api.step('realpath go/go/VERSION',
                         cmd=['realpath', str(symlink_version_file)],
                         stdout=api.raw_io.output_text())
  step_result = api.step('dirname',
                         cmd=['dirname', step_result.stdout],
                         stdout=api.raw_io.output_text())
  go_root_nosymlinks = step_result.stdout.strip()
  if go_root_nosymlinks != "":
    return go_root_nosymlinks # pragma: nocover
  else:
    # This branch exists solely to appease recipe tests, under which the
    # workdir variable is unset. Returning an empty string causes tests to
    # fail, so we return the original GOROOT instead.
    return go_root


def RunSteps(api):
  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  # The 'build' and 'depot_tools directories come from recipe DEPS and aren't
  # provided by default. We have to set them manually.
  api.path.c.base_paths['depot_tools'] = (
      api.path.c.base_paths['start_dir'] +
      ('recipe_bundle', 'depot_tools'))

  gopath = api.path['start_dir'].join('cache', 'gopath')
  infra_dir = api.path['start_dir'].join('buildbot')
  go_cache = api.path['start_dir'].join('cache', 'go_cache')
  go_root = compute_go_root(api)
  go_bin = api.path.join(go_root, 'bin')

  # Initialize the Git repo. We receive the code via Isolate, but it doesn't
  # include the .git dir.
  with api.context(cwd=infra_dir):
    api.step('git init', cmd=['git', 'init'])
    api.step('git add', cmd=['git', 'add', '.'])
    api.step('git commit',
             cmd=['git', 'commit', '-m', 'Fake commit to satisfy recipe tests'])

  # Fetch Go dependencies.
  env = {
      'CHROME_HEADLESS': '1',
      'DOCKER_CONFIG': '/home/chrome-bot/.docker',
      'GOCACHE': go_cache,
      'GOFLAGS': '-mod=readonly', # Prohibit builds from modifying go.mod.
      'GOROOT': go_root,
      'GOPATH': gopath,
      'GIT_USER_AGENT': 'git/1.9.1',  # I don't think this version matters.
      'PATH': api.path.pathsep.join([
          str(go_bin),
          str(gopath.join('bin')),
          str(api.path['start_dir'].join('gcloud_linux', 'bin')),
          str(api.path['start_dir'].join('protoc', 'bin')),
          str(api.path['start_dir'].join('node', 'node', 'bin')),
          str(api.path['start_dir'].join('cockroachdb')),
          '%(PATH)s',
      ]),
  }
  with api.context(cwd=infra_dir, env=env):
    api.step('which go', cmd=['which', 'go'])

    # Try up to three times in case of transient network failures.
    retry(api, 3, 'go mod download', cmd=['go', 'mod', 'download'])

    install_targets = [
      'github.com/golang/protobuf/protoc-gen-go',
      'github.com/kisielk/errcheck',
      'golang.org/x/tools/cmd/goimports',
      'golang.org/x/tools/cmd/stringer',
      'github.com/twitchtv/twirp/protoc-gen-twirp',
      'github.com/skia-dev/protoc-gen-twirp_typescript'
    ]
    for target in install_targets:
      api.step('go install %s' % target, cmd=['go', 'install', '-v', target])

  # More prerequisites.
  builder = api.properties['buildername']
  run_emulators = infra_dir.join('scripts', 'run_emulators', 'run_emulators')
  if ('Large' in builder) or ('Race' in builder):
    with api.context(cwd=infra_dir, env=env):
      api.step('start the cloud emulators', cmd=[run_emulators, 'start'])
    env['DATASTORE_EMULATOR_HOST'] = 'localhost:8891'
    env['BIGTABLE_EMULATOR_HOST'] = 'localhost:8892'
    env['PUBSUB_EMULATOR_HOST'] = 'localhost:8893'
    env['FIRESTORE_EMULATOR_HOST'] = 'localhost:8894'
    env['COCKROACHDB_EMULATOR_HOST'] = 'localhost:8895'

  # Run tests.
  env['SKIABOT_TEST_DEPOT_TOOLS'] = api.path['depot_tools']
  env['PATH'] = api.path.pathsep.join([
      env['PATH'], str(api.path['depot_tools'])])

  cmd = ['go', 'run', './run_unittests.go', '--verbose']
  if 'Race' in builder:
    cmd.extend(['--race', '--large', '--medium', '--small'])
  elif 'Large' in builder:
    cmd.append('--large')
  elif 'Medium' in builder:
    cmd.append('--medium')
  else:
    cmd.append('--small')
  try:
    with api.context(cwd=infra_dir, env=env):
      api.step('run_unittests', cmd)
  finally:
    if ('Large' in builder) or ('Race' in builder):
      with api.context(cwd=infra_dir, env=env):
        api.step('stop the cloud emulators',
                 cmd=[run_emulators, 'stop', '--dump-logs'])

  # Sanity check; none of the above should have modified the go.mod file.
  with api.context(cwd=infra_dir):
    api.step('git diff go.mod',
             cmd=['git', 'diff', '--no-ext-diff', '--exit-code', 'go.mod'])

def GenTests(api):
  test_revision = 'abc123'
  yield (
      api.test('Infra-PerCommit') +
      api.properties(buildername='Infra-PerCommit-Small',
                     path_config='kitchen') +
      api.step_data('go mod download', retcode=1)
  )
  yield (
      api.test('Infra-PerCommit_initialcheckout') +
      api.properties(buildername='Infra-PerCommit-Small',
                     path_config='kitchen')
  )
  yield (
      api.test('Infra-PerCommit_try_gerrit') +
      api.properties(buildername='Infra-PerCommit-Small',
                     revision=test_revision,
                     patch_issue='1234',
                     patch_ref='refs/changes/34/1234/1',
                     patch_repo='https://skia.googlesource.com/buildbot.git',
                     patch_set='1',
                     patch_storage='gerrit',
                     path_config='kitchen',
                     repository='https://skia.googlesource.com/buildbot.git')
  )
  yield (
      api.test('Infra-PerCommit-Large') +
      api.properties(buildername='Infra-PerCommit-Large',
                     path_config='kitchen')
  )
  yield (
      api.test('Infra-PerCommit-Medium') +
      api.properties(buildername='Infra-PerCommit-Medium',
                     path_config='kitchen')
  )
  yield (
      api.test('Infra-PerCommit-Race') +
      api.properties(buildername='Infra-PerCommit-Race',
                     path_config='kitchen')
  )
