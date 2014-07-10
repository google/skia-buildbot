#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Sync the Android sources."""


import os
import shlex
import sys

from build_step import BuildStep
from py.utils import misc
from py.utils import shell_utils
from utils.gclient_utils import GIT


ANDROID_CHECKOUT_PATH = os.path.join(os.pardir, 'android_repo')
ANDROID_REPO_URL = ('https://googleplex-android.googlesource.com/a/platform/'
                    'manifest')
GIT_COOKIE_AUTHDAEMON = os.path.join(os.path.expanduser('~'), 'skia-repo',
                                     'gcompute-tools', 'git-cookie-authdaemon')
REPO_URL = 'http://commondatastorage.googleapis.com/git-repo-downloads/repo'
REPO = os.path.join(os.path.expanduser('~'), 'bin', 'repo')

class GitAuthenticate(object):
  def __init__(self):
    self._auth_daemon_pid = None

  def __enter__(self):
    shell_utils.run([GIT, 'config', 'user.email',
                     '"31977622648@project.gserviceaccount.com"'])
    shell_utils.run([GIT, 'config', 'user.name',
                     '"Skia_Android Canary Bot"'])
    # Authenticate. This is only required on the actual build slave - not on
    # a test slave on someone's machine, where the file does not exist.
    if os.path.exists(GIT_COOKIE_AUTHDAEMON):
      output = shell_utils.run([GIT_COOKIE_AUTHDAEMON])
      self._auth_daemon_pid = shlex.split(output)[-1]
    else:
      print 'No authentication file. Did you authenticate?'

  def __exit__(self, *args):
    if self._auth_daemon_pid:
      shell_utils.run(['kill', self._auth_daemon_pid])


class SyncAndroid(BuildStep):
  """BuildStep which syncs the Android sources."""

  def _Run(self):
    try:
      os.makedirs(ANDROID_CHECKOUT_PATH)
    except OSError:
      pass
    with misc.ChDir(ANDROID_CHECKOUT_PATH):
      if not os.path.exists(REPO):
        # Download repo.
        shell_utils.run(['curl', REPO_URL, '>', REPO])
        shell_utils.run(['chmod', 'a+x', REPO])

      with GitAuthenticate():
        shell_utils.run([REPO, 'init', '-u', ANDROID_REPO_URL, '-g',
                         'all,-notdefault,-darwin', '-b', 'master-skia'])
        shell_utils.run([REPO, 'sync', '-j32'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(SyncAndroid))
