#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run GM through Valgrind. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from run_gm import RunGM
import os
import sys


class ValgrindRunGM(ValgrindBuildStep, RunGM):
  def __init__(self, suppressions_file=os.path.join('gm', 'valgrind.supp'),
               **kwargs):
    super(ValgrindRunGM, self).__init__(suppressions_file=suppressions_file,
                                        **kwargs)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindRunGM))
