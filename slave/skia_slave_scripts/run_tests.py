#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from build_step import BuildStep
import sys


class RunTests(BuildStep):
  def __init__(self, timeout=9600, no_output_timeout=2400, **kwargs):
    super(RunTests, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _Run(self):
    self._test_args.extend(['--tmpDir', self._device_dirs.TmpDir()])
    if 'Xoom' in self._builder_name:
      # WritePixels fails on Xoom due to a bug which won't be fixed very soon.
      # http://code.google.com/p/skia/issues/detail?id=1699
      self._test_args.extend(['--match', '~WritePixels'])
    self._flavor_utils.RunFlavoredCmd('tests', self._test_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunTests))
