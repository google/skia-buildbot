#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Create (if needed) and sync a nested checkout of Skia inside of Chrome. """

import optparse
import os
import re
import shlex
import sys
from urllib.request import urlopen

import gclient_utils
import misc
import shell_utils


CHROME_GIT_URL = 'https://chromium.googlesource.com/chromium/src.git'
CHROME_LKGR_URL = 'http://chromium-status.appspot.com/git-lkgr'
FETCH = 'fetch.bat' if os.name == 'nt' else 'fetch'
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'
GCLIENT_FILE = '.gclient'
PATH_TO_SKIA_IN_CHROME = os.path.join('src', 'third_party', 'skia', 'src')
DEFAULT_FETCH_TARGET = 'chromium'

# Sync Chrome to LKGR.
CHROME_REV_LKGR = 'CHROME_REV_LKGR'
# Sync Chrome to origin/main.
CHROME_REV_MAIN = 'CHROME_REV_MAIN'

# Skia repo URL.
SKIA_GIT_URL = 'https://skia.googlesource.com/skia.git'
# Code revision specified by DEPS.
SKIA_REV_DEPS = 'SKIA_REV_DEPS'
# Sync to origin/main.
SKIA_REV_MAIN = 'SKIA_REV_MAIN'


def GetRemoteMainHash(git_url):
  return shell_utils.run(['git', 'ls-remote', git_url, '--verify',
                          'refs/heads/main']).rstrip()


def GetDepsVar(deps_filepath, variable):
  """Read the given DEPS file and return the value of the given variable.

  Args:
      deps_filepath: string; path to a DEPS file.
      variable: string; name of the variable whose value should be returned.
  Returns:
      string; value of the requested variable.
  """
  deps_vars = {}
  deps_vars['Var'] = lambda x: deps_vars['vars'][x]
  execfile(deps_filepath, deps_vars)
  return deps_vars['vars'][variable]


def Sync(skia_revision=SKIA_REV_DEPS, chrome_revision=CHROME_REV_LKGR,
         fetch_target=DEFAULT_FETCH_TARGET,
         gyp_defines=None, gyp_generators=None):
  """ Create and sync a checkout of Skia inside a checkout of Chrome. Returns
  a tuple containing the actually-obtained revision of Skia and the actually-
  obtained revision of Chrome.

  skia_revision: revision of Skia to sync. Should be a commit hash or one of
      (SKIA_REV_DEPS, SKIA_REV_MAIN).
  chrome_revision: revision of Chrome to sync. Should be a commit hash or one
      of (CHROME_REV_LKGR, CHROME_REV_MAIN).
  fetch_target: string; Calls the fetch tool in depot_tools with the specified
      argument. Default is DEFAULT_FETCH_TARGET.
  gyp_defines: optional string; GYP_DEFINES to be passed to Gyp.
  gyp_generators: optional string; which GYP_GENERATORS to use.
  """
  # Figure out what revision of Skia we should use.
  if skia_revision == SKIA_REV_MAIN:
    output = GetRemoteMainHash(SKIA_GIT_URL)
    if output:
      skia_revision = shlex.split(output)[0]
    if not skia_revision:
      raise Exception('Could not determine current Skia revision!')
  skia_revision = str(skia_revision)

  # Use Chrome LKGR, since gclient_utils will force a sync to origin/main.
  if chrome_revision == CHROME_REV_LKGR:
    chrome_revision = urlopen(CHROME_LKGR_URL).read()
  elif chrome_revision == CHROME_REV_MAIN:
    chrome_revision = shlex.split(
        GetRemoteMainHash(CHROME_GIT_URL))[0]

  # Run "fetch chromium". The initial run is allowed to fail after it does some
  # work. At the least, we expect the .gclient file to be present when it
  # finishes.
  if not os.path.isfile(GCLIENT_FILE):
    try:
      shell_utils.run([FETCH, fetch_target, '--nosvn=True'])
    except shell_utils.CommandFailedException:
      pass
  if not os.path.isfile(GCLIENT_FILE):
    raise Exception('Could not fetch %s!' % fetch_target)

  # Run "gclient sync"
  revisions = [('src', chrome_revision)]
  if skia_revision != SKIA_REV_DEPS:
    revisions.append(('src/third_party/skia', skia_revision))

  try:
    # Hack: We have to set some GYP_DEFINES, or upstream scripts will complain.
    os.environ['GYP_DEFINES'] = os.environ.get('GYP_DEFINES') or ''
    gclient_utils.Sync(
        revisions=revisions,
        jobs=1,
        no_hooks=True,
        force=True)
  except shell_utils.CommandFailedException as e:
    # We frequently see sync failures because a lock file wasn't deleted. In
    # that case, delete the lock file and try again.
    pattern = r".*fatal: Unable to create '(\S+)': File exists\..*"
    match = re.search(pattern, e.output)
    if not match:
      raise e
    file_to_delete = match.groups()[0]
    try:
      print('Attempting to remove %s' % file_to_delete)
      os.remove(file_to_delete)
    except OSError:
      # If the file no longer exists, just try again.
      pass
    gclient_utils.Sync(
        revisions=revisions,
        jobs=1,
        no_hooks=True,
        force=True)

  # Find the actually-obtained Chrome revision.
  os.chdir('src')
  actual_chrome_rev = shell_utils.run([gclient_utils.GIT, 'rev-parse', 'HEAD'],
                                      log_in_real_time=False).rstrip().decode()


  # Find the actually-obtained Skia revision.
  with misc.ChDir(os.path.join('third_party', 'skia')):
    actual_skia_rev = shell_utils.run([gclient_utils.GIT, 'rev-parse', 'HEAD'],
                                      log_in_real_time=False).rstrip().decode()

  # Run gclient hooks
  gclient_utils.RunHooks(gyp_defines=gyp_defines, gyp_generators=gyp_generators)

  # Fix the submodules so that they don't show up in "git status"
  # This fails on Windows...
  if os.name != 'nt':
    submodule_cmd = ('\'git config -f '
                     '$toplevel/.git/config submodule.$name.ignore all\'')
    shell_utils.run(
        ' '.join([gclient_utils.GIT, 'submodule', 'foreach', submodule_cmd]),
        shell=True)

  # Verify that we got the requested revisions of Chrome and Skia.
  if (skia_revision != actual_skia_rev[:len(skia_revision)] and
      skia_revision != SKIA_REV_DEPS):
    raise Exception('Requested Skia revision %s but got %s!' % (
        skia_revision, actual_skia_rev))
  if (chrome_revision and
      chrome_revision != actual_chrome_rev[:len(chrome_revision)]):
    raise Exception('Requested Chrome revision %s but got %s!' % (
        chrome_revision, actual_chrome_rev))

  return (actual_skia_rev, actual_chrome_rev)


def Main():
  parser = optparse.OptionParser()
  parser.add_option('--skia_revision',
                    help=('Desired revision of Skia. Defaults to the most '
                          'recent revision.'))
  parser.add_option('--chrome_revision',
                    help=('Desired revision of Chrome. Defaults to the most '
                          'recent revision.'))
  parser.add_option('--destination',
                    help=('Where to sync the code. Defaults to the current '
                          'directory.'),
                    default=os.curdir)
  parser.add_option('--fetch_target',
                    help=('Calls the fetch tool in depot_tools with the '
                          'specified target.'),
                    default=DEFAULT_FETCH_TARGET)
  (options, _) = parser.parse_args()
  dest_dir = os.path.abspath(options.destination)
  with misc.ChDir(dest_dir):
    actual_skia_rev, actual_chrome_rev = Sync(
        skia_revision=options.skia_revision or SKIA_REV_DEPS,
        chrome_revision=options.chrome_revision or CHROME_REV_MAIN,
        fetch_target=options.fetch_target)
    print('Chrome synced to %s' % actual_chrome_rev)
    print('Skia synced to %s' % actual_skia_rev)


if __name__ == '__main__':
  sys.exit(Main())
