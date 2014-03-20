#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """


from build_step import BuildStep
from utils import force_update_checkout
import sys


class UpdateScripts(BuildStep):
  def __init__(self, attempts=5, **kwargs):
    super(UpdateScripts, self).__init__(attempts=attempts, **kwargs)

  def _Run(self):
    force_update_checkout.force_update()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))
