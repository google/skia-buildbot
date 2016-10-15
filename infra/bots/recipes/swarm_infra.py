# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


# Recipe for Skia Infra.


import re


DEPS = [
  'build/file',
  'depot_tools/bot_update',
  'depot_tools/infra_paths',
  'depot_tools/rietveld',
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


def git_checkout(api, url, dest, ref=None):
  """Create a git checkout of the given repo in dest."""
  if api.path.exists(dest.join('.git')):
    # Already have a git checkout. Ensure that the correct remote is set.
    git(api, 'remote', 'set-url', 'origin', INFRA_GIT_URL, cwd=dest)
  else:
    # Clone the repo
    git(api, 'clone', INFRA_GIT_URL, dest)

  # Ensure that the correct ref is checked out.
  git(api, 'fetch', 'origin', cwd=dest)
  git(api, 'clean', '-d', '-f', cwd=dest)
  if ref == REF_HEAD:
    ref = REF_ORIGIN_MASTER
  git(api, 'reset', '--hard', ref or REF_ORIGIN_MASTER, cwd=dest)

  api.path['checkout'] = dest

  # Maybe apply a patch.
  if (api.properties.get('rietveld') and
      api.properties.get('issue') and
      api.properties.get('patchset')):
    api.rietveld.apply_issue()
  elif (api.properties.get('repository') and
        api.properties.get('event.patchSet.ref') and
        api.properties.get('event.change.number')):
    api.bot_update.apply_gerrit_ref(root=dest)
    # Applying gerrit ref puts us in a detached branch. Go back to the master.
    git(api, 'checkout', 'master', cwd=dest)


def RunSteps(api):
  # The 'build' and 'depot_tools directories come from recipe DEPS and aren't
  # provided by default. We have to set them manually.
  api.path.c.base_paths['depot_tools'] = (
      api.path.c.base_paths['slave_build'] +
      ('buildbot', 'infra', 'bots', '.recipe_deps', 'depot_tools'))

  go_dir = api.path['slave_build'].join('go')
  go_src = go_dir.join('src')
  api.file.makedirs('makedirs go/src', go_src)
  infra_dir = go_src.join(INFRA_GO)

  # Check out the infra repo.
  git_checkout(
      api,
      INFRA_GIT_URL,
      dest=infra_dir,
      ref=api.properties.get('revision', 'origin/master'))

  # Fetch Go dependencies.
  env = {'CHROME_HEADLESS': '1',
         'GOPATH': go_dir,
         'GIT_USER_AGENT': 'git/1.9.1',  # I don't think this version matters.
         'PATH': api.path.pathsep.join([str(go_dir.join('bin')), '%(PATH)s'])}
  api.step('update_deps', cmd=['go', 'get', '-u', './...'], cwd=infra_dir,
           env=env)

  # Checkout AGAIN to undo whatever `go get -u` did to the infra repo.
  git_checkout(
      api,
      INFRA_GIT_URL,
      dest=infra_dir,
      ref=api.properties.get('revision', 'origin/master'))

  # Set got_revision.
  test_data = lambda: api.raw_io.test_api.stream_output('abc123')
  rev_parse = git(api, 'rev-parse', 'HEAD',
                  cwd=infra_dir, stdout=api.raw_io.output(),
                  step_test_data=test_data)
  rev_parse.presentation.properties['got_revision'] = rev_parse.stdout.strip()

  # More prerequisites.
  api.step(
      'install goimports',
      cmd=['go', 'get', 'golang.org/x/tools/cmd/goimports'],
      cwd=infra_dir,
      env=env)
  api.step(
      'install errcheck',
      cmd=['go', 'get', 'github.com/kisielk/errcheck'],
      cwd=infra_dir,
      env=env)
  api.step(
      'setup database',
      cmd=['./setup_test_db'],
      cwd=infra_dir.join('go', 'database'),
      env=env)

  # Run tests.
  buildslave = api.properties['slavename']
  m = re.search('^[a-zA-Z-]*(\d+)$', buildslave)
  karma_port = '9876'
  if m and len(m.groups()) > 0:
    karma_port = '15%s' % m.groups()[0]
  env['KARMA_PORT'] = karma_port
  api.python('run_unittests', 'run_unittests', cwd=infra_dir, env=env)


def GenTests(api):
  yield (
      api.test('Infra-PerCommit') +
      api.path.exists(api.path['slave_build'].join('go', 'src', INFRA_GO,
                                                   '.git')) +
      api.properties(slavename='skiabot-linux-infra-001',
                     path_config='kitchen')
  )
  yield (
      api.test('Infra-PerCommit_initialcheckout') +
      api.properties(slavename='skiabot-linux-infra-001',
                     path_config='kitchen')
  )
  yield (
      api.test('Infra-PerCommit_try_rietveld') +
      api.properties(rietveld='https://codereview.chromium.org',
                     issue=1234,
                     patchset=1,
                     revision=REF_HEAD,
                     slavename='skiabot-linux-infra-001',
                     path_config='kitchen')
  )
  gerrit_kwargs = {
      'repository': 'skia',
      'event.patchSet.ref': 'refs/changes/00/2100/2',
      'event.change.number': '2100',
  }
  yield (
      api.test('Infra-PerCommit_try_gerrit') +
      api.properties(revision=REF_HEAD,
                     slavename='skiabot-linux-infra-001',
                     path_config='kitchen',
                     **gerrit_kwargs)
  )
