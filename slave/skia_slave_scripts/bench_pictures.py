#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

import sys

from build_step import BuildStep

class BenchPictures(BuildStep):
  def _Run(self):
    return  # TODO(mtklein): delete this file after master restart

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchPictures))
