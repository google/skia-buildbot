#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for Chrome canary build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from common import chromium_utils
from utils import misc
from utils import shell_utils

import os

# If you're trying to change GYP flags in here, you're too late!
# GYP only runs as part of runhooks in sync_skia_in_chrome.py.

class ChromeCanaryBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    self._baseline_dir = os.path.join(os.pardir, 'webkit-master')
    self._result_dir = os.path.join(os.pardir, 'layouttest_results')

  def RunFlavoredCmd(self, app, args):
    """Run the executable."""
    # Run through runtest.py everywhere but Windows, where it doesn't work for
    # some reason (see http://skbug.com/2520).
    if os.name == 'nt':
      cmd = [self._PathToBinary(app)] + args
    else:
      runtest = os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'scripts', 'slave',
                             'runtest.py')
      cmd = ['python', runtest, '--target', self._step.configuration, app,
             '--xvfb', '--build-dir', 'out']  + args
    shell_utils.run(cmd)

  def Compile(self, target):
    make_cmd = 'ninja'
    cmd = [make_cmd,
           '-C', os.path.join('out', self._step.configuration),
           target,
           ]
    cmd.extend(self._step.make_flags)
    shell_utils.run(cmd)

  def MakeClean(self):
    if os.path.isdir('out'):
      chromium_utils.RemoveDirectory('out')

  @property
  def baseline_dir(self):
    return self._baseline_dir

  @property
  def result_dir(self):
    return self._result_dir
