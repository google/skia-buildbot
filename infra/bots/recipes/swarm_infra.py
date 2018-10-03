# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


# Recipe for Skia Infra.


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


INFRA_GO = 'go.skia.org/infra'
INFRA_GIT_URL = 'https://skia.googlesource.com/buildbot'

REF_HEAD = 'HEAD'
REF_ORIGIN_MASTER = 'origin/master'


def git(api, *cmd, **kwargs):
  git_cmd = 'git.bat' if api.platform.is_win else 'git'
  return api.step(
      'git %s' % cmd[0],
      cmd=[git_cmd] + list(cmd),
      **kwargs)


def RunSteps(api):
  # The 'build' and 'depot_tools directories come from recipe DEPS and aren't
  # provided by default. We have to set them manually.
  api.path.c.base_paths['depot_tools'] = (
      api.path.c.base_paths['start_dir'] +
      ('recipe_bundle', 'depot_tools'))

  go_dir = api.path['start_dir'].join('go_deps')
  go_src = go_dir.join('src')
  api.file.ensure_directory('makedirs go/src', go_src)
  infra_dir = go_src.join(INFRA_GO)
  go_root = api.path['start_dir'].join('go', 'go')
  go_bin = go_root.join('bin')

  # Run bot_update.
  cfg_kwargs = {}
  gclient_cfg = api.gclient.make_config(**cfg_kwargs)
  dirname = go_dir.join('src', 'go.skia.org')
  basename = 'infra'
  sln = gclient_cfg.solutions.add()
  sln.name = basename
  sln.managed = False
  sln.url = INFRA_GIT_URL
  sln.revision = api.properties.get('revision', 'origin/master')
  gclient_cfg.got_revision_mapping[basename] = 'got_revision'
  patch_refs = None
  patch_ref = api.properties.get('patch_ref')
  if patch_ref:
    patch_refs = ['%s@%s' %(api.properties['patch_repo'], patch_ref)]

  with api.context(cwd=dirname):
    api.bot_update.ensure_checkout(gclient_config=gclient_cfg,
                                   patch_refs=patch_refs)

  # Fetch Go dependencies.
  env = {
      'CHROME_HEADLESS': '1',
      'GOROOT': go_root,
      'GOPATH': go_dir,
      'GIT_USER_AGENT': 'git/1.9.1',  # I don't think this version matters.
      'PATH': api.path.pathsep.join([
          str(go_bin),
          str(go_dir.join('bin')),
          str(api.path['start_dir'].join('gcloud_linux', 'bin')),
          str(api.path['start_dir'].join('protoc', 'bin')),
          str(api.path['start_dir'].join('node', 'node', 'bin')),
          '%(PATH)s',
      ]),
  }
  with api.context(cwd=infra_dir, env=env):
    api.step('which go', cmd=['which', 'go'])

  # Set got_revision.
  test_data = lambda: api.raw_io.test_api.stream_output('abc123')
  with api.context(cwd=infra_dir):
    rev_parse = git(api, 'rev-parse', 'HEAD',
                    stdout=api.raw_io.output(),
                    step_test_data=test_data)
  rev_parse.presentation.properties['got_revision'] = rev_parse.stdout.strip()

  # More prerequisites.
  builder = api.properties['buildername']
  with api.context(cwd=infra_dir, env=env):
    if 'Race' not in builder:
      api.step(
          'install bower',
          cmd=['sudo', 'npm', 'i', '-g', 'bower@1.8.2'])

  with api.context(cwd=infra_dir.join('go', 'database'), env=env):
    api.step(
        'setup database',
        cmd=['./setup_test_db'])

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
  env['TMPDIR'] = None
  env['PATH'] = api.path.pathsep.join([
      env['PATH'], str(api.path['depot_tools'])])

  cmd = ['go', 'run', './run_unittests.go', '--alsologtostderr']
  if 'Race' in api.properties['buildername']:
    cmd.extend(['--race', '--large', '--medium', '--small'])
  elif 'Large' in api.properties['buildername']:
    cmd.append('--large')
  elif 'Medium' in api.properties['buildername']:
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

def GenTests(api):
  yield (
      api.test('Infra-PerCommit') +
      api.path.exists(api.path['start_dir'].join('gopath', 'src', INFRA_GO,
                                                 '.git')) +
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
                     revision=REF_HEAD,
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
