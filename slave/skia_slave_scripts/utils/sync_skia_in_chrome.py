#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Create (if needed) and sync a nested checkout of Skia inside of Chrome. """

from common import chromium_utils
from optparse import OptionParser

import gclient_utils
import os
import shell_utils
import sys


CHROME_LKGR_URL = 'http://chromium-status.appspot.com/git-lkgr'
CHROME_SVN_URL = 'https://src.chromium.org/chrome/trunk/src'
FETCH = 'fetch.bat' if os.name == 'nt' else 'fetch'
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'
GIT = 'git.bat' if os.name == 'nt' else 'git'
GCLIENT_FILE = '.gclient'
PATH_TO_SKIA_IN_CHROME = os.path.join('src', 'third_party', 'skia', 'src')
REVISION_PREFIX = 'Revision: '
SKIA_SVN_URL = 'https://skia.googlecode.com/svn/trunk'
SVN = 'svn.bat' if os.name == 'nt' else 'svn'
SVNVERSION = 'svnversion.bat' if os.name == 'nt' else 'svnversion'


def Sync(skia_revision=None, chrome_revision=None):
  """ Create and sync a checkout of Skia inside a checkout of Chrome. Returns
  a tuple containing the actually-obtained revision of Skia and the actually-
  obtained revision of Chrome.

  skia_revision: revision of Skia to sync. If None, will attempt to determine
      the most recent Skia revision.
  chrome_revision: revision of Chrome to sync. If None, will use the LKGR.
  """
  # Figure out what revision of Skia we should use.
  if not skia_revision:
    output = shell_utils.Bash([SVN, 'info', SKIA_SVN_URL])
    for line in output.splitlines():
      if line.startswith(REVISION_PREFIX):
        skia_revision = line[len(REVISION_PREFIX):].rstrip()
        break
    if not skia_revision:
      raise Exception('Could not determine current Skia revision!')
  skia_revision = str(skia_revision)

  # Run "fetch chromium". The initial run is allowed to fail after it does some
  # work. At the least, we expect the .gclient file to be present when it
  # finishes.
  if not os.path.isfile(GCLIENT_FILE):
    try:
      shell_utils.Bash([FETCH, 'chromium', '--nosvn=True'])
    except Exception:
      pass
  if not os.path.isfile(GCLIENT_FILE):
    raise Exception('Could not fetch chromium!')

  # Hack the .gclient file to use LKGR and NOT check out Skia.
  gclient_vars = {}
  execfile(GCLIENT_FILE, gclient_vars)
  for solution in gclient_vars['solutions']:
    if solution['name'] == 'src':
      solution['safesync_url'] = CHROME_LKGR_URL
      if not solution.get('custom_deps'):
        solution['custom_deps'] = {}
      solution['custom_deps']['src/third_party/skia/gyp'] = None
      solution['custom_deps']['src/third_party/skia/include'] = None
      solution['custom_deps']['src/third_party/skia/src'] = None
      break
  with open(GCLIENT_FILE, 'w') as gclient_file:
    for gclient_var in gclient_vars.iteritems():
      if not gclient_var[0].startswith('_'):
        gclient_file.write('%s = %s\n' % gclient_var)

  # Run "gclient sync"
  gclient_utils.Sync(revision=str(chrome_revision), jobs=1, no_hooks=True,
                     force=True)

  # Find the actually-obtained Chrome revision.
  os.chdir('src')
  actual_chrome_rev = shell_utils.Bash([GIT, 'rev-parse', 'HEAD']).rstrip()

  # Check out Skia.
  os.chdir(os.path.join('third_party', 'skia'))
  try:
    # Assume that we already have a Skia checkout.
    current_skia_rev = int(shell_utils.Bash([SVNVERSION, os.curdir]).rstrip())
    print 'Found existing Skia checkout at revision %d' % current_skia_rev
    shell_utils.Bash([SVN, 'update', '--revision', skia_revision])
  except Exception:
    # If svnversion fails, assume that we haven't checked out Skia yet. First,
    # remove some troublesome paths, then check out Skia.
    if os.path.isfile('LICENSE'):
      os.remove('LICENSE')
    if os.path.isdir('gyp'):
      chromium_utils.RemoveDirectory('gyp')
    if os.path.isdir('include'):
      chromium_utils.RemoveDirectory('include')
    if os.path.isdir('src'):
      chromium_utils.RemoveDirectory('src')
    shell_utils.Bash([SVN, 'checkout', '%s@%s' % (SKIA_SVN_URL, skia_revision),
                      os.curdir])

  # Find the actually-obtained Skia revision.
  actual_skia_rev = shell_utils.Bash([SVNVERSION, os.curdir]).rstrip()

  # Run gclient hooks
  os.chdir(os.path.join(os.pardir, os.pardir, os.pardir))
  shell_utils.Bash([GCLIENT, 'runhooks'])

  # Verify that we got the requested revisions of Chrome and Skia.
  if skia_revision != actual_skia_rev:
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
