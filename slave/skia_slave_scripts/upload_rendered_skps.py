#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Uploads the results of render_skps.py.

TODO(epoger): In the midst of re-implementing using checksums;
see https://code.google.com/p/skia/issues/detail?id=1942
"""

import sys

from build_step import BuildStep
import upload_gm_results


class UploadRenderedSKPs(upload_gm_results.UploadGMResults):

  def __init__(self, attempts=3, **kwargs):
    super(UploadRenderedSKPs, self).__init__(
        attempts=attempts, **kwargs)

  def _Run(self):
    self._SVNUploadJsonFiles(src_dir=self.skp_out_dir,
                             dest_subdir='rendered-skps')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadRenderedSKPs))
