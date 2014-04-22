#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Create (if needed) and sync a nested checkout of Skia inside of Chrome. """

from config_private import SKIA_GIT_URL
from common import chromium_utils
from optparse import OptionParser

import gclient_utils
import os
import re
import shell_utils
import shlex
import sys
import urllib2


CHROME_LKGR_URL = 'http://chromium-status.appspot.com/git-lkgr'
FETCH = 'fetch.bat' if os.name == 'nt' else 'fetch'
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'
GIT = gclient_utils.GIT
GCLIENT_FILE = '.gclient'
PATH_TO_SKIA_IN_CHROME = os.path.join('src', 'third_party', 'skia', 'src')


def Sync(skia_revision=None, chrome_revision=None, use_lkgr_skia=False,
         override_skia_checkout=True):
  """ Create and sync a checkout of Skia inside a checkout of Chrome. Returns
  a tuple containing the actually-obtained revision of Skia and the actually-
  obtained revision of Chrome.

  skia_revision: revision of Skia to sync. If None, will attempt to determine
      the most recent Skia revision. Ignored if use_lkgr_skia is True.
  chrome_revision: revision of Chrome to sync. If None, will use the LKGR.
  use_lkgr_skia: boolean; if True, leaves Skia at the revision requested by
      Chrome instead of using skia_revision.
  override_skia_checkout: boolean; whether or not to replace the default Skia
      checkout, which is actually a set of three subdirectory checkouts in
      third_party/skia: src, include, and gyp, with a single checkout of skia at
      the root level. Default is True.
  """
  # Figure out what revision of Skia we should use.
  if not skia_revision:
    output = shell_utils.run([GIT, 'ls-remote', SKIA_GIT_URL, '--verify',
                              'refs/heads/master'])
    if output:
      skia_revision = shlex.split(output)[0]
    if not skia_revision:
      raise Exception('Could not determine current Skia revision!')
  skia_revision = str(skia_revision)

  # Use Chrome LKGR, since gclient_utils will force a sync to origin/master.
  if not chrome_revision:
    chrome_revision = urllib2.urlopen(CHROME_LKGR_URL).read()

  # Run "fetch chromium". The initial run is allowed to fail after it does some
  # work. At the least, we expect the .gclient file to be present when it
  # finishes.
  if not os.path.isfile(GCLIENT_FILE):
    try:
      shell_utils.run([FETCH, 'chromium', '--nosvn=True'])
    except shell_utils.CommandFailedException:
      pass
  if not os.path.isfile(GCLIENT_FILE):
    raise Exception('Could not fetch chromium!')

  if override_skia_checkout:
    # Hack the .gclient file to use LKGR and NOT check out Skia.
    gclient_vars = {}
    execfile(GCLIENT_FILE, gclient_vars)
    for solution in gclient_vars['solutions']:
      if solution['name'] == 'src':
        solution['safesync_url'] = CHROME_LKGR_URL
        if not solution.get('custom_deps'):
          solution['custom_deps'] = {}
        solution['managed'] = True
        solution['custom_deps']['src/third_party/skia/gyp'] = None
        solution['custom_deps']['src/third_party/skia/include'] = None
        solution['custom_deps']['src/third_party/skia/src'] = None
        break
    print 'Writing %s:' % GCLIENT_FILE
    with open(GCLIENT_FILE, 'w') as gclient_file:
      for k, v in gclient_vars.iteritems():
        if not k.startswith('_'):
          write_str = '%s = %s\n' % (str(k), str(v))
          print write_str
          gclient_file.write(write_str)

  # Run "gclient sync"
  try:
    # Hack: We have to set some GYP_DEFINES, or upstream scripts will complain.
    os.environ['GYP_DEFINES'] = os.environ.get('GYP_DEFINES') or ''
    gclient_utils.Sync(
        revisions=[('src', chrome_revision)],
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
      print 'Attempting to remove %s' % file_to_delete
      os.remove(file_to_delete)
    except OSError:
      # If the file no longer exists, just try again.
      pass
    gclient_utils.Sync(
        revisions=[('src', chrome_revision)],
        jobs=1,
        no_hooks=True,
        force=True)

  # Find the actually-obtained Chrome revision.
  os.chdir('src')
  actual_chrome_rev = shell_utils.run([GIT, 'rev-parse', 'HEAD'],
                                      log_in_real_time=False).rstrip()

  skia_dir = os.path.join('third_party', 'skia')
  src_dir = os.getcwd()
  if override_skia_checkout:
    if use_lkgr_skia:
      # Get the Skia revision requested by Chrome.
      deps_vars = {}
      deps_vars['Var'] = lambda x: deps_vars['vars'][x]
      execfile('DEPS', deps_vars)
      skia_revision = deps_vars['vars']['skia_hash']
      print 'Overriding skia_revision with %s' % skia_revision

    # Check out Skia.
    print 'cd %s' % skia_dir
    os.chdir(skia_dir)
    try:
      # Determine whether we already have a Skia checkout. If so, just update.
      if not 'skia' in shell_utils.run([GIT, 'remote', '-v']):
        raise Exception('%s does not contain a Skia checkout!' % skia_dir)
      current_skia_rev = shell_utils.run([GIT, 'rev-parse', 'HEAD']).rstrip()
      print 'Found existing Skia checkout at %s' % current_skia_rev
      shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])
      shell_utils.run([GIT, 'checkout', 'master'])
      shell_utils.run([GIT, 'fetch'])
      shell_utils.run([GIT, 'reset', '--hard', 'origin/master'])
    except Exception:
      # If updating fails, assume that we need to check out Skia from scratch.
      os.chdir(os.pardir)
      chromium_utils.RemoveDirectory('skia')
      shell_utils.run([GIT, 'clone', SKIA_GIT_URL, 'skia'])
      os.chdir('skia')
    shell_utils.run([GIT, 'reset', '--hard', skia_revision])
  else:
    os.chdir(os.path.join(skia_dir, 'src'))

  # Find the actually-obtained Skia revision.
  actual_skia_rev = shell_utils.run([GIT, 'rev-parse', 'HEAD'],
                                    log_in_real_time=False).rstrip()

  # Run gclient hooks
  os.chdir(src_dir)
  shell_utils.run([GCLIENT, 'runhooks'])

  # Fix the submodules so that they don't show up in "git status"
  # This fails on Windows...
  if os.name != 'nt':
    submodule_cmd = ('\'git config -f '
                     '$toplevel/.git/config submodule.$name.ignore all\'')
    shell_utils.run(' '.join([GIT, 'submodule', 'foreach', submodule_cmd]),
                    shell=True)

  # Verify that we got the requested revisions of Chrome and Skia.
  if skia_revision != actual_skia_rev and override_skia_checkout:
    raise Exception('Requested Skia revision %s but got %s!' % (
        repr(skia_revision), repr(actual_skia_rev)))
  if chrome_revision and chrome_revision != actual_chrome_rev:
    raise Exception('Requested Chrome revision %s but got %s!' % (
        repr(chrome_revision), repr(actual_chrome_rev)))

  return (actual_skia_rev, actual_chrome_rev)


def Main():
  parser = OptionParser()
  parser.add_option('--skia_revision',
                    help=('Desired revision of Skia. Defaults to the most '
                          'recent revision.'))
  parser.add_option('--chrome_revision',
                    help=('Desired revision of Chrome. Defaults to the Last '
                          'Known Good Revision.'))
  parser.add_option('--destination',
                    help=('Where to sync the code. Defaults to the current '
                          'directory.'),
                    default=os.curdir)
  (options, _) = parser.parse_args()
  dest_dir = os.path.abspath(options.destination)
  cur_dir = os.path.abspath(os.curdir)
  os.chdir(dest_dir)
  try:
    actual_skia_rev, actual_chrome_rev = Sync(options.skia_revision,
                                              options.chrome_revision)
    print 'Chrome synced to %s' % actual_chrome_rev
    print 'Skia synced to %s' % actual_skia_rev
  finally:
    os.chdir(cur_dir)


if __name__ == '__main__':
  sys.exit(Main())
