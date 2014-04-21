#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Sync the Android sources."""


import os
import sys

from build_step import BuildStep
from utils import shell_utils


ANDROID_CHECKOUT_PATH = os.path.join(os.pardir, 'android_repo')
ANDROID_REPO_URL = ('https://googleplex-android.googlesource.com/a/platform/'
                    'manifest')
REPO_URL = 'http://commondatastorage.googleapis.com/git-repo-downloads/repo'
REPO = os.path.join(os.path.expanduser('~'), 'bin', 'repo')

class SyncAndroid(BuildStep):
  """BuildStep which syncs the Android sources."""

  def _Run(self):
    try:
      os.makedirs(ANDROID_CHECKOUT_PATH)
    except OSError:
      pass
    print 'cd %s' % ANDROID_CHECKOUT_PATH
    os.chdir(ANDROID_CHECKOUT_PATH)

    if not os.path.exists(REPO):
      # Download repo.
      shell_utils.run(['curl', REPO_URL, '>', REPO])
      shell_utils.run(['chmod', 'a+x', REPO])

    shell_utils.run([REPO, 'init', '-u', ANDROID_REPO_URL, '-g',
                     'all,-notdefault,-darwin', '-b', 'master-skia'])
    shell_utils.run([REPO, 'sync', '-j32'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(SyncAndroid))
