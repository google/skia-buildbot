#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run before the render steps. """

from build_step import BuildStep
import os
import shutil
import sys


class PreRender(BuildStep):
  def _Run(self):
    if os.path.exists(self._gm_actual_dir):
      print 'Removing %s' % self._gm_actual_dir
      shutil.rmtree(self._gm_actual_dir)
    print 'Creating %s' % self._gm_actual_dir
    os.makedirs(self._gm_actual_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PreRender))