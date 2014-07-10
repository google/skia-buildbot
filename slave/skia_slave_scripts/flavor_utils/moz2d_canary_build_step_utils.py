#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities step for Moz2D build steps. """

from common import chromium_utils
from default_build_step_utils import DefaultBuildStepUtils
from py.utils import misc
from py.utils import shell_utils

import os


MOZ2D_DIR = 'moz2d'


class Moz2DCanaryBuildStepUtils(DefaultBuildStepUtils):
  def Compile(self, target):
    if target == 'moz2d':
      with misc.ChDir(os.path.join(os.pardir, MOZ2D_DIR)):
        shell_utils.run(['make', '-f', 'Makefile.standalone',
                         'MOZ2D_SKIA=../skia'])
    else:
      DefaultBuildStepUtils.Compile(self, target)

  def MakeClean(self):
    DefaultBuildStepUtils.MakeClean(self)
    with misc.ChDir(os.path.join(os.pardir, MOZ2D_DIR)):
      if os.path.isdir('release'):
        chromium_utils.RemoveDirectory('release')