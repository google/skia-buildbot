#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compares the results of render_skps.py against expectations.
"""

import sys

from build_step import BuildStep


class CompareRenderedSKPs(BuildStep):

  def _Run(self):
    print 'TODO(epoger): Not yet implemented; see http://skbug.com/1942'


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareRenderedSKPs))
