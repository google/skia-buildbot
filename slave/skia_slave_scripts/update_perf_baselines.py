#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Update the performance baselines for this bot."""


import sys

from build_step import BuildStep


class UpdatePerfBaselines(BuildStep):
  """Update the performance baselines for this bot."""

  def _Run(self):
    print 'Placeholder.'


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdatePerfBaselines))
