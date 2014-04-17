#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Uploads the results of render_pictures.py.

TODO(epoger): Re-implement using checksums.

TODO(epoger): Rename as upload_rendered_skps.py, and add a separate
compare_rendered_skps.py .

"""

import sys

from build_step import BuildStep


class CompareAndUploadWebpageGMs(BuildStep):

  def _Run(self):
    # Skip this step for now until we have checksums.
    # Bug: https://code.google.com/p/skia/issues/detail?id=1455
    return


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareAndUploadWebpageGMs))
