# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


# Recipe for Skia Infra.


import json
import re


DEPS = [
  'depot_tools/bot_update',
  'depot_tools/gclient',
  'depot_tools/infra_paths',
  'recipe_engine/context',
  'recipe_engine/file',
  'recipe_engine/path',
  'recipe_engine/platform',
  'recipe_engine/properties',
  'recipe_engine/python',
  'recipe_engine/raw_io',
  'recipe_engine/step',
]


INFRA_GIT_URL = 'https://skia.googlesource.com/buildbot'


def RunSteps(api):
  # The 'build' and 'depot_tools directories come from recipe DEPS and aren't
  # provided by default. We have to set them manually.
  api.path.c.base_paths['depot_tools'] = (
      api.path.c.base_paths['start_dir'] +
      ('recipe_bundle', 'depot_tools'))

  gopath = api.path['start_dir'].join('cache', 'gopath')
  infra_dir = api.path['start_dir'].join('buildbot')
  go_cache = api.path['start_dir'].join('cache', 'go_cache')
  go_root = api.path['start_dir'].join('go', 'go')
  go_bin = go_root.join('bin')

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
          '%(PATH)s',
      ]),
  }
  with api.context(cwd=infra_dir, env=env):
    api.step('which go', cmd=['which', 'go'])
    api.step('go mod download', cmd=['go', 'mod', 'download'])
    install_targets = [
      'github.com/golang/protobuf/protoc-gen-go',
      'github.com/kisielk/errcheck',
      'golang.org/x/tools/cmd/goimports',
      'golang.org/x/tools/cmd/stringer',
      'github.com/vektra/mockery/...'
    ]
    for target in install_targets:
      api.step('go install %s' % target, cmd=['go', 'install', '-v', target])

  # More prerequisites.
  builder = api.properties['buildername']
  with api.context(cwd=infra_dir, env=env):
    if 'Race' not in builder:
      api.step(
          'install bower',
          cmd=['sudo', 'npm', 'i', '-g', 'bower@1.8.2'])

  if ('Large' in builder) or ('Race' in builder):
    with api.context(cwd=infra_dir.join('go', 'ds', 'emulator'), env=env):
      api.step(
          'start the cloud data store emulator',
           cmd=['./run_emulator', 'start'])
    env['DATASTORE_EMULATOR_HOST'] = 'localhost:8891'
    env['BIGTABLE_EMULATOR_HOST'] = 'localhost:8892'
    env['PUBSUB_EMULATOR_HOST'] = 'localhost:8893'

  # Run tests.
  env['SKIABOT_TEST_DEPOT_TOOLS'] = api.path['depot_tools']
  env['PATH'] = api.path.pathsep.join([
      env['PATH'], str(api.path['depot_tools'])])

  if 'Build' in builder:
    with api.context(cwd=infra_dir, env=env):
      api.step('make all', ['make', 'all'])
  else:
    cmd = ['go', 'run', './run_unittests.go', '--alsologtostderr']
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
        with api.context(cwd=infra_dir.join('go', 'ds', 'emulator'), env=env):
          api.step('stop the cloud data store emulator',
              cmd=['./run_emulator', 'stop'])

  # Sanity check; none of the above should have modified the go.mod file.
  with api.context(cwd=infra_dir):
    api.step('git diff go.mod',
             cmd=['git', 'diff', '--no-ext-diff', '--exit-code', 'go.mod'])

def GenTests(api):
  test_revision = 'abc123'
  yield (
      api.test('Infra-PerCommit') +
      api.properties(buildername='Infra-PerCommit-Small',
                     path_config='kitchen')
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
      api.test('Infra-PerCommit-Build') +
      api.properties(buildername='Infra-PerCommit-Build',
                     path_config='kitchen')
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
