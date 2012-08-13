#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia GM executable. """

from utils import misc
from build_step import BuildStep
import errno
import os
import shutil
import sys

class RunGM(BuildStep):
  def _PreGM(self,):
    print 'Removing %s' % self._gm_actual_dir
    try:
      shutil.rmtree(self._gm_actual_dir)
    except:
      pass
    print 'Creating %s' % self._gm_actual_dir
    try:
      os.makedirs(self._gm_actual_dir)
    except OSError as e:
      if e.errno == errno.EEXIST:
        pass
      else:
        raise e

  def _Run(self, args):
    self._PreGM()
    cmd = [self._PathToBinary('gm'),
           '-w', self._gm_actual_dir,
           ]
    misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(RunGM))