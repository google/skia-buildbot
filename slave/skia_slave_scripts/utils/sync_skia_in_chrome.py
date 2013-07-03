#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Create (if needed) and sync a nested checkout of Skia inside of Chrome. """


from config_private import SKIA_SVN_BASEURL
import gclient_utils
from optparse import OptionParser
import os
import shell_utils
import sys


CHROME_LKGR_URL = 'http://chromium-status.appspot.com/lkgr'
CHROME_SVN_URL = 'https://src.chromium.org/chrome/trunk/src'
PATH_TO_SKIA_IN_CHROME = os.path.join('src', 'third_party', 'skia', 'src')
REVISION_PREFIX = 'Revision: '
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
  if not skia_revision:
    output = shell_utils.Bash([SVN, 'info', SKIA_SVN_BASEURL])
    for line in output.splitlines():
      if line.startswith(REVISION_PREFIX):
        skia_revision = line[len(REVISION_PREFIX):].rstrip('\n')
        break
    if not skia_revision:
      raise Exception('Could not determine current Skia revision!')

  gclient_spec = [
    {
      'name': CHROME_SVN_URL.split('/')[-1],
      'url': CHROME_SVN_URL,
      'deps_file': 'DEPS',
      'managed': 'True',
      'safesync_url': CHROME_LKGR_URL,
      'custom_vars': {
        'skia_revision': str(skia_revision),
      },
    },
  ]

  gclient_utils.Config('solutions = %s' % repr(gclient_spec))
  gclient_utils.Sync(revision=str(chrome_revision), jobs=1)
  actual_skia_rev = shell_utils.Bash([SVNVERSION, PATH_TO_SKIA_IN_CHROME])
  actual_chrome_rev = shell_utils.Bash([SVNVERSION,
                                        CHROME_SVN_URL.split('/')[-1]])
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
    Sync(options.skia_revision, options.chrome_revision)
  finally:
    os.chdir(cur_dir)


if __name__ == '__main__':
  sys.exit(Main())