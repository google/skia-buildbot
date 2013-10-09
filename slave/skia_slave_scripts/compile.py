#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from build_step import BuildStep
import sys


class Compile(BuildStep):
  def __init__(self, timeout=16800, no_output_timeout=16800, **kwargs):
    super (Compile, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _Run(self):
    self._flavor_utils.Compile(self._args['target'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Compile))
