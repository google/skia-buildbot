#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run after the render steps. """

from build_step import BuildStep
import sys


class PostRender(BuildStep):
  def _Run(self):
    # This is a no-op in the default case.
    pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PostRender))