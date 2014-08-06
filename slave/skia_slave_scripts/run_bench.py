#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia benchmarking executable."""

import sys

from build_step import BuildStep

class RunBench(BuildStep):
  """A BuildStep that runs bench."""

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunBench, self).__init__(timeout=timeout,
                                   no_output_timeout=no_output_timeout,
                                   **kwargs)

  def _Run(self):
    return  # TODO(mtklein): delete this file after master restart


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
